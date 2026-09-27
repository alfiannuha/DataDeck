//go:build integration

package database_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

func pgConfig(t *testing.T) database.Config {
	t.Helper()
	host := os.Getenv("DATADECK_TEST_PG_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping PostgreSQL resilience test")
	}
	port := 5432
	if raw := os.Getenv("DATADECK_TEST_PG_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_PG_PORT %q: %v", raw, err)
		}
		port = parsed
	}
	return database.Config{
		Driver:   model.DriverPostgres,
		Host:     host,
		Port:     port,
		Database: envOr("DATADECK_TEST_PG_DATABASE", "postgres"),
		Username: os.Getenv("DATADECK_TEST_PG_USER"),
		Password: os.Getenv("DATADECK_TEST_PG_PASSWORD"),
		SSLMode:  envOr("DATADECK_TEST_PG_SSLMODE", "disable"),
	}
}

func mysqlConfig(t *testing.T) database.Config {
	t.Helper()
	host := os.Getenv("DATADECK_TEST_MYSQL_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_MYSQL_HOST not set; skipping MySQL resilience test")
	}
	port := 3306
	if raw := os.Getenv("DATADECK_TEST_MYSQL_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_MYSQL_PORT %q: %v", raw, err)
		}
		port = parsed
	}
	return database.Config{
		Driver:   model.DriverMySQL,
		Host:     host,
		Port:     port,
		Database: envOr("DATADECK_TEST_MYSQL_DATABASE", "datadeck_test"),
		Username: envOr("DATADECK_TEST_MYSQL_USER", "root"),
		Password: os.Getenv("DATADECK_TEST_MYSQL_PASSWORD"),
		SSLMode:  envOr("DATADECK_TEST_MYSQL_SSLMODE", "disable"),
	}
}

// recoverQuery runs a trivial query, retrying briefly so a pool that lost its
// backend has time to establish a fresh connection. It still fails the test if
// the pool never recovers.
func recoverQuery(t *testing.T, manager *database.Manager, id string, cfg database.Config) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, lastErr = manager.Execute(ctx, id, cfg, `SELECT 1`)
		cancel()
		if lastErr == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("pool did not recover: %v", lastErr)
}

func TestPostgresResilience(t *testing.T) {
	cfg := pgConfig(t)
	ctx := context.Background()
	manager := database.NewManager(database.DefaultOptions(), database.Postgres{})
	if _, err := manager.Open(ctx, "pg", cfg); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	t.Run("client cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(250 * time.Millisecond)
			cancel()
		}()
		_, err := manager.Execute(cancelCtx, "pg", cfg, `SELECT pg_sleep(5)`)
		cancel()
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want context cancellation", err)
		}
		// The pool must remain usable after a canceled query.
		if _, err := manager.Execute(ctx, "pg", cfg, `SELECT 1`); err != nil {
			t.Fatalf("post-cancel execute error = %v", err)
		}
	})

	t.Run("connection loss and recovery", func(t *testing.T) {
		// Terminate our own backend; the next query must fail safely, and the
		// one after must succeed on a fresh pooled connection.
		_, _ = manager.Execute(ctx, "pg", cfg, `SELECT pg_terminate_backend(pg_backend_pid())`)
		recoverQuery(t, manager, "pg", cfg)
	})

	t.Run("edge values", func(t *testing.T) {
		result, err := manager.Execute(ctx, "pg", cfg, `SELECT
			'-9223372036854775808'::bigint AS mn,
			9223372036854775807::bigint AS mx,
			123456789012345678901234567890.123456789::numeric AS bigdec,
			repeat('z', 200000)::text AS bigtext,
			jsonb_build_object('k', repeat('v', 1000))::jsonb AS bigjson,
			'\xdeadbeef'::bytea AS bin`)
		if err != nil {
			t.Fatalf("edge execute error = %v", err)
		}
		row := result.Rows[0]
		if row[0] != "-9223372036854775808" || row[1] != "9223372036854775807" {
			t.Errorf("int64 extremes = %v / %v", row[0], row[1])
		}
		if s, ok := row[2].(string); !ok || !strings.HasPrefix(s, "123456789012345678901234567890") {
			t.Errorf("huge numeric = %v", row[2])
		}
		if s, ok := row[3].(string); !ok || len(s) != 200000 {
			t.Errorf("large text length = %d", len(s))
		}
		if raw, ok := row[4].(json.RawMessage); ok {
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil || decoded["k"] == nil {
				t.Errorf("large json did not decode: %v (err=%v)", decoded, err)
			}
		} else {
			t.Errorf("large json type = %T", row[4])
		}
		if s, ok := row[5].(string); !ok || s == "" {
			t.Errorf("binary = %v", row[5])
		}
	})
}

func TestMySQLResilience(t *testing.T) {
	cfg := mysqlConfig(t)
	ctx := context.Background()
	manager := database.NewManager(database.DefaultOptions(), database.MySQL{})
	if _, err := manager.Open(ctx, "my", cfg); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	t.Run("client cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(250 * time.Millisecond)
			cancel()
		}()
		_, err := manager.Execute(cancelCtx, "my", cfg, `SELECT SLEEP(5)`)
		cancel()
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want context cancellation", err)
		}
		if _, err := manager.Execute(ctx, "my", cfg, `SELECT 1`); err != nil {
			t.Fatalf("post-cancel execute error = %v", err)
		}
	})

	t.Run("connection loss and recovery", func(t *testing.T) {
		_, _ = manager.Execute(ctx, "my", cfg, `KILL CONNECTION_ID()`)
		recoverQuery(t, manager, "my", cfg)
	})

	t.Run("edge values", func(t *testing.T) {
		result, err := manager.Execute(ctx, "my", cfg, `SELECT
			CAST(18446744073709551615 AS UNSIGNED) AS umax,
			CAST(-9223372036854775808 AS SIGNED) AS smin,
			CAST(123456789012345678901234567890.12345 AS DECIMAL(40,5)) AS bigdec,
			REPEAT('z', 200000) AS bigtext,
			CAST('{"k":"v"}' AS JSON) AS js,
			UNHEX('DEADBEEF') AS bin`)
		if err != nil {
			t.Fatalf("edge execute error = %v", err)
		}
		row := result.Rows[0]
		if row[0] != "18446744073709551615" {
			t.Errorf("unsigned bigint = %v (%T)", row[0], row[0])
		}
		if row[1] != "-9223372036854775808" {
			t.Errorf("signed min bigint = %v (%T)", row[1], row[1])
		}
		if s, ok := row[2].(string); !ok || !strings.HasPrefix(s, "123456789012345678901234567890") {
			t.Errorf("huge decimal = %v", row[2])
		}
		if s, ok := row[3].(string); !ok || len(s) != 200000 {
			t.Errorf("large text length = %d", len(s))
		}
		if s, ok := row[5].(string); !ok || s == "" {
			t.Errorf("binary = %v", row[5])
		}
	})
}
