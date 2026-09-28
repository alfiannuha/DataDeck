//go:build integration

package database_test

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// TestMySQLIntegration exercises MySQL connection handling against a real
// server. It skips (NOT RUN) when DATADECK_TEST_MYSQL_HOST is unset, so the
// default `go test ./...` never requires MySQL.
func TestMySQLIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_MYSQL_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_MYSQL_HOST not set; skipping MySQL integration test")
	}

	port := 3306
	if raw := os.Getenv("DATADECK_TEST_MYSQL_PORT"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_MYSQL_PORT %q: %v", raw, err)
		}
		port = value
	}

	cfg := database.Config{
		Driver:   model.DriverMySQL,
		Host:     host,
		Port:     port,
		Database: envOr("DATADECK_TEST_MYSQL_DATABASE", "datadeck_test"),
		Username: envOr("DATADECK_TEST_MYSQL_USER", "root"),
		Password: os.Getenv("DATADECK_TEST_MYSQL_PASSWORD"),
		SSLMode:  envOr("DATADECK_TEST_MYSQL_SSLMODE", "disable"),
	}

	ctx := context.Background()
	manager := database.NewManager(database.DefaultOptions(), database.MySQL{})
	defer func() { _ = manager.CloseAll() }()

	if err := manager.Test(ctx, cfg); err != nil {
		t.Fatalf("Test() with valid settings error = %v", err)
	}

	db, err := manager.Open(ctx, "mit", cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if db == nil {
		t.Fatal("Open() returned nil pool")
	}
	if _, ok := manager.Get("mit", cfg.Database); !ok {
		t.Error("Get() did not return the active pool")
	}
	if err := manager.Close("mit", cfg.Database); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	t.Run("invalid credentials", func(t *testing.T) {
		bad := cfg
		bad.Password = "definitely-wrong-password"
		bad.Username = "definitely_wrong_user"
		if err := manager.Test(ctx, bad); !errors.Is(err, database.ErrConnection) {
			t.Errorf("Test(invalid credentials) error = %v, want ErrConnection", err)
		}
	})

	t.Run("unreachable host", func(t *testing.T) {
		unreachable := cfg
		unreachable.Host = "127.0.0.1"
		unreachable.Port = 1
		if err := manager.Test(ctx, unreachable); !errors.Is(err, database.ErrConnection) {
			t.Errorf("Test(unreachable) error = %v, want ErrConnection", err)
		}
	})
}
