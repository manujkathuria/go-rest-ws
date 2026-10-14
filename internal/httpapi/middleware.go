package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

type contextKey string

const requestIDContextKey contextKey = "request_id"

// GetRequestID returns the request ID from context if present.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDContextKey).(string); ok {
		return id
	}
	return ""
}

// RequestIDMiddleware extracts or generates a unique correlation request ID.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if reqID == "" || len(reqID) > 128 {
			reqID = generateRequestID()
		}

		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), requestIDContextKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	wroteHeader  bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// LoggingMiddleware emits structured slog events for HTTP requests.
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseWriter(w)
			reqID := GetRequestID(r.Context())

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			// Log health checks at debug level to prevent log clutter during frequent polling;
			// log all other endpoints at info (or warn/error based on status).
			logLevel := slog.LevelInfo
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				logLevel = slog.LevelDebug
			}
			if rw.statusCode >= 500 {
				logLevel = slog.LevelError
			} else if rw.statusCode >= 400 && logLevel != slog.LevelDebug {
				logLevel = slog.LevelWarn
			}

			logger.Log(r.Context(), logLevel, "http request completed",
				"request_id", reqID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.statusCode,
				"duration_ms", float64(duration.Microseconds())/1000.0,
				"bytes", rw.bytesWritten,
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

// normalizeMethod bounds the HTTP method label to prevent metric cardinality explosion.
func normalizeMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete,
		http.MethodPatch, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		return method
	default:
		return "OTHER"
	}
}

// MetricsMiddleware tracks Prometheus HTTP counts, duration histograms, and in-flight requests.
func MetricsMiddleware(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			boundedMethod := normalizeMethod(r.Method)

			m.InFlightGauge.WithLabelValues(boundedMethod).Inc()
			defer m.InFlightGauge.WithLabelValues(boundedMethod).Dec()

			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r)

			// Use matched route pattern from Go 1.22+ ServeMux or normalize to "unmatched"
			routePattern := r.Pattern
			if routePattern == "" {
				routePattern = "unmatched"
			} else {
				// Strip leading HTTP method if present in r.Pattern (e.g. "GET /healthz" -> "/healthz")
				parts := strings.SplitN(routePattern, " ", 2)
				if len(parts) == 2 {
					routePattern = parts[1]
				}
			}

			statusCodeStr := strconv.Itoa(rw.statusCode)
			durationSec := time.Since(start).Seconds()

			m.RequestsTotal.WithLabelValues(boundedMethod, routePattern, statusCodeStr).Inc()
			m.RequestDuration.WithLabelValues(boundedMethod, routePattern, statusCodeStr).Observe(durationSec)
		})
	}
}

// committedResponseWriter tracks whether response headers have been committed.
type committedResponseWriter struct {
	http.ResponseWriter
	committed bool
}

func (w *committedResponseWriter) WriteHeader(code int) {
	w.committed = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *committedResponseWriter) Write(b []byte) (int, error) {
	w.committed = true
	return w.ResponseWriter.Write(b)
}

func (w *committedResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *committedResponseWriter) Flush() {
	w.committed = true
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// RecoveryMiddleware recovers from panics and logs the stack trace with correlation ID.
// If headers have not been committed, it writes a standard JSON 500 error response.
// If headers have already been committed, it logs the panic and aborts the connection/stream
// so that a truncated response is never mistaken for a successful transmission.
func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cw := &committedResponseWriter{ResponseWriter: w}
			defer func() {
				if rvr := recover(); rvr != nil {
					if rvrErr, ok := rvr.(error); ok && errors.Is(rvrErr, http.ErrAbortHandler) {
						panic(rvr)
					}

					reqID := GetRequestID(r.Context())
					if cw.committed {
						// Response headers were already committed to the client wire.
						// Abort the connection so the client receives an abrupt termination instead of a truncated 200.
						logger.ErrorContext(r.Context(), "panic recovered in HTTP handler after response headers committed; aborting connection",
							"error", fmt.Sprint(rvr),
							"stack", string(debug.Stack()),
							"request_id", reqID,
						)

						// Attempt to hijack and abruptly close the underlying network connection
						rc := http.NewResponseController(cw.ResponseWriter)
						if conn, _, err := rc.Hijack(); err == nil {
							_ = conn.Close()
							return
						}

						// Fallback if hijacking is unsupported: panic with http.ErrAbortHandler to abort stream
						panic(http.ErrAbortHandler)
					}

					logger.ErrorContext(r.Context(), "panic recovered in HTTP handler",
						"error", fmt.Sprint(rvr),
						"stack", string(debug.Stack()),
						"request_id", reqID,
					)

					writeJSONError(cw.ResponseWriter, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "An unexpected error occurred", reqID)
				}
			}()
			next.ServeHTTP(cw, r)
		})
	}
}

// errorInterceptorResponseWriter intercepts non-JSON 404 and 405 error responses from ServeMux
// and formats them as standard JSON error responses while preserving headers (such as Allow and X-Request-ID).
type errorInterceptorResponseWriter struct {
	http.ResponseWriter
	reqID        string
	statusCode   int
	wroteHeader  bool
	intercepting bool
}

func (w *errorInterceptorResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.statusCode = code
	w.wroteHeader = true

	// If 404 Not Found or 405 Method Not Allowed and not already application/json,
	// intercept the response to provide standard JSON error payload.
	ct := w.Header().Get("Content-Type")
	if (code == http.StatusNotFound || code == http.StatusMethodNotAllowed) && !strings.HasPrefix(ct, "application/json") {
		w.intercepting = true
		w.Header().Set("Content-Type", "application/json")
		w.ResponseWriter.WriteHeader(code)
		var errCode, errMsg string
		if code == http.StatusNotFound {
			errCode = "NOT_FOUND"
			errMsg = "The requested resource was not found"
		} else {
			errCode = "METHOD_NOT_ALLOWED"
			errMsg = "The requested HTTP method is not allowed for this endpoint"
		}
		_ = json.NewEncoder(w.ResponseWriter).Encode(map[string]string{
			"code":       errCode,
			"message":    errMsg,
			"request_id": w.reqID,
		})
		return
	}

	w.ResponseWriter.WriteHeader(code)
}

func (w *errorInterceptorResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.intercepting {
		// Suppress standard library's default plaintext body
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}

func (w *errorInterceptorResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *errorInterceptorResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// JSONErrorMiddleware intercepts non-JSON 404 and 405 responses and writes JSON error payloads.
func JSONErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := GetRequestID(r.Context())
		ew := &errorInterceptorResponseWriter{
			ResponseWriter: w,
			reqID:          reqID,
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(ew, r)
		if !ew.wroteHeader {
			ew.WriteHeader(http.StatusOK)
		}
	})
}

// writeJSONError formats a consistent error response compliant with OpenAPI specification.
func writeJSONError(w http.ResponseWriter, status int, code, message, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	payload := map[string]string{
		"code":       code,
		"message":    message,
		"request_id": requestID,
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// generateRequestID generates a 16-byte cryptographically random hex string formatted like UUID v4.
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based representation if PRNG fails
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
