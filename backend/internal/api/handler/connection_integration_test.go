//go:build integration

package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/api/handler"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/security"
	"github.com/datadeck/datadeck/backend/internal/storage"
)

// TestConnectionTestIntegration verifies the /connections/test endpoint against
// a real PostgreSQL server. Opt-in via build tag; skips when
// DATADECK_TEST_PG_HOST is unset. See postgres_integration_test.go in the
// database package for the environment variables.
func TestConnectionTestIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_PG_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping connection API integration test")
	}

	port := 5432
	if raw := os.Getenv("DATADECK_TEST_PG_PORT"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_PG_PORT %q: %v", raw, err)
		}
		port = value
	}

	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "datadeck.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer func() { _ = store.Close() }()

	cipher, err := security.NewCipher("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	manager := database.NewManager(database.DefaultOptions(), database.Postgres{})
	h := handler.NewConnectionHandler(
		repository.NewConnectionRepository(store.DB()),
		manager,
		cipher,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	body := fmt.Sprintf(
		`{"driver":"postgres","host":%q,"port":%d,"database_name":%q,"username":%q,"password":%q,"ssl_mode":%q}`,
		host, port,
		envOr("DATADECK_TEST_PG_DATABASE", "postgres"),
		os.Getenv("DATADECK_TEST_PG_USER"),
		os.Getenv("DATADECK_TEST_PG_PASSWORD"),
		envOr("DATADECK_TEST_PG_SSLMODE", "disable"),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.Test(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var env struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if !env.Success {
		t.Errorf("success = false (body=%s)", rec.Body.String())
	}
	if err := manager.CloseAll(); err != nil {
		t.Errorf("CloseAll() error = %v", err)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
