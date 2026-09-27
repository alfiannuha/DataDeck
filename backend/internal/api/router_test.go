package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/config"
	"github.com/datadeck/datadeck/backend/internal/security"
)

func newTestServer() *Server {
	cfg := config.Config{
		Host:               config.DefaultHost,
		Port:               0,
		StoragePath:        "./data/test.db",
		LogLevel:           slog.LevelInfo,
		Environment:        config.EnvTest,
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	credentials, err := security.NewCipher("0123456789abcdef0123456789abcdef")
	if err != nil {
		panic(err)
	}
	return NewServer(cfg, logger, credentials, nil, nil, nil, nil)
}

func TestHealthEndpoint(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	newTestServer().Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header missing from response")
	}

	var env struct {
		Success bool              `json:"success"`
		Data    map[string]string `json:"data"`
		Error   any               `json:"error"`
		Meta    map[string]any    `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !env.Success {
		t.Error("success = false, want true")
	}
	if env.Error != nil {
		t.Errorf("error = %v, want nil", env.Error)
	}
	if env.Data["status"] != "healthy" {
		t.Errorf("data.status = %q, want healthy", env.Data["status"])
	}
	if env.Meta == nil {
		t.Error("meta = nil, want empty object")
	}
}

func TestUnknownRouteNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil)
	newTestServer().Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUnsupportedMethodIsRejected(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodPatch} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/v1/connections", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		newTestServer().Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestSecurityHeadersPresentOnAPIResponses(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	newTestServer().Router().ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}
