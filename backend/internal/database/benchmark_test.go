package database

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// BenchmarkExecuteResultSizes measures end-to-end execution + serialization for
// representative result sizes against a real (temporary) SQLite target. It uses
// a recursive CTE so no external fixture is required.
func BenchmarkExecuteResultSizes(b *testing.B) {
	sizes := []int{1_000, 10_000, 50_000, 100_000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("rows_%d", size), func(b *testing.B) {
			cfg := Config{Driver: model.DriverSQLite, Database: filepath.Join(b.TempDir(), "bench.db")}
			manager := NewManager(DefaultOptions(), SQLite{})
			ctx := context.Background()
			if _, err := manager.Open(ctx, "bench", cfg); err != nil {
				b.Fatalf("open: %v", err)
			}
			defer func() { _ = manager.CloseAll() }()

			sql := fmt.Sprintf(
				`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n < %d)
				 SELECT n AS id, 'row-' || n AS label, n * 2 AS value FROM seq`,
				size,
			)

			// Warm up and record the serialized payload size once.
			result, err := manager.Execute(ctx, "bench", cfg, sql)
			if err != nil {
				b.Fatalf("execute: %v", err)
			}
			if len(result.Rows) != size {
				b.Fatalf("rows = %d, want %d", len(result.Rows), size)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				b.Fatalf("marshal: %v", err)
			}
			b.Logf("rows=%d json_bytes=%d", size, len(encoded))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := manager.Execute(ctx, "bench", cfg, sql); err != nil {
					b.Fatalf("execute: %v", err)
				}
			}
		})
	}
}

// BenchmarkIntrospectTableCounts measures schema introspection for schemas with
// 100, 500 and 1000 tables, and reports the JSON tree size.
func BenchmarkIntrospectTableCounts(b *testing.B) {
	for _, tables := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("tables_%d", tables), func(b *testing.B) {
			cfg := Config{Driver: model.DriverSQLite, Database: filepath.Join(b.TempDir(), "schema.db")}
			manager := NewManager(DefaultOptions(), SQLite{})
			ctx := context.Background()
			if _, err := manager.Open(ctx, "bench", cfg); err != nil {
				b.Fatalf("open: %v", err)
			}
			defer func() { _ = manager.CloseAll() }()

			var ddl strings.Builder
			for i := 0; i < tables; i++ {
				fmt.Fprintf(&ddl,
					"CREATE TABLE table_%04d (id INTEGER PRIMARY KEY, name TEXT, amount NUMERIC, created_at TEXT);\n",
					i)
			}
			if _, err := manager.Execute(ctx, "bench", cfg, ddl.String()); err != nil {
				b.Fatalf("create tables: %v", err)
			}

			tree, err := manager.Introspect(ctx, "bench", cfg)
			if err != nil {
				b.Fatalf("introspect: %v", err)
			}
			encoded, err := json.Marshal(tree)
			if err != nil {
				b.Fatalf("marshal tree: %v", err)
			}
			b.Logf("tables=%d json_bytes=%d", tables, len(encoded))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := manager.Introspect(ctx, "bench", cfg); err != nil {
					b.Fatalf("introspect: %v", err)
				}
			}
		})
	}
}

// BenchmarkConcurrentQueries measures throughput for a representative
// concurrent workload against SQLite (serialized), mirroring the M5-T05 matrix.
func BenchmarkConcurrentQueries(b *testing.B) {
	cfg := Config{Driver: model.DriverSQLite, Database: filepath.Join(b.TempDir(), "conc.db")}
	manager := NewManager(DefaultOptions(), SQLite{})
	ctx := context.Background()
	if _, err := manager.Open(ctx, "bench", cfg); err != nil {
		b.Fatalf("open: %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	sql := `WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n < 100)
		SELECT n FROM seq`
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := manager.Execute(ctx, "bench", cfg, sql); err != nil {
				b.Fatalf("execute: %v", err)
			}
		}
	})
}
