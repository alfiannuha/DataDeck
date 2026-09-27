//go:build integration

package database_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

var concurrencyLevels = []int{1, 5, 10, 25, 50}

// TestConcurrentQueryMatrix runs concurrent queries at increasing concurrency
// against PostgreSQL, MySQL and SQLite, asserting every operation completes
// without error and reporting peak goroutines and elapsed time.
func TestConcurrentQueryMatrix(t *testing.T) {
	type target struct {
		name    string
		manager *database.Manager
		id      string
		cfg     database.Config
	}
	var targets []target

	if cfg := pgConfig(t); cfg.Driver != "" {
		m := database.NewManager(database.DefaultOptions(), database.Postgres{})
		if _, err := m.Open(context.Background(), "pg", cfg); err != nil {
			t.Fatalf("open pg: %v", err)
		}
		t.Cleanup(func() { _ = m.CloseAll() })
		targets = append(targets, target{"postgres", m, "pg", cfg})
	}
	if cfg := mysqlConfig(t); cfg.Driver != "" {
		m := database.NewManager(database.DefaultOptions(), database.MySQL{})
		if _, err := m.Open(context.Background(), "my", cfg); err != nil {
			t.Fatalf("open mysql: %v", err)
		}
		t.Cleanup(func() { _ = m.CloseAll() })
		targets = append(targets, target{"mysql", m, "my", cfg})
	}
	sqliteCfg := database.Config{Driver: model.DriverSQLite, Database: filepath.Join(t.TempDir(), "busy.db")}
	sqliteMgr := database.NewManager(database.DefaultOptions(), database.SQLite{})
	if _, err := sqliteMgr.Open(context.Background(), "lite", sqliteCfg); err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqliteMgr.CloseAll() })
	targets = append(targets, target{"sqlite", sqliteMgr, "lite", sqliteCfg})

	for _, tg := range targets {
		for _, level := range concurrencyLevels {
			tg, level := tg, level
			t.Run(fmt.Sprintf("%s/%d", tg.name, level), func(t *testing.T) {
				start := time.Now()
				base := runtime.NumGoroutine()
				var wg sync.WaitGroup
				errs := make([]error, level)
				for i := 0; i < level; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
						defer cancel()
						_, errs[i] = tg.manager.Execute(ctx, tg.id, tg.cfg, "SELECT 1")
					}(i)
				}
				wg.Wait()
				peak := runtime.NumGoroutine()
				for i, err := range errs {
					if err != nil {
						t.Fatalf("query %d error = %v", i, err)
					}
				}
				t.Logf("%s concurrency=%d elapsed=%s goroutines base=%d peak=%d",
					tg.name, level, time.Since(start).Round(time.Millisecond), base, peak)
			})
		}
	}
}

// TestPoolLimitsRespectedIntegration proves the configured MaxOpenConns actually
// caps concurrent database connections for a real server.
func TestPoolLimitsRespectedIntegration(t *testing.T) {
	cfg := pgConfig(t)
	opts := database.Options{
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Minute,
		PingTimeout:     5 * time.Second,
	}
	m := database.NewManager(opts, database.Postgres{})
	if _, err := m.Open(context.Background(), "pg", cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = m.CloseAll() }()

	db, _ := m.Get("pg")
	const parallel = 8
	var wg sync.WaitGroup
	errs := make([]error, parallel)
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_, errs[i] = m.Execute(ctx, "pg", cfg, "SELECT pg_sleep(0.3)")
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("query %d error = %v", i, err)
		}
	}
	stats := db.Stats()
	if stats.MaxOpenConnections != 2 {
		t.Fatalf("MaxOpenConnections = %d, want 2", stats.MaxOpenConnections)
	}
	if stats.OpenConnections > 2 {
		t.Fatalf("OpenConnections = %d, exceeded the pool limit", stats.OpenConnections)
	}
	t.Logf("pool stats: open=%d inUse=%d idle=%d maxOpen=%d", stats.OpenConnections, stats.InUse, stats.Idle, stats.MaxOpenConnections)
}

