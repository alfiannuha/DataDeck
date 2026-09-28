// Package database owns target database connectivity: the per-driver
// connectors and the process-wide connection pool manager.
//
// Schema introspection and query execution are intentionally absent here; they
// arrive in later milestones. The Connector interface is deliberately small so
// additional drivers can be added without changing the manager.
package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// Sentinel errors. Callers match them with errors.Is.
var (
	ErrUnsupportedDriver = errors.New("database: unsupported driver")
	ErrNotFound          = errors.New("database: connection not found")
	ErrConnection        = errors.New("database: connection failed")
	// ErrNotImplemented marks driver operations that exist in the contract but
	// are not implemented for that engine yet.
	ErrNotImplemented = errors.New("database: operation not implemented for this driver")
	// ErrNoBootstrapDatabase marks a discovery attempt where a PostgreSQL
	// profile has no database and none of the bootstrap candidates ("postgres",
	// the login-named database) is reachable (PRF-01/ADR-009).
	ErrNoBootstrapDatabase = errors.New("database: no bootstrap database available")
)

// applyPoolOptions applies the shared pool sizing/lifetime settings.
func applyPoolOptions(db *sql.DB, opts Options) {
	if opts.MaxOpenConns > 0 {
		db.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if opts.MaxIdleConns > 0 {
		db.SetMaxIdleConns(opts.MaxIdleConns)
	}
	if opts.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(opts.ConnMaxLifetime)
	}
	if opts.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(opts.ConnMaxIdleTime)
	}
}

// Config holds resolved target connection parameters. Password is plaintext and
// must never be logged or persisted.
type Config struct {
	Driver   model.Driver
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
}

// Options controls pool sizing and the startup health check.
type Options struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingTimeout     time.Duration
	// MaxPoolsPerConnection bounds the number of cached per-database pools for
	// one connection profile (PRF-01). The least-recently-used pool is evicted
	// when the cap is exceeded. Zero falls back to DefaultMaxPoolsPerConnection.
	MaxPoolsPerConnection int
}

// DefaultMaxPoolsPerConnection bounds cached per-database pools per profile.
const DefaultMaxPoolsPerConnection = 16

// DefaultOptions returns conservative, local-first defaults.
func DefaultOptions() Options {
	return Options{
		MaxOpenConns:          5,
		MaxIdleConns:          2,
		ConnMaxLifetime:       30 * time.Minute,
		ConnMaxIdleTime:       5 * time.Minute,
		PingTimeout:           5 * time.Second, // PRD §2.1 health check
		MaxPoolsPerConnection: DefaultMaxPoolsPerConnection,
	}
}

// DatabaseLister is an optional connector capability: drivers that expose
// multiple selectable databases on one server-level connection implement it
// (PostgreSQL — PRF-01). MySQL/SQLite deliberately do not.
type DatabaseLister interface {
	// ListDatabases returns lightweight metadata for the databases visible to
	// the connection's credentials, excluding templates and databases the user
	// cannot connect to.
	ListDatabases(ctx context.Context, db *sql.DB) ([]model.DatabaseInfo, error)
}

// Connector opens and health-checks pools for a single target database driver,
// reads schema metadata, executes SQL, and advertises its capabilities.
//
// The interface is intentionally small and capability-focused: every method is
// needed by the shared manager/engine today.
type Connector interface {
	Name() model.Driver
	Open(ctx context.Context, cfg Config, opts Options) (*sql.DB, error)
	Ping(ctx context.Context, db *sql.DB) error
	Introspect(ctx context.Context, db *sql.DB) ([]model.Database, error)
	Execute(ctx context.Context, db *sql.DB, sqlText string) (model.QueryResult, error)
	Capabilities() model.Capabilities
}
