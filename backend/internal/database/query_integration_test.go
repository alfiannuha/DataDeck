//go:build integration

package database_test

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestPostgresQueryIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_PG_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping query integration test")
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
	if _, err := manager.Open(ctx, "q", cfg); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	runQuery := func(sql string) (model.QueryResult, error) {
		return manager.Execute(ctx, "q", cfg, sql)
	}

	t.Run("scalars and null", func(t *testing.T) {
		result, err := runQuery(`SELECT 1::int4 AS i, 'x'::text AS t, true AS b, 1.5::float8 AS f, NULL::text AS n`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if len(result.Columns) != 5 || len(result.Rows) != 1 {
			t.Fatalf("columns=%d rows=%d", len(result.Columns), len(result.Rows))
		}
		row := result.Rows[0]
		if row[4] != nil {
			t.Errorf("NULL column = %v, want nil", row[4])
		}
		if row[1] != "x" || row[2] != true || row[3] != 1.5 {
			t.Errorf("scalars = %v", row)
		}
	})

	t.Run("bigint precision", func(t *testing.T) {
		result, err := runQuery(`SELECT 9007199254740993::bigint AS big`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if got := result.Rows[0][0]; got != "9007199254740993" {
			t.Errorf("bigint = %v (%T), want string", got, got)
		}
	})

	t.Run("json uuid timestamp bytea", func(t *testing.T) {
		result, err := runQuery(`SELECT '{"a":1}'::jsonb AS j, '00112233-4455-6677-8899-aabbccddeeff'::uuid AS id,
			'2026-09-25T14:32:00Z'::timestamptz AS ts, '\x00ff'::bytea AS raw`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		row := result.Rows[0]
		if row[1] != "00112233-4455-6677-8899-aabbccddeeff" {
			t.Errorf("uuid = %v", row[1])
		}
		if row[2] == nil || row[3] == nil {
			t.Errorf("timestamp/bytea = %v / %v", row[2], row[3])
		}
	})

	t.Run("zero rows", func(t *testing.T) {
		result, err := runQuery(`SELECT 1 WHERE false`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if len(result.Columns) != 1 || len(result.Rows) != 0 {
			t.Errorf("columns=%d rows=%d", len(result.Columns), len(result.Rows))
		}
		if result.Truncated {
			t.Error("truncated = true, want false")
		}
	})

	t.Run("non-row statements", func(t *testing.T) {
		if _, err := runQuery(`CREATE TEMP TABLE datadeck_q (i int)`); err != nil {
			t.Fatalf("create temp table error = %v", err)
		}
		result, err := runQuery(`INSERT INTO datadeck_q (i) VALUES (1), (2), (3)`)
		if err != nil {
			t.Fatalf("insert error = %v", err)
		}
		if len(result.Columns) != 0 {
			t.Errorf("columns = %d, want 0", len(result.Columns))
		}
		if result.RowsAffected != 3 {
			t.Errorf("rows_affected = %d, want 3", result.RowsAffected)
		}
	})

	t.Run("syntax error", func(t *testing.T) {
		_, err := runQuery(`SELECT WHERRE`)
		var sqlErr *database.SQLError
		if !errors.As(err, &sqlErr) {
			t.Fatalf("error = %v, want *database.SQLError", err)
		}
		if sqlErr.Position == 0 {
			t.Error("syntax error position = 0, want a useful position")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		if _, err := manager.Execute(timeoutCtx, "q", cfg, `SELECT pg_sleep(5)`); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("truncation", func(t *testing.T) {
		result, err := runQuery(`SELECT repeat('x', 1024) AS v FROM generate_series(1, 60000)`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if !result.Truncated {
			t.Fatalf("truncated = false, want true (rows=%d)", len(result.Rows))
		}
		if len(result.Rows) >= 60000 {
			t.Errorf("rows = %d, expected truncation before the full set", len(result.Rows))
		}
	})
}
