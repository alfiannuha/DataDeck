package config

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

var envKeys = []string{"HOST", "PORT", "STORAGE_PATH", "LOG_LEVEL", "ENVIRONMENT", "CORS_ALLOWED_ORIGINS", "ENCRYPTION_KEY", "ALLOW_REMOTE"}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range envKeys {
		old, ok := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		t.Cleanup(func() {
			if ok {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Host != DefaultHost {
		t.Errorf("Host = %q, want %q", cfg.Host, DefaultHost)
	}
	if cfg.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.StoragePath != DefaultStoragePath {
		t.Errorf("StoragePath = %q, want %q", cfg.StoragePath, DefaultStoragePath)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelInfo)
	}
	if cfg.Environment != EnvDevelopment {
		t.Errorf("Environment = %q, want %q", cfg.Environment, EnvDevelopment)
	}
	if got := cfg.Addr(); got != "127.0.0.1:8080" {
		t.Errorf("Addr() = %q, want %q", got, "127.0.0.1:8080")
	}
	if cfg.EncryptionKey != "" {
		t.Errorf("EncryptionKey = %q, want empty when unset", cfg.EncryptionKey)
	}
}

func TestLoadEncryptionKey(t *testing.T) {
	clearEnv(t)
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.EncryptionKey != "0123456789abcdef0123456789abcdef" {
		t.Errorf("EncryptionKey = %q", cfg.EncryptionKey)
	}
}

func TestDefaultBindHostIsLoopback(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("default bind host = %q, want loopback 127.0.0.1", cfg.Host)
	}
	if !cfg.IsLoopback() {
		t.Errorf("IsLoopback() = false for default host %q", cfg.Host)
	}
}

func TestLoadOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("HOST", "0.0.0.0")
	t.Setenv("ALLOW_REMOTE", "1")
	t.Setenv("PORT", "9090")
	t.Setenv("STORAGE_PATH", "/tmp/datadeck-test.db")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want %q", cfg.Host, "0.0.0.0")
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.StoragePath != "/tmp/datadeck-test.db" {
		t.Errorf("StoragePath = %q", cfg.StoragePath)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelDebug)
	}
	if cfg.Environment != EnvProduction {
		t.Errorf("Environment = %q, want %q", cfg.Environment, EnvProduction)
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "https://app.example.com" {
		t.Errorf("CORSAllowedOrigins = %v", cfg.CORSAllowedOrigins)
	}
	if cfg.IsLoopback() {
		t.Errorf("IsLoopback() = true for host 0.0.0.0")
	}
}

func TestLoadInvalid(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
	}{
		{"port not a number", "PORT", "not-a-port"},
		{"port zero", "PORT", "0"},
		{"port negative", "PORT", "-1"},
		{"port too large", "PORT", "70000"},
		{"invalid log level", "LOG_LEVEL", "loud"},
		{"invalid environment", "ENVIRONMENT", "staging"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil, want non-nil error for %s=%q", tc.key, tc.value)
			}
		})
	}
}

func TestLoadRefusesRemoteBindingWithoutOptIn(t *testing.T) {
	clearEnv(t)
	t.Setenv("HOST", "0.0.0.0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want refusal for non-loopback host")
	}
	if !strings.Contains(err.Error(), "ALLOW_REMOTE") {
		t.Errorf("error = %v, want an actionable ALLOW_REMOTE message", err)
	}
}

func TestLoadAllowsRemoteBindingWithOptIn(t *testing.T) {
	clearEnv(t)
	t.Setenv("HOST", "0.0.0.0")
	t.Setenv("ALLOW_REMOTE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil with ALLOW_REMOTE", err)
	}
	if !cfg.AllowRemote {
		t.Error("AllowRemote = false, want true")
	}
	if cfg.IsLoopback() {
		t.Error("IsLoopback() = true for 0.0.0.0")
	}
}

func TestLoadTreatsFalsyAllowRemoteAsOptOut(t *testing.T) {
	for _, value := range []string{"0", "false", "no", "typo"} {
		t.Run(value, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("HOST", "192.0.2.10")
			t.Setenv("ALLOW_REMOTE", value)
			if _, err := Load(); err == nil {
				t.Fatalf("ALLOW_REMOTE=%q accepted a non-loopback bind", value)
			}
		})
	}
}
