package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestSQLiteQueryExecution(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "queries.db")
	manager := NewManager(DefaultOptions(), SQLite{})
	cfg := Config{Driver: model.DriverSQLite, Database: path}

	if _, err := manager.Open(context.Background(), "s1", cfg); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	run := func(sql string) (model.QueryResult, error) {
		return manager.Execute(context.Background(), "s1", cfg, sql)
	}

	t.Run("ddl", func(t *testing.T) {
		result, err := run(`CREATE TABLE q (
			id INTEGER,
			big INTEGER,
			real_col REAL,
			txt TEXT,
			blob_col BLOB,
			nullable TEXT
		)`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if len(result.Columns) != 0 {
			t.Errorf("DDL columns = %d, want 0", len(result.Columns))
		}
	})

	t.Run("insert update delete rows affected", func(t *testing.T) {
		insert, err := run(`INSERT INTO q VALUES
			(1, 9007199254740993, 1.5, 'hello', X'00FF', NULL),
			(2, 42, 0.0, 'world', X'0102', 'x')`)
		if err != nil {
			t.Fatalf("insert error = %v", err)
		}
		if insert.RowsAffected != 2 {
			t.Errorf("insert rows_affected = %d, want 2", insert.RowsAffected)
		}
		update, err := run(`UPDATE q SET txt = 'x' WHERE id = 2`)
		if err != nil {
			t.Fatalf("update error = %v", err)
		}
		if update.RowsAffected != 1 {
			t.Errorf("update rows_affected = %d, want 1", update.RowsAffected)
		}
	})

	t.Run("select types and bigint precision", func(t *testing.T) {
		result, err := run(`SELECT id, big, real_col, txt, blob_col, nullable FROM q ORDER BY id`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if len(result.Columns) != 6 || len(result.Rows) != 2 {
			t.Fatalf("columns=%d rows=%d", len(result.Columns), len(result.Rows))
		}
		first := result.Rows[0]
		if first[0] != int64(1) {
			t.Errorf("id = %v (%T), want int64 1", first[0], first[0])
		}
		if first[1] != "9007199254740993" {
			t.Errorf("big = %v (%T), want string", first[1], first[1])
		}
		if first[2] != 1.5 {
			t.Errorf("real = %v, want 1.5", first[2])
		}
		if first[3] != "hello" {
			t.Errorf("txt = %v", first[3])
		}
		if first[4] != "AP8=" { // base64 of 0x00 0xFF
			t.Errorf("blob = %v, want base64 AP8=", first[4])
		}
		if first[5] != nil {
			t.Errorf("nullable = %v, want nil", first[5])
		}
		// Row 2's big value is within safe range and stays numeric.
		if result.Rows[1][1] != int64(42) {
			t.Errorf("safe integer = %v (%T), want int64", result.Rows[1][1], result.Rows[1][1])
		}
	})

	t.Run("zero rows", func(t *testing.T) {
		result, err := run(`SELECT 1 WHERE 1 = 0`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if len(result.Rows) != 0 || result.Truncated {
			t.Errorf("rows=%d truncated=%v", len(result.Rows), result.Truncated)
		}
	})

	t.Run("delete", func(t *testing.T) {
		result, err := run(`DELETE FROM q WHERE id = 1`)
		if err != nil {
			t.Fatalf("delete error = %v", err)
		}
		if result.RowsAffected != 1 {
			t.Errorf("delete rows_affected = %d, want 1", result.RowsAffected)
		}
	})

	t.Run("syntax error", func(t *testing.T) {
		_, err := run(`SELCT 1`)
		var sqlErr *SQLError
		if !errors.As(err, &sqlErr) {
			t.Fatalf("error = %v, want *SQLError", err)
		}
		if !sqlErr.Syntax {
			t.Errorf("expected syntax classification, got %+v", sqlErr)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := manager.Execute(timeoutCtx, "s1", cfg,
			`WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 100000000) SELECT count(*) FROM c`)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("truncation", func(t *testing.T) {
		result, err := run(`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 60000)
			SELECT hex(zeroblob(512)) AS v FROM seq`)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if !result.Truncated {
			t.Fatalf("truncated = false, want true (rows=%d)", len(result.Rows))
		}
		if len(result.Rows) >= 60000 {
			t.Errorf("rows = %d, expected truncation", len(result.Rows))
		}
	})
}
