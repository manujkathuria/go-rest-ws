package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/manujkathuria/go-rest-ws/internal/config"
	"github.com/manujkathuria/go-rest-ws/internal/httpapi"
	"gopkg.in/yaml.v3"
)

// syncBuffer provides a thread-safe bytes.Buffer for slog testing under -race.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newTestServer(logBuf io.Writer) (*httpapi.Server, *httpapi.Metrics) {
	cfg := config.DefaultConfig().HTTP
	if logBuf == nil {
		logBuf = io.Discard
	}
	logger := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	metrics := httpapi.NewMetrics()
	srv := httpapi.NewServer(cfg, logger, metrics)
	return srv, metrics
}

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}

	reqID := rec.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected non-empty X-Request-ID header")
	}
}

func TestReadyzTransitions(t *testing.T) {
	srv, _ := newTestServer(nil)

	// 1. Initial ready state
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected initial status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if body["status"] != "ready" {
		t.Errorf("expected status 'ready', got %q", body["status"])
	}

	// 2. Draining state
	srv.SetDraining(true)

	reqDraining := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recDraining := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDraining, reqDraining)

	if recDraining.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected draining status 503, got %d", recDraining.Code)
	}

	var bodyDraining map[string]string
	if err := json.Unmarshal(recDraining.Body.Bytes(), &bodyDraining); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if bodyDraining["status"] != "draining" {
		t.Errorf("expected status 'draining', got %q", bodyDraining["status"])
	}
}

func TestJSONErrorResponses(t *testing.T) {
	srv, _ := newTestServer(nil)

	t.Run("404 Not Found returns JSON error and preserves request ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent-route-xyz", nil)
		req.Header.Set("X-Request-ID", "custom-404-id")
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}

		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected application/json Content-Type, got %q", ct)
		}

		if rec.Header().Get("X-Request-ID") != "custom-404-id" {
			t.Errorf("expected X-Request-ID custom-404-id, got %q", rec.Header().Get("X-Request-ID"))
		}

		var errBody map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
			t.Fatalf("failed to parse JSON error: %v (body: %s)", err, rec.Body.String())
		}

		if errBody["code"] != "NOT_FOUND" {
			t.Errorf("expected code NOT_FOUND, got %q", errBody["code"])
		}
		if errBody["request_id"] != "custom-404-id" {
			t.Errorf("expected request_id custom-404-id, got %q", errBody["request_id"])
		}
	})

	t.Run("405 Method Not Allowed returns JSON error, preserves Allow and request ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
		req.Header.Set("X-Request-ID", "custom-405-id")
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected status 405, got %d", rec.Code)
		}

		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected application/json Content-Type, got %q", ct)
		}

		allowHeader := rec.Header().Get("Allow")
		if !strings.Contains(allowHeader, "GET") {
			t.Errorf("expected Allow header containing GET, got %q", allowHeader)
		}

		if rec.Header().Get("X-Request-ID") != "custom-405-id" {
			t.Errorf("expected X-Request-ID custom-405-id, got %q", rec.Header().Get("X-Request-ID"))
		}

		var errBody map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
			t.Fatalf("failed to parse JSON error: %v (body: %s)", err, rec.Body.String())
		}

		if errBody["code"] != "METHOD_NOT_ALLOWED" {
			t.Errorf("expected code METHOD_NOT_ALLOWED, got %q", errBody["code"])
		}
		if errBody["request_id"] != "custom-405-id" {
			t.Errorf("expected request_id custom-405-id, got %q", errBody["request_id"])
		}
	})
}

