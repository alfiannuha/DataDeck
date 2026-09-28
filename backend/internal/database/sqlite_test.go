package database

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/storage"
)

func TestSQLiteNameAndCapabilities(t *testing.T) {
	if got := (SQLite{}).Name(); got != model.DriverSQLite {
		t.Errorf("Name() = %q, want sqlite", got)
	}
	caps := (SQLite{}).Capabilities()
	if caps.Schemas || caps.SSL || caps.SSH {
		t.Errorf("unexpected capabilities: %+v", caps)
	}
	if !caps.ForeignKeys || !caps.Indexes {
		t.Errorf("expected FK/index support: %+v", caps)
	}
	if caps.Dialect != "sqlite" || caps.IdentifierQuote != `"` {
		t.Errorf("unexpected dialect/quote: %+v", caps)
	}
}

func TestSQLiteDSN(t *testing.T) {
	if _, err := sqliteDSN("   "); err == nil {
		t.Error("empty path should be rejected")
	}

	dir := t.TempDir()
	dsn, err := sqliteDSN(filepath.Join(dir, "x.db"))
	if err != nil {
		t.Fatalf("sqliteDSN() error = %v", err)
	}
	if !strings.HasPrefix(dsn, "file:/") || !strings.Contains(dsn, "busy_timeout(5000)") {
		t.Errorf("unexpected DSN: %s", dsn)
	}
	if !strings.HasSuffix(strings.SplitN(dsn, "?", 2)[0], "x.db") {
		t.Errorf("DSN path not normalized: %s", dsn)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		tilde, err := sqliteDSN("~/datadeck-target-test.db")
		if err != nil {
			t.Fatalf("tilde DSN error = %v", err)
		}
		if !strings.HasPrefix(tilde, "file:"+home) {
			t.Errorf("tilde not expanded: %s", tilde)
		}
	}
}

func TestSQLiteOpenCreatesFileAndPing(t *testing.T) {
	manager := NewManager(DefaultOptions(), SQLite{})
	path := filepath.Join(t.TempDir(), "target.db")

	ctx := context.Background()
	if err := manager.Test(ctx, Config{Driver: model.DriverSQLite, Database: path}); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("target database file was not created: %v", err)
	}
}

func TestSQLiteManagerLifecycle(t *testing.T) {
	manager := NewManager(DefaultOptions(), SQLite{})
	path := filepath.Join(t.TempDir(), "target.db")
	cfg := Config{Driver: model.DriverSQLite, Database: path}

	db, err := manager.Open(context.Background(), "s1", cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if db == nil {
		t.Fatal("Open() returned nil")
	}
	if _, ok := manager.Get("s1", cfg.Database); !ok {
		t.Error("Get() did not return the open pool")
	}
	if err := manager.Close("s1", cfg.Database); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if _, ok := manager.Get("s1", cfg.Database); ok {
		t.Error("pool still registered after Close")
	}
}

func TestSQLiteInvalidPathIsConnectionError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	manager := NewManager(Options{PingTimeout: 500 * time.Millisecond}, SQLite{})
	err := manager.Test(context.Background(), Config{
		Driver:   model.DriverSQLite,
		Database: filepath.Join(file, "target.db"),
	})
	if err == nil {
		t.Fatal("expected an error for an unusable path")
	}
}

// TestSQLiteTargetSeparateFromInternalStore ensures the target connector and the
// embedded application store touch different files.
func TestSQLiteTargetSeparateFromInternalStore(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.db")
	internalPath := filepath.Join(dir, "datadeck-internal.db")

	store, err := storage.Open(context.Background(), internalPath)
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	manager := NewManager(DefaultOptions(), SQLite{})
	db, err := manager.Open(context.Background(), "s1", Config{Driver: model.DriverSQLite, Database: targetPath})
	if err != nil {
		t.Fatalf("target Open() error = %v", err)
	}
	defer func() { _ = manager.Close("s1", targetPath) }()

	// The target connection must be independent of the internal store.
	if db == store.DB() {
		t.Fatal("target SQLite reused the internal DataDeck store")
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Errorf("target file missing: %v", err)
	}
	if targetPath == internalPath {
		t.Fatal("target and internal paths must differ")
	}
}