// TestDeleteDuringUse closes a pool while a query is in flight and asserts the
// behavior is deterministic (the in-flight query completes or returns an error;
// no panic/hang) and the connection can be reopened afterwards.
func TestDeleteDuringUse(t *testing.T) {
	cfg := pgConfig(t)
	m := database.NewManager(database.DefaultOptions(), database.Postgres{})
	if _, err := m.Open(context.Background(), "pg", cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = m.CloseAll() }()

	type outcome struct {
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := m.Execute(ctx, "pg", cfg, "SELECT pg_sleep(1)")
		done <- outcome{err: err}
	}()

	time.Sleep(150 * time.Millisecond)
	if err := m.Close("pg"); err != nil {
		t.Fatalf("Close during use error = %v", err)
	}
	if _, ok := m.Get("pg"); ok {
		t.Error("pool still registered after Close during use")
	}

	select {
	case result := <-done:
		// Either outcome is acceptable; it must not panic or hang.
		t.Logf("in-flight query outcome after close: err=%v", result.err)
	case <-time.After(10 * time.Second):
		t.Fatal("in-flight query did not complete after Close")
	}

	recovered := database.NewManager(database.DefaultOptions(), database.Postgres{})
	defer func() { _ = recovered.CloseAll() }()
	if _, err := recovered.Open(context.Background(), "pg2", cfg); err != nil {
		t.Fatalf("reopen after delete-during-use: %v", err)
	}
	if _, err := recovered.Execute(context.Background(), "pg2", cfg, "SELECT 1"); err != nil {
		t.Fatalf("post-delete execute: %v", err)
	}
}

// TestSQLiteConcurrentAccess exercises SQLite's single-connection, busy-timeout
// policy under concurrent writers: operations must serialize rather than fail
// with SQLITE_BUSY.
func TestSQLiteConcurrentAccess(t *testing.T) {
	cfg := database.Config{Driver: model.DriverSQLite, Database: filepath.Join(t.TempDir(), "concurrent.db")}
	m := database.NewManager(database.DefaultOptions(), database.SQLite{})
	if _, err := m.Open(context.Background(), "lite", cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = m.CloseAll() }()

	exec := func(sql string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := m.Execute(ctx, "lite", cfg, sql)
		return err
	}
	if err := exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	const writers = 25
	var wg sync.WaitGroup
	errs := make([]error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = exec(fmt.Sprintf(`INSERT INTO t (v) VALUES ('w%d')`, i))
		}(i)
	}
	wg.Wait()
	lockErrors := 0
	for i, err := range errs {
		if err != nil {
			lockErrors++
			t.Errorf("writer %d error = %v", i, err)
		}
	}

	result, err := func() (model.QueryResult, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return m.Execute(ctx, "lite", cfg, `SELECT COUNT(*) FROM t`)
	}()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	t.Logf("sqlite concurrent writers=%d lockErrors=%d rows=%v", writers, lockErrors, result.Rows)
	if got := result.Rows[0][0]; got != int64(writers) && got != fmt.Sprintf("%d", writers) {
		t.Fatalf("row count = %v (%T), want %d", got, got, writers)
	}
}

// TestConcurrentDistinctConnections runs queries for two different connection
// ids (PostgreSQL and SQLite) at the same time and asserts each returns its own
// driver's result.
func TestConcurrentDistinctConnections(t *testing.T) {
	pgCfg := pgConfig(t)
	liteCfg := database.Config{Driver: model.DriverSQLite, Database: filepath.Join(t.TempDir(), "distinct.db")}

	pgMgr := database.NewManager(database.DefaultOptions(), database.Postgres{})
	liteMgr := database.NewManager(database.DefaultOptions(), database.SQLite{})
	if _, err := pgMgr.Open(context.Background(), "pg", pgCfg); err != nil {
		t.Fatalf("open pg: %v", err)
	}
	if _, err := liteMgr.Open(context.Background(), "lite", liteCfg); err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = pgMgr.CloseAll(); _ = liteMgr.CloseAll() }()

	var wg sync.WaitGroup
	var pgErr, liteErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var result model.QueryResult
		result, pgErr = pgMgr.Execute(ctx, "pg", pgCfg, `SELECT 9007199254740993::bigint`)
		if pgErr == nil && result.Rows[0][0] != "9007199254740993" {
			pgErr = fmt.Errorf("pg result = %v", result.Rows[0][0])
		}
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, liteErr = liteMgr.Execute(ctx, "lite", liteCfg, `SELECT 1`)
	}()
	wg.Wait()
	if pgErr != nil {
		t.Errorf("postgres concurrent query: %v", pgErr)
	}
	if liteErr != nil {
		t.Errorf("sqlite concurrent query: %v", liteErr)
	}
}
