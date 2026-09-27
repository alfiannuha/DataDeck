package repository

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/storage"
)

// seedHistory inserts a connection profile and count history rows directly so
// pagination benchmarks have a realistic data volume quickly.
func seedHistory(b *testing.B, db *sql.DB, count int) {
	b.Helper()
	if _, err := db.Exec(
		`INSERT INTO connection_profiles (id, name, driver, database_name) VALUES ('c1', 'bench', 'sqlite', '/tmp/x.db')`,
	); err != nil {
		b.Fatalf("seed profile: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		b.Fatalf("begin: %v", err)
	}
	stmt, err := tx.Prepare(
		`INSERT INTO query_history (id, connection_id, sql_text, status, execution_time_ms, rows_affected)
		 VALUES (?, 'c1', ?, 'SUCCESS', 5, 1)`,
	)
	if err != nil {
		b.Fatalf("prepare: %v", err)
	}
	for i := 0; i < count; i++ {
		if _, err := stmt.Exec(fmt.Sprintf("h%08d", i), fmt.Sprintf("SELECT %d", i)); err != nil {
			b.Fatalf("insert history %d: %v", i, err)
		}
	}
	_ = stmt.Close()
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit: %v", err)
	}
}

// BenchmarkHistoryListPage measures a single page read at the start and deep
// into a 10k-row history, evidencing that cost is per-page, not per-dataset.
func BenchmarkHistoryListPage(b *testing.B) {
	store, err := storage.Open(context.Background(), filepath.Join(b.TempDir(), "store.db"))
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	seedHistory(b, store.DB(), 10_000)

	repo := NewQueryHistoryRepository(store.DB())
	ctx := context.Background()
	cases := map[string]int{"page_1": 0, "page_100": 9_900, "page_200": 19_800}
	for name, offset := range cases {
		offset := offset
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				rows, err := repo.List(ctx, HistoryFilter{Limit: 50, Offset: offset})
				if err != nil {
					b.Fatalf("list: %v", err)
				}
				if offset < 10_000 && len(rows) != 50 {
					b.Fatalf("rows = %d, want 50", len(rows))
				}
			}
		})
	}
}

// BenchmarkSavedQueryListPage mirrors the history benchmark for saved queries.
func BenchmarkSavedQueryListPage(b *testing.B) {
	store, err := storage.Open(context.Background(), filepath.Join(b.TempDir(), "store.db"))
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := store.DB().Exec(
		`INSERT INTO connection_profiles (id, name, driver, database_name) VALUES ('c1', 'bench', 'sqlite', '/tmp/x.db')`,
	); err != nil {
		b.Fatalf("seed profile: %v", err)
	}
	tx, _ := store.DB().Begin()
	stmt, _ := tx.Prepare(
		`INSERT INTO saved_queries (id, connection_id, title, sql_text, tags) VALUES (?, 'c1', ?, ?, 'bench')`,
	)
	for i := 0; i < 10_000; i++ {
		if _, err := stmt.Exec(fmt.Sprintf("s%08d", i), fmt.Sprintf("Query %d", i), fmt.Sprintf("SELECT %d", i)); err != nil {
			b.Fatalf("insert saved %d: %v", i, err)
		}
	}
	_ = stmt.Close()
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit: %v", err)
	}

	repo := NewSavedQueryRepository(store.DB())
	ctx := context.Background()
	for name, offset := range map[string]int{"page_1": 0, "page_100": 9_900} {
		offset := offset
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := repo.List(ctx, 50, offset); err != nil {
					b.Fatalf("list: %v", err)
				}
			}
		})
	}
}
