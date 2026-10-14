package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/manujkathuria/go-rest-ws/internal/config"
)

// isolateEnv unsets all supported environment variables for the test duration
// and registers a cleanup function to restore their original values and remove
// any variables that were set during the test (including those from .env).
func isolateEnv(t *testing.T) {
	t.Helper()
	supportedVars := []string{
		"HTTP_HOST",
		"HTTP_PORT",
		"HTTP_READ_TIMEOUT",
		"HTTP_WRITE_TIMEOUT",
		"HTTP_IDLE_TIMEOUT",
		"HTTP_SHUTDOWN_TIMEOUT",
		"LOG_FORMAT",
		"LOG_LEVEL",
	}

	// Capture initial state
	originalEnv := make(map[string]*string, len(supportedVars))
	for _, k := range supportedVars {
		if val, exists := os.LookupEnv(k); exists {
			v := val
			originalEnv[k] = &v
			_ = os.Unsetenv(k)
		} else {
			originalEnv[k] = nil
		}
	}

	// Restore original state completely on test completion
	t.Cleanup(func() {
		for k, orig := range originalEnv {
			if orig != nil {
				_ = os.Setenv(k, *orig)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	isolateEnv(t)
	cfg := config.DefaultConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig() should be valid, got: %v", err)
	}

	if cfg.HTTP.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.HTTP.Port)
	}
	if cfg.HTTP.Addr() != "0.0.0.0:8080" {
		t.Errorf("expected addr 0.0.0.0:8080, got %s", cfg.HTTP.Addr())
	}
	if cfg.Log.Format != "json" {
		t.Errorf("expected log format 'json', got %s", cfg.Log.Format)
	}
}

func TestConfigValidation(t *testing.T) {
	isolateEnv(t)
	tests := []struct {
		name    string
		mutate  func(*config.Config)
		wantErr bool
	}{
		{
			name:    "valid defaults",
			mutate:  func(c *config.Config) {},
			wantErr: false,
		},
		{
			name: "invalid port zero",
			mutate: func(c *config.Config) {
				c.HTTP.Port = 0
			},
			wantErr: true,
		},
		{
			name: "invalid port too high",
			mutate: func(c *config.Config) {
				c.HTTP.Port = 70000
			},
			wantErr: true,
		},
		{
			name: "zero read timeout",
			mutate: func(c *config.Config) {
				c.HTTP.ReadTimeout = 0
			},
			wantErr: true,
		},
		{
			name: "negative write timeout",
			mutate: func(c *config.Config) {
				c.HTTP.WriteTimeout = -1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "invalid log format",
			mutate: func(c *config.Config) {
				c.Log.Format = "xml"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigEnvironmentOverrides(t *testing.T) {
	isolateEnv(t)

	t.Setenv("HTTP_HOST", "127.0.0.1")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("HTTP_READ_TIMEOUT", "15s")
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.HTTP.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.HTTP.Host)
	}
	if cfg.HTTP.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.HTTP.Port)
	}
	if cfg.HTTP.ReadTimeout != 15*time.Second {
		t.Errorf("expected read timeout 15s, got %v", cfg.HTTP.ReadTimeout)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("expected log format 'text', got %s", cfg.Log.Format)
	}
}

func TestConfigInvalidEnv(t *testing.T) {
	isolateEnv(t)

	t.Run("invalid port string", func(t *testing.T) {
		t.Setenv("HTTP_PORT", "invalid-port")
		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error on invalid HTTP_PORT, got nil")
		}
	})

	t.Run("invalid duration string", func(t *testing.T) {
		t.Setenv("HTTP_READ_TIMEOUT", "invalid-duration")
		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error on invalid HTTP_READ_TIMEOUT, got nil")
		}
	})

	t.Run("invalid log level", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "verbose_super")
		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error on invalid LOG_LEVEL, got nil")
		}
	})
}

func TestConfigDotEnvLoading(t *testing.T) {
	isolateEnv(t)

	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	envContent := []byte("HTTP_PORT=9999\nLOG_FORMAT=text\n")
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), envContent, 0644); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tmpDir: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.HTTP.Port != 9999 {
		t.Errorf("expected port 9999 from .env, got %d", cfg.HTTP.Port)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("expected log format text from .env, got %s", cfg.Log.Format)
	}
}

func TestConfigDotEnvMalformed(t *testing.T) {
	isolateEnv(t)

	t.Run("missing equals sign", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(originalWd) }()

		envContent := []byte("VALID_VAR=123\nMALFORMED_LINE_WITHOUT_EQUALS\n")
		if err := os.WriteFile(filepath.Join(tmpDir, ".env"), envContent, 0644); err != nil {
			t.Fatalf("failed to write test .env: %v", err)
		}
		_ = os.Chdir(tmpDir)

		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error on malformed .env line missing '=', got nil")
		}
		if !strings.Contains(err.Error(), "line 2: malformed .env entry, missing '='") {
			t.Errorf("expected line number and missing '=' error, got: %v", err)
		}
	})

	t.Run("empty key", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(originalWd) }()

		envContent := []byte("=somevalue\n")
		if err := os.WriteFile(filepath.Join(tmpDir, ".env"), envContent, 0644); err != nil {
			t.Fatalf("failed to write test .env: %v", err)
		}
		_ = os.Chdir(tmpDir)

		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error on malformed .env line with empty key, got nil")
		}
		if !strings.Contains(err.Error(), "line 1: malformed .env entry, empty key") {
			t.Errorf("expected empty key error, got: %v", err)
		}
	})
}

func TestConfigEnvironmentPrecedence(t *testing.T) {
	isolateEnv(t)

	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	// .env specifies port 9999 and host 127.0.0.1
	envContent := []byte("HTTP_PORT=9999\nHTTP_HOST=127.0.0.1\n")
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), envContent, 0644); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}
	_ = os.Chdir(tmpDir)

	// OS environment overrides port to 7777 (OS environment should win over .env)
	t.Setenv("HTTP_PORT", "7777")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	// Port should come from OS environment (7777), NOT .env (9999)
	if cfg.HTTP.Port != 7777 {
		t.Errorf("expected port 7777 from OS env precedence, got %d", cfg.HTTP.Port)
	}

	// Host should come from .env (127.0.0.1) since OS env didn't specify it
	if cfg.HTTP.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1 from .env, got %s", cfg.HTTP.Host)
	}
}
