package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// SQLite is the connector for *user target* SQLite database files. It is
// separate from the embedded application store in internal/storage: this
// connector opens a file the user explicitly configured and never defaults to
// DataDeck's own database.
//
// Connection lifecycle is implemented; schema introspection and query execution
// are deferred and return ErrNotImplemented.
type SQLite struct{}

// Name returns the driver identifier.
func (SQLite) Name() model.Driver { return model.DriverSQLite }

// Capabilities reports the SQLite feature set. A SQLite file is a single
// database with no schema level and no network/credential concepts.
func (SQLite) Capabilities() model.Capabilities {
	return model.Capabilities{
		Schemas:         false,
		ForeignKeys:     true,
		Indexes:         true,
		SSL:             false,
		SSH:             false,
		Dialect:         "sqlite",
		IdentifierQuote: `"`,
	}
}

// Open opens a user-selected SQLite file. The path in cfg.Database is required;
// host/port/username/password are not used.
func (SQLite) Open(_ context.Context, cfg Config, opts Options) (*sql.DB, error) {
	dsn, err := sqliteDSN(cfg.Database)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite target: %w", err)
	}
	// SQLite serializes writers; a single connection avoids SQLITE_BUSY
	// contention. This intentionally differs from the PostgreSQL/MySQL pool
	// defaults (which allow several connections).
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if opts.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(opts.ConnMaxLifetime)
	}
	if opts.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(opts.ConnMaxIdleTime)
	}
	return db, nil
}

// Ping validates the file is reachable. SQLite creates a missing file on first
// open (documented behavior); an unusable path fails here.
func (SQLite) Ping(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
}

// Introspect reads the target database's tables into the neutral model using
// PRAGMA metadata (no schema level; sqlite_* internals filtered).
func (SQLite) Introspect(ctx context.Context, db *sql.DB) ([]model.Database, error) {
	return introspectSQLite(ctx, db)
}

// sqliteDSN normalizes a user path and builds the modernc DSN. Only
// busy_timeout is set; the connection deliberately does NOT force journal_mode
// or foreign_keys so the user's database semantics are not silently changed.
func sqliteDSN(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("sqlite database path must not be empty")
	}
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		trimmed = filepath.Join(home, strings.TrimPrefix(trimmed, "~"))
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve sqlite path: %w", err)
	}
	return "file:" + absolute + "?_pragma=busy_timeout(5000)", nil
}
