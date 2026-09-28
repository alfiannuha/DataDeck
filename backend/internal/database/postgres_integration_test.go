//go:build integration

package database_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// TestPostgresIntegration exercises the manager against a real PostgreSQL
// server. It is opt-in: run with
//
//	go test -tags=integration ./internal/database/ -run TestPostgresIntegration
//
// and set DATADECK_TEST_PG_HOST (see the other DATADECK_TEST_PG_* variables for
// host/port/database/user/password/sslmode). It skips when no host is provided
// so the default `go test ./...` never requires a database.
func TestPostgresIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_PG_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping PostgreSQL integration test")
	}

	port := 5432
	if raw := os.Getenv("DATADECK_TEST_PG_PORT"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_PG_PORT %q: %v", raw, err)
		}
		port = value
	}

	cfg := database.Config{
		Driver:   model.DriverPostgres,
		Host:     host,
		Port:     port,
		Database: envOr("DATADECK_TEST_PG_DATABASE", "postgres"),
		Username: os.Getenv("DATADECK_TEST_PG_USER"),
		Password: os.Getenv("DATADECK_TEST_PG_PASSWORD"),
		SSLMode:  envOr("DATADECK_TEST_PG_SSLMODE", "disable"),
	}

	ctx := context.Background()
	manager := database.NewManager(database.DefaultOptions(), database.Postgres{})

	if err := manager.Test(ctx, cfg); err != nil {
		t.Fatalf("Test() error = %v", err)
	}

	db, err := manager.Open(ctx, "itest", cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if db == nil {
		t.Fatal("Open() returned nil pool")
	}
	if _, ok := manager.Get("itest", cfg.Database); !ok {
		t.Error("Get() did not return the active pool")
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
