package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/manujkathuria/go-rest-ws/internal/config"
	"github.com/manujkathuria/go-rest-ws/internal/httpapi"
)

func main() {
	if err := run(os.Stdout, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "service error: %v\n", err)
		os.Exit(1)
	}
}

// run encapsulates process startup, wiring, signal handling, and bounded shutdown.
func run(logOutput io.Writer, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	logger := newLogger(logOutput, cfg.Log)

	metrics := httpapi.NewMetrics()
	srv := httpapi.NewServer(cfg.HTTP, logger, metrics)

	// Root context listening for shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting HTTP service",
			"addr", cfg.HTTP.Addr(),
			"read_timeout", cfg.HTTP.ReadTimeout,
			"write_timeout", cfg.HTTP.WriteTimeout,
			"shutdown_timeout", cfg.HTTP.ShutdownTimeout,
		)
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining traffic")
	}

	// Bounded shutdown context that remains usable after root context cancellation
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	logger.Info("service terminated cleanly")
	return nil
}

// newLogger constructs a service-owned slog.Logger with standard service attributes.
func newLogger(w io.Writer, cfg config.LogConfig) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: cfg.Level,
	}

	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	return slog.New(handler).With("service", "go-rest-ws")
}
