package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func openStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "datadeck.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func tableExists(t *testing.T, store *Store, name string) bool {
	t.Helper()
	var got string
	err := store.DB().QueryRow(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&got)
	if err != nil {
		return false
	}
	return got == name
}

func indexExists(t *testing.T, store *Store, name string) bool {
	t.Helper()
	var got string
	err := store.DB().QueryRow(
		"SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?", name).Scan(&got)
	if err != nil {
		return false
	}
	return got == name
}

func migrationCount(t *testing.T, store *Store) int {
	t.Helper()
	var count int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	return count
}

func TestOpenCreatesDatabaseAndTables(t *testing.T) {
	store := openStore(t)

	if _, err := os.Stat(store.Path()); err != nil {
		t.Fatalf("database file not created at %s: %v", store.Path(), err)
	}
	for _, table := range requiredTables {
		if !tableExists(t, store, table) {
			t.Errorf("expected table %q to exist", table)
		}
	}
}

func TestOpenCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "datadeck.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file not created at nested path %s: %v", path, err)
	}
}

func TestExpectedIndexesExist(t *testing.T) {
	store := openStore(t)

	for _, index := range []string{"idx_query_history_conn", "idx_saved_queries_conn"} {
		if !indexExists(t, store, index) {
			t.Errorf("expected index %q to exist", index)
		}
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	store := openStore(t)

	var enabled int
	if err := store.DB().QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if enabled != 1 {
		t.Errorf("foreign_keys = %d, want 1", enabled)
	}
}

func TestJournalModeIsWAL(t *testing.T) {
	store := openStore(t)

	var mode string
	if err := store.DB().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
}

func TestForeignKeyConstraintEnforced(t *testing.T) {
	store := openStore(t)

	_, err := store.DB().Exec(
		`INSERT INTO query_history (id, connection_id, sql_text, status, execution_time_ms)
		 VALUES ('h1', 'does-not-exist', 'SELECT 1', 'SUCCESS', 1)`)
	if err == nil {
		t.Fatal("expected foreign key violation, got nil error")
	}
}

func TestCheckConstraintEnforced(t *testing.T) {
	store := openStore(t)

	_, err := store.DB().Exec(
		`INSERT INTO connection_profiles (id, name, driver, database_name)
		 VALUES ('c1', 'Oracle', 'oracle', 'db')`)
	if err == nil {
		t.Fatal("expected CHECK constraint violation for unsupported driver, got nil error")
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "datadeck.db")

	first, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	firstCount := migrationCount(t, first)
	if firstCount == 0 {
		t.Fatal("expected at least one applied migration")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}

	second, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer func() { _ = second.Close() }()

	secondCount := migrationCount(t, second)
	if secondCount != firstCount {
		t.Errorf("migration count after reopen = %d, want %d", secondCount, firstCount)
	}

	var version int
	if err := second.DB().QueryRow("SELECT MIN(version) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatalf("read migration version: %v", err)
	}
	if version != 1 {
		t.Errorf("minimum migration version = %d, want 1", version)
	}
}

func TestClose(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "datadeck.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := store.DB().Ping(); err == nil {
		t.Error("expected error pinging a closed store, got nil")
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open(context.Background(), "   "); err == nil {
		t.Fatal("expected error for empty storage path, got nil")
	}
}

func TestOpenRejectsUnwritablePath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := Open(context.Background(), filepath.Join(file, "datadeck.db")); err == nil {
		t.Fatal("expected error for path whose parent is a file, got nil")
	}
}

// TestStorageDirectoryPermissions verifies the store directory is created with
// restrictive permissions (M5-T01 SEC-MED-2). File permissions are logged, not
// asserted, because only the directory is explicitly enforced today.
func TestStorageDirectoryPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(dir, "datadeck.db")

	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("store dir permissions = %o, want 0700", perm)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("store file permissions = %o, want 0600", perm)
	}
}

// TestOpenRejectsCorruptedDatabase verifies an unreadable/garbage store fails
// explicitly instead of starting with a broken database.
func TestOpenRejectsCorruptedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(path, []byte("this is not a sqlite database"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	store, err := Open(context.Background(), path)
	if err == nil {
		_ = store.Close()
		t.Fatal("Open() error = nil, want failure for a corrupted database")
	}
}