func TestPanicRecoveryOnAssembledServer(t *testing.T) {
	t.Run("panic before headers committed emits clean JSON 500 error", func(t *testing.T) {
		logBuf := &syncBuffer{}
		srv, _ := newTestServer(logBuf)

		srv.RegisterHandler("GET /test/panic/pre-commit", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("simulated fatal handler crash before write")
		}))

		req := httptest.NewRequest(http.MethodGet, "/test/panic/pre-commit", nil)
		req.Header.Set("X-Request-ID", "panic-trace-pre-123")
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rec.Code)
		}

		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected application/json, got %q", ct)
		}

		if rec.Header().Get("X-Request-ID") != "panic-trace-pre-123" {
			t.Errorf("expected X-Request-ID panic-trace-pre-123, got %q", rec.Header().Get("X-Request-ID"))
		}

		var errBody map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
			t.Fatalf("failed to decode JSON error body: %v", err)
		}
		if errBody["code"] != "INTERNAL_SERVER_ERROR" {
			t.Errorf("expected code INTERNAL_SERVER_ERROR, got %q", errBody["code"])
		}
		if errBody["request_id"] != "panic-trace-pre-123" {
			t.Errorf("expected request_id panic-trace-pre-123, got %q", errBody["request_id"])
		}

		logOutput := logBuf.String()
		if !strings.Contains(logOutput, "panic recovered in HTTP handler") {
			t.Errorf("expected panic log message in output: %s", logOutput)
		}
		if !strings.Contains(logOutput, `"request_id":"panic-trace-pre-123"`) {
			t.Errorf("expected request_id in panic log output: %s", logOutput)
		}

		mReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		mRec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(mRec, mReq)

		if !strings.Contains(mRec.Body.String(), `status_code="500"`) {
			t.Errorf("expected metrics to record status_code=500 on panic recovery: %s", mRec.Body.String())
		}
	})

	t.Run("panic after headers committed aborts connection to real HTTP client", func(t *testing.T) {
		logBuf := &syncBuffer{}
		srv, _ := newTestServer(logBuf)

		// Handler writes 200 OK, flushes partial chunk using ResponseController, then panics
		srv.RegisterHandler("GET /test/panic/post-commit", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"partial":"chunk"}`))

			rc := http.NewResponseController(w)
			if err := rc.Flush(); err != nil {
				panic("flush failed: " + err.Error())
			}
			panic("simulated fatal crash after headers committed")
		}))

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		serverAddr := ln.Addr().String()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = srv.Serve(ln)
		}()

		var shutdownOnce sync.Once
		safeShutdown := func() {
			shutdownOnce.Do(func() {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				if err := srv.Shutdown(cleanupCtx); err != nil {
					_ = srv.Close()
				}
				_ = ln.Close()
				wg.Wait()
			})
		}
		t.Cleanup(safeShutdown)

		// Make request using real HTTP client
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get("http://" + serverAddr + "/test/panic/post-commit")
		if err != nil {
			t.Fatalf("client.Get failed unexpectedly: %v", err)
		}
		defer resp.Body.Close()

		// 1. Client must receive status 200 OK because headers were committed and flushed
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
		}

		// 2. Client must receive partial flushed chunk before stream abort
		buf := make([]byte, 64)
		n, readErr := resp.Body.Read(buf)
		partialReceived := string(buf[:n])
		if !strings.Contains(partialReceived, `{"partial":"chunk"}`) {
			t.Errorf("expected client to receive partial flushed chunk, got: %q", partialReceived)
		}

		// 3. Subsequent read must fail with connection abort/reset
		var terminalErr error
		if readErr != nil {
			terminalErr = readErr
		} else {
			_, terminalErr = io.ReadAll(resp.Body)
		}
		if terminalErr == nil {
			t.Fatal("expected reading aborted body to fail, but read succeeded without error")
		}

		// Shutdown server and wait for all connection goroutines to finish before checking log buffer
		safeShutdown()

		// Verify slog recorded the abort
		logOutput := logBuf.String()
		if !strings.Contains(logOutput, "panic recovered in HTTP handler after response headers committed; aborting connection") {
			t.Errorf("expected abort log message in output: %s", logOutput)
		}
	})
}

func TestMetricsBoundedMethodLabels(t *testing.T) {
	srv, _ := newTestServer(nil)

	req := httptest.NewRequest("ARBITRARY_METHOD", "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	mReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(mRec, mReq)

	metricsOutput := mRec.Body.String()

	if strings.Contains(metricsOutput, `method="ARBITRARY_METHOD"`) {
		t.Errorf("arbitrary method leaked into Prometheus metrics labels: %s", metricsOutput)
	}

	if !strings.Contains(metricsOutput, `method="OTHER"`) {
		t.Errorf("expected method to be normalized to 'OTHER' in metrics: %s", metricsOutput)
	}
}

// openAPIDocument represents the top-level structural schema of an OpenAPI 3.1 contract.
type openAPIDocument struct {
	OpenAPI    string                    `yaml:"openapi"`
	Info       map[string]any            `yaml:"info"`
	Servers    []map[string]any          `yaml:"servers"`
	Paths      map[string]map[string]any `yaml:"paths"`
	Components struct {
		Headers   map[string]any `yaml:"headers"`
		Schemas   map[string]any `yaml:"schemas"`
		Responses map[string]any `yaml:"responses"`
	} `yaml:"components"`
}

// TestOpenAPIYAMLAndStructure performs YAML syntax and top-level structural validation of the
// embedded OpenAPI contract, and executes contract calls against each declared path.
// Note: This is a structural schema check; it does not perform full JSON Schema meta-schema
// validation or deep $ref graph resolution.
func TestOpenAPIYAMLAndStructure(t *testing.T) {
	srv, _ := newTestServer(nil)

	// 1. Fetch raw spec from server endpoint
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from /openapi.yaml, got %d", rec.Code)
	}

	rawYAML := rec.Body.Bytes()

	// 2. Structurally parse YAML document
	var doc openAPIDocument
	if err := yaml.Unmarshal(rawYAML, &doc); err != nil {
		t.Fatalf("OpenAPI document failed YAML syntax validation: %v", err)
	}

	// 3. Validate OpenAPI version and metadata
	if !strings.HasPrefix(doc.OpenAPI, "3.1") {
		t.Errorf("expected OpenAPI 3.1.x, got %q", doc.OpenAPI)
	}
	if len(doc.Servers) == 0 || doc.Servers[0]["url"] != "/" {
		t.Errorf("expected relative server URL '/', got: %v", doc.Servers)
	}

	// 4. Validate all expected paths are declared
	expectedPaths := []string{"/healthz", "/readyz", "/metrics", "/openapi.yaml", "/docs"}
	for _, p := range expectedPaths {
		if _, ok := doc.Paths[p]; !ok {
			t.Errorf("OpenAPI spec missing declared path: %s", p)
		}
	}

	// 5. Validate reusable components (schemas, headers, responses)
	if _, ok := doc.Components.Schemas["ErrorResponse"]; !ok {
		t.Error("OpenAPI spec missing ErrorResponse schema")
	}
	if _, ok := doc.Components.Headers["X-Request-ID"]; !ok {
		t.Error("OpenAPI spec missing X-Request-ID header definition")
	}
	if _, ok := doc.Components.Headers["Allow"]; !ok {
		t.Error("OpenAPI spec missing Allow header definition")
	}
	if _, ok := doc.Components.Responses["NotFound"]; !ok {
		t.Error("OpenAPI spec missing NotFound response definition")
	}
	if _, ok := doc.Components.Responses["MethodNotAllowed"]; !ok {
		t.Error("OpenAPI spec missing MethodNotAllowed response definition")
	}
	if _, ok := doc.Components.Responses["InternalServerError"]; !ok {
		t.Error("OpenAPI spec missing InternalServerError response definition")
	}

	// 6. Contract execution: execute every documented endpoint and verify it returns HTTP 200
	for _, p := range expectedPaths {
		t.Run("Contract execution: GET "+p, func(t *testing.T) {
			pathReq := httptest.NewRequest(http.MethodGet, p, nil)
			pathRec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(pathRec, pathReq)

			if pathRec.Code != http.StatusOK {
				t.Errorf("endpoint %s returned status %d, want 200", p, pathRec.Code)
			}
			if pathRec.Header().Get("X-Request-ID") == "" {
				t.Errorf("endpoint %s did not return X-Request-ID header", p)
			}
		})
	}

	// 7. Interactive docs UI loads specification URL
	t.Run("Docs UI serves correct spec URL", func(t *testing.T) {
		docsReq := httptest.NewRequest(http.MethodGet, "/docs", nil)
		docsRec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(docsRec, docsReq)

		if docsRec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", docsRec.Code)
		}
		body := docsRec.Body.String()
		if !strings.Contains(body, `data-url="/openapi.yaml"`) {
			t.Errorf("expected docs UI to reference data-url='/openapi.yaml', got: %s", body)
		}
	})
}

func TestRealListenerGracefulShutdown(t *testing.T) {
	t.Run("in-flight requests complete cleanly before shutdown deadline", func(t *testing.T) {
		cfg := config.DefaultConfig().HTTP
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		metrics := httpapi.NewMetrics()
		srv := httpapi.NewServer(cfg, logger, metrics)

		// Create real TCP listener on ephemeral port
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		serverAddr := ln.Addr().String()

		inFlightStarted := make(chan struct{})
		releaseInFlight := make(chan struct{})

		var releaseOnce sync.Once
		safeRelease := func() {
			releaseOnce.Do(func() { close(releaseInFlight) })
		}

		srv.RegisterHandler("GET /test/slow", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(inFlightStarted)
			<-releaseInFlight
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":"completed"}`))
		}))

		var wg sync.WaitGroup

		// Register failure-path cleanup immediately with bounded deadline
		t.Cleanup(func() {
			safeRelease()
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			if err := srv.Shutdown(cleanupCtx); err != nil {
				_ = srv.Close()
			}
			_ = ln.Close()
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Error("timed out waiting for goroutines during graceful shutdown cleanup")
			}
		})

		// Start server loop
		wg.Add(1)
		serverErr := make(chan error, 1)
		go func() {
			defer wg.Done()
			serverErr <- srv.Serve(ln)
		}()

		// Verify initial readiness returns 200 OK
		initialRec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(initialRec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if initialRec.Code != http.StatusOK {
			t.Fatalf("expected initial readiness 200, got %d", initialRec.Code)
		}

		// Launch in-flight request over real TCP connection
		wg.Add(1)
		client := &http.Client{Timeout: 5 * time.Second}
		clientErr := make(chan error, 1)
		clientRespCode := make(chan int, 1)
		clientBody := make(chan string, 1)

		go func() {
			defer wg.Done()
			resp, err := client.Get("http://" + serverAddr + "/test/slow")
			if err != nil {
				clientErr <- err
				return
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			clientRespCode <- resp.StatusCode
			clientBody <- string(b)
			clientErr <- nil
		}()

		// Wait until request is actively processing inside the server handler
		select {
		case <-inFlightStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for in-flight request to start")
		}

		// Initiate graceful shutdown
		wg.Add(1)
		shutdownDone := make(chan error, 1)
		go func() {
			defer wg.Done()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer shutdownCancel()
			shutdownDone <- srv.Shutdown(shutdownCtx)
		}()

		// Synchronize on draining state explicitly
		drainingReached := false
		for i := 0; i < 100; i++ {
			if srv.IsDraining() {
				drainingReached = true
				break
			}
			time.Sleep(2 * time.Millisecond)
		}
		if !drainingReached {
			t.Fatal("timed out waiting for server to enter draining state")
		}

		// Assert readiness through the assembled handler without listener races
		readyRec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(readyRec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if readyRec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503 while draining, got %d", readyRec.Code)
		}

		// Release the in-flight request to finish processing
		safeRelease()

		// Assert in-flight request finishes cleanly with 200 OK and expected body
		select {
		case err := <-clientErr:
			if err != nil {
				t.Fatalf("in-flight client request failed: %v", err)
			}
			if code := <-clientRespCode; code != http.StatusOK {
				t.Errorf("expected in-flight request to succeed with 200, got %d", code)
			}
			if b := <-clientBody; !strings.Contains(b, "completed") {
				t.Errorf("expected in-flight body to contain 'completed', got: %s", b)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for in-flight client to finish")
		}

		// Assert shutdown completes cleanly with nil error
		select {
		case err := <-shutdownDone:
			if err != nil {
				t.Fatalf("srv.Shutdown() failed: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for srv.Shutdown() to return")
		}

		// Assert server reported clean closure
		err = <-serverErr
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("expected ErrServerClosed, got %v", err)
		}
	})

	t.Run("shutdown respects deadline when in-flight requests exceed timeout", func(t *testing.T) {
		cfg := config.DefaultConfig().HTTP
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		metrics := httpapi.NewMetrics()
		srv := httpapi.NewServer(cfg, logger, metrics)

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		serverAddr := ln.Addr().String()

		inFlightStarted := make(chan struct{})
		hangRequest := make(chan struct{})

		var hangOnce sync.Once
		safeUnblockHang := func() {
			hangOnce.Do(func() { close(hangRequest) })
		}

		srv.RegisterHandler("GET /test/hang", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(inFlightStarted)
			<-hangRequest
		}))

		var wg sync.WaitGroup

		// Register failure-path cleanup immediately with bounded deadline
		t.Cleanup(func() {
			safeUnblockHang()
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			if err := srv.Shutdown(cleanupCtx); err != nil {
				_ = srv.Close()
			}
			_ = ln.Close()
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Error("timed out waiting for goroutines during hang cleanup")
			}
		})

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = srv.Serve(ln)
		}()

		wg.Add(1)
		client := &http.Client{}
		go func() {
			defer wg.Done()
			resp, _ := client.Get("http://" + serverAddr + "/test/hang")
			if resp != nil {
				_ = resp.Body.Close()
			}
		}()

		// Wait until in-flight request starts
		select {
		case <-inFlightStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for hang request to start")
		}

		// Short shutdown timeout: 50ms
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer shutdownCancel()

		err = srv.Shutdown(shutdownCtx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded on hung in-flight request, got: %v", err)
		}

		// Unblock the hanging request explicitly before test cleanup
		safeUnblockHang()
	})
}

func TestConcurrentRequests(t *testing.T) {
	srv, _ := newTestServer(nil)

	const concurrency = 20
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("worker %d: expected 200, got %d", idx, rec.Code)
			}
		}(i)
	}

	wg.Wait()
}
