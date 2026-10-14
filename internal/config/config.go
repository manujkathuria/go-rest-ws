package config

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the complete typed configuration for the service.
type Config struct {
	HTTP HTTPConfig
	Log  LogConfig
}

// HTTPConfig contains network and server timeout settings.
type HTTPConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Addr returns the host:port listen address.
func (h HTTPConfig) Addr() string {
	return net.JoinHostPort(h.Host, strconv.Itoa(h.Port))
}

// LogConfig contains settings for structured slog logging.
type LogConfig struct {
	Format string     // "json" or "text"
	Level  slog.Level // LevelDebug, LevelInfo, LevelWarn, LevelError
}

// DefaultConfig returns safe, production-sensible defaults.
func DefaultConfig() Config {
	return Config{
		HTTP: HTTPConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 15 * time.Second,
		},
		Log: LogConfig{
			Format: "json",
			Level:  slog.LevelInfo,
		},
	}
}

// Load loads configuration with precedence:
// 1. Defaults
// 2. Optional .env file (if present)
// 3. Environment variables (highest precedence)
func Load() (Config, error) {
	cfg := DefaultConfig()

	// Parse optional local .env file if it exists.
	// Ignore os.ErrNotExist so missing .env is not an error.
	if err := loadDotEnv(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("reading .env file: %w", err)
	}

	// Environment variable overrides
	if host := os.Getenv("HTTP_HOST"); host != "" {
		cfg.HTTP.Host = host
	}

	if portStr := os.Getenv("HTTP_PORT"); portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid HTTP_PORT %q: %w", portStr, err)
		}
		cfg.HTTP.Port = p
	}

	if rtStr := os.Getenv("HTTP_READ_TIMEOUT"); rtStr != "" {
		d, err := time.ParseDuration(rtStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid HTTP_READ_TIMEOUT %q: %w", rtStr, err)
		}
		cfg.HTTP.ReadTimeout = d
	}

	if wtStr := os.Getenv("HTTP_WRITE_TIMEOUT"); wtStr != "" {
		d, err := time.ParseDuration(wtStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid HTTP_WRITE_TIMEOUT %q: %w", wtStr, err)
		}
		cfg.HTTP.WriteTimeout = d
	}

	if itStr := os.Getenv("HTTP_IDLE_TIMEOUT"); itStr != "" {
		d, err := time.ParseDuration(itStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid HTTP_IDLE_TIMEOUT %q: %w", itStr, err)
		}
		cfg.HTTP.IdleTimeout = d
	}

	if stStr := os.Getenv("HTTP_SHUTDOWN_TIMEOUT"); stStr != "" {
		d, err := time.ParseDuration(stStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid HTTP_SHUTDOWN_TIMEOUT %q: %w", stStr, err)
		}
		cfg.HTTP.ShutdownTimeout = d
	}

	if fmtStr := os.Getenv("LOG_FORMAT"); fmtStr != "" {
		cfg.Log.Format = strings.ToLower(fmtStr)
	}

	if lvlStr := os.Getenv("LOG_LEVEL"); lvlStr != "" {
		var lvl slog.Level
		if err := lvl.UnmarshalText([]byte(lvlStr)); err != nil {
			return Config{}, fmt.Errorf("invalid LOG_LEVEL %q: %w", lvlStr, err)
		}
		cfg.Log.Level = lvl
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validating configuration: %w", err)
	}

	return cfg, nil
}

// Validate checks configuration sanity.
func (c Config) Validate() error {
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return fmt.Errorf("HTTP port must be between 1 and 65535, got %d", c.HTTP.Port)
	}
	if c.HTTP.ReadTimeout <= 0 {
		return errors.New("HTTP read timeout must be positive")
	}
	if c.HTTP.WriteTimeout <= 0 {
		return errors.New("HTTP write timeout must be positive")
	}
	if c.HTTP.IdleTimeout <= 0 {
		return errors.New("HTTP idle timeout must be positive")
	}
	if c.HTTP.ShutdownTimeout <= 0 {
		return errors.New("HTTP shutdown timeout must be positive")
	}
	if c.Log.Format != "json" && c.Log.Format != "text" {
		return fmt.Errorf("LOG_FORMAT must be 'json' or 'text', got %q", c.Log.Format)
	}
	return nil
}

// loadDotEnv reads key=value lines from a file and sets them in the environment
// only if the key is not already set in the current process environment.
func loadDotEnv(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("line %d: malformed .env entry, missing '=': %q", lineNum, line)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return fmt.Errorf("line %d: malformed .env entry, empty key: %q", lineNum, line)
		}
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)

		// Only populate if not already set in OS environment (OS env takes precedence)
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, val); err != nil {
				return fmt.Errorf("line %d: setting environment variable %q: %w", lineNum, key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading .env file: %w", err)
	}
	return nil
}
