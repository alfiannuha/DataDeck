package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

// Environment identifies the runtime mode DataDeck is running in.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
	EnvTest        Environment = "test"
)

// Default values for local-first execution. Host defaults to loopback so the
// daemon is not exposed to the local network unless explicitly overridden.
const (
	DefaultHost        = "127.0.0.1"
	DefaultPort        = 8080
	DefaultStoragePath = "./data/datadeck.db"
	DefaultLogLevel    = "info"
	DefaultEnvironment = EnvDevelopment
)

// Config is the typed runtime configuration for the backend daemon.
type Config struct {
	Host               string
	Port               int
	StoragePath        string
	LogLevel           slog.Level
	Environment        Environment
	CORSAllowedOrigins []string
	// AllowRemote records that the operator explicitly opted into binding
	// beyond loopback (ALLOW_REMOTE). It is informational once validated.
	AllowRemote bool
	// EncryptionKey is the raw key material for the credential cipher. It is
	// validated by the security package, never logged, and never returned via
	// the API.
	EncryptionKey string
}

// Addr returns the host:port address the HTTP server should bind to.
func (c Config) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// IsLoopback reports whether the configured host is a loopback address.
func (c Config) IsLoopback() bool {
	ip := net.ParseIP(c.Host)
	if ip != nil {
		return ip.IsLoopback()
	}
	// Hostnames such as "localhost" resolve to loopback.
	return strings.EqualFold(c.Host, "localhost")
}

// Load reads configuration from environment variables, applies safe defaults,
// and validates the result. It returns an error rather than starting with an
// invalid configuration.
func Load() (Config, error) {
	cfg := Config{
		Host:          envOr("HOST", DefaultHost),
		StoragePath:   envOr("STORAGE_PATH", DefaultStoragePath),
		Environment:   Environment(envOr("ENVIRONMENT", string(DefaultEnvironment))),
		EncryptionKey: envOr("ENCRYPTION_KEY", ""),
		AllowRemote:   parseBool(envOr("ALLOW_REMOTE", "")),
	}

	port, err := strconv.Atoi(envOr("PORT", strconv.Itoa(DefaultPort)))
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("invalid PORT %q: must be an integer between 1 and 65535", envOr("PORT", ""))
	}
	cfg.Port = port

	level, err := parseLogLevel(envOr("LOG_LEVEL", DefaultLogLevel))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = level

	cfg.CORSAllowedOrigins = splitList(envOr("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"))

	if strings.TrimSpace(cfg.Host) == "" {
		return Config{}, fmt.Errorf("HOST must not be empty")
	}
	if !cfg.IsLoopback() && !cfg.AllowRemote {
		return Config{}, fmt.Errorf(
			"refusing to bind to non-loopback host %q without ALLOW_REMOTE=1: DataDeck has no authentication and would be exposed to the network",
			cfg.Host,
		)
	}
	if strings.TrimSpace(cfg.StoragePath) == "" {
		return Config{}, fmt.Errorf("STORAGE_PATH must not be empty")
	}
	if !cfg.Environment.valid() {
		return Config{}, fmt.Errorf("invalid ENVIRONMENT %q: expected one of %q, %q, %q",
			cfg.Environment, EnvDevelopment, EnvProduction, EnvTest)
	}

	return cfg, nil
}

func (e Environment) valid() bool {
	switch e {
	case EnvDevelopment, EnvProduction, EnvTest:
		return true
	default:
		return false
	}
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid LOG_LEVEL %q: expected one of debug, info, warn, error", raw)
	}
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

// parseBool treats a small set of truthy strings as true; anything else (empty,
// "0", "false", typos) is false, which fails safe for security switches.
func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
