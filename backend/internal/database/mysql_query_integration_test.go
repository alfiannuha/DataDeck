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

// TestMySQLQueryIntegration exercises MySQL query execution through the shared
// engine. Skips (NOT RUN) without DATADECK_TEST_MYSQL_HOST.
func TestMySQLQueryIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_MYSQL_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_MYSQL_HOST not set; skipping MySQL query integration test")
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
	db, err := manager.Open(ctx, "mq", cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	for _, stmt := range []string{
		`DROP TABLE IF EXISTS q_types`,
		`CREATE TABLE q_types (
			id BIGINT PRIMARY KEY,
			ubig BIGINT UNSIGNED NULL,
			dec_col DECIMAL(20,4) NULL,
			dbl DOUBLE NULL,
			txt VARCHAR(64) NULL,
			js JSON NULL,
			ts DATETIME NULL,
			bin VARBINARY(8) NULL
		)`,
		`INSERT INTO q_types (id, ubig, dec_col, dbl, txt, js, ts, bin) VALUES
			(9007199254740993, 18446744073709551615, 12345.6789, 1.5, 'hello', '{"a":1}', '2026-09-25 14:32:00', X'00FF'),
			(2, 1, 0.0001, 0.0, NULL, NULL, NULL, NULL)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	runQuery := func(sql string) (model.QueryResult, error) {
		return manager.Execute(ctx, "mq", cfg, sql)
	}

	t.Run("scalars null and types", func(t *testing.T) {
		result, err := runQuery(`SELECT id, ubig, dec_col, dbl, txt, js, ts, bin FROM q_types ORDER BY id`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if len(result.Columns) != 8 || len(result.Rows) != 2 {
			t.Fatalf("columns=%d rows=%d", len(result.Columns), len(result.Rows))
		}
		// Row 1 (id=2) has NULLs in txt/js/ts/bin.
		tail := result.Rows[0]
		if tail[4] != nil || tail[5] != nil || tail[6] != nil || tail[7] != nil {
			t.Errorf("expected NULLs, got %v", tail)
		}
	})

	t.Run("bigint and unsigned bigint precision", func(t *testing.T) {
		result, err := runQuery(`SELECT id, ubig FROM q_types WHERE id = 9007199254740993`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if got := result.Rows[0][0]; got != "9007199254740993" {
			t.Errorf("bigint = %v (%T), want string", got, got)
		}
		if got := result.Rows[0][1]; got != "18446744073709551615" {
			t.Errorf("unsigned bigint = %v (%T), want string", got, got)
		}
	})

	t.Run("decimal double json datetime binary", func(t *testing.T) {
		result, err := runQuery(`SELECT dec_col, dbl, js, ts, bin FROM q_types WHERE id = 9007199254740993`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		row := result.Rows[0]
		if row[0] != "12345.6789" {
			t.Errorf("decimal = %v, want string 12345.6789", row[0])
		}
		if row[1] != 1.5 {
			t.Errorf("double = %v, want 1.5", row[1])
		}
		if row[3] == nil || row[4] == nil {
			t.Errorf("datetime/binary = %v / %v", row[3], row[4])
		}
	})

	t.Run("zero rows", func(t *testing.T) {
		result, err := runQuery(`SELECT 1 WHERE 1 = 0`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if len(result.Rows) != 0 || result.Truncated {
			t.Errorf("rows=%d truncated=%v", len(result.Rows), result.Truncated)
		}
	})

	t.Run("non-row statements rows affected", func(t *testing.T) {
		if _, err := runQuery(`DELETE FROM q_types`); err != nil {
			t.Fatalf("delete error = %v", err)
		}
		insert, err := runQuery(`INSERT INTO q_types (id, txt) VALUES (10, 'a'), (11, 'b'), (12, 'c')`)
		if err != nil {
			t.Fatalf("insert error = %v", err)
		}
		if insert.RowsAffected != 3 {
			t.Errorf("insert rows_affected = %d, want 3", insert.RowsAffected)
		}
		update, err := runQuery(`UPDATE q_types SET txt = 'x' WHERE id >= 11`)
		if err != nil {
			t.Fatalf("update error = %v", err)
		}
		if update.RowsAffected != 2 {
			t.Errorf("update rows_affected = %d, want 2", update.RowsAffected)
		}
		deleted, err := runQuery(`DELETE FROM q_types WHERE id = 10`)
		if err != nil {
			t.Fatalf("delete error = %v", err)
		}
		if deleted.RowsAffected != 1 {
			t.Errorf("delete rows_affected = %d, want 1", deleted.RowsAffected)
		}
	})

	t.Run("syntax error", func(t *testing.T) {
		_, err := runQuery(`SELCT 1`)
		var sqlErr *database.SQLError
		if !errors.As(err, &sqlErr) {
			t.Fatalf("error = %v, want *database.SQLError", err)
		}
		if !sqlErr.Syntax {
			t.Errorf("expected a syntax-classified error, got %+v", sqlErr)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		_, err := manager.Execute(timeoutCtx, "mq", cfg, `SELECT SLEEP(5)`)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("truncation", func(t *testing.T) {
		result, err := runQuery(`SELECT REPEAT('x', 1024) AS v
			FROM information_schema.COLUMNS a JOIN information_schema.COLUMNS b
			LIMIT 60000`)
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
