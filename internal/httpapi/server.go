package httpapi

import (
	"context"
	_ "embed"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/manujkathuria/go-rest-ws/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed openapi.yaml
var openAPISpec []byte

//go:embed docs.html
var docsHTML []byte

// Server wraps the HTTP server with lifecycle controls, metrics, and structured logging.
type Server struct {
	httpServer *http.Server
	mux        *http.ServeMux
	logger     *slog.Logger
	metrics    *Metrics
	draining   atomic.Bool
	handler    http.Handler
}

// NewServer initializes routes, applies middleware in the correct order, and configures the HTTP server.
func NewServer(cfg config.HTTPConfig, logger *slog.Logger, metrics *Metrics) *Server {
	mux := http.NewServeMux()

	s := &Server{
		mux:     mux,
		logger:  logger,
		metrics: metrics,
	}

	// Operational and documentation endpoints
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.Handle("GET /metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	mux.HandleFunc("GET /openapi.yaml", s.handleOpenAPISpec)
	mux.HandleFunc("GET /docs", s.handleDocsUI)

	// Compose middleware stack.
	// Execution order (Outermost -> Innermost):
	// RequestID -> Logging -> Metrics -> Recovery -> JSONError -> Mux
	var h http.Handler = mux
	h = JSONErrorMiddleware(h)
	h = RecoveryMiddleware(logger)(h)
	h = MetricsMiddleware(metrics)(h)
	h = LoggingMiddleware(logger)(h)
	h = RequestIDMiddleware(h)

	s.handler = h
	s.httpServer = &http.Server{
		Addr:         cfg.Addr(),
		Handler:      h,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return s
}

// RegisterHandler registers an additional route handler on the server's internal mux.
func (s *Server) RegisterHandler(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// Serve starts serving traffic on the specified net.Listener.
func (s *Server) Serve(l net.Listener) error {
	return s.httpServer.Serve(l)
}

// Start begins listening on the configured address and serving traffic.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Shutdown initiates graceful draining and waits for in-flight requests to complete.
func (s *Server) Shutdown(ctx context.Context) error {
	s.SetDraining(true)
	return s.httpServer.Shutdown(ctx)
}

// Close immediately closes all active listeners and connections.
func (s *Server) Close() error {
	s.SetDraining(true)
	return s.httpServer.Close()
}

// SetDraining updates the readiness probe state.
func (s *Server) SetDraining(draining bool) {
	s.draining.Store(draining)
}

// IsDraining returns true if the server is in the draining phase.
func (s *Server) IsDraining() bool {
	return s.draining.Load()
}

// Handler returns the assembled root HTTP handler with full middleware stack.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// handleHealthz responds with 200 OK as long as the process is alive.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// handleReadyz signals whether the service can accept traffic.
// Returns 200 OK during normal operation, or 503 Service Unavailable when draining.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.draining.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "draining",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}

// handleOpenAPISpec serves the raw OpenAPI 3.1 YAML document.
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openAPISpec)
}

// handleDocsUI serves the interactive API documentation HTML.
func (s *Server) handleDocsUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(docsHTML)
}
