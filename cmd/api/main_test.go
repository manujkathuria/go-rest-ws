package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/manujkathuria/go-rest-ws/internal/config"
)

func TestNewLogger(t *testing.T) {
	t.Run("json logger emits json", func(t *testing.T) {
		buf := &bytes.Buffer{}
		cfg := config.LogConfig{
			Format: "json",
			Level:  slog.LevelInfo,
		}
		logger := newLogger(buf, cfg)
		logger.Info("test json message", "key", "val")

		output := buf.String()
		if !strings.Contains(output, `"msg":"test json message"`) {
			t.Errorf("expected json message, got: %s", output)
		}
		if !strings.Contains(output, `"service":"go-rest-ws"`) {
			t.Errorf("expected service attribute in json, got: %s", output)
		}
	})

	t.Run("text logger emits text", func(t *testing.T) {
		buf := &bytes.Buffer{}
		cfg := config.LogConfig{
			Format: "text",
			Level:  slog.LevelInfo,
		}
		logger := newLogger(buf, cfg)
		logger.Info("test text message", "key", "val")

		output := buf.String()
		if !strings.Contains(output, "msg=\"test text message\"") {
			t.Errorf("expected text message, got: %s", output)
		}
		if !strings.Contains(output, "service=go-rest-ws") {
			t.Errorf("expected service attribute in text, got: %s", output)
		}
	})
}

func TestRunInvalidConfig(t *testing.T) {
	t.Setenv("HTTP_PORT", "99999") // Out of range

	buf := &bytes.Buffer{}
	err := run(buf, []string{"api"})
	if err == nil {
		t.Fatal("expected error on invalid config, got nil")
	}
	if !strings.Contains(err.Error(), "configuration error") {
		t.Errorf("expected configuration error, got: %v", err)
	}
}
