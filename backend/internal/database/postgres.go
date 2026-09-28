package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// Postgres is the PostgreSQL connector, backed by the pgx stdlib driver.
type Postgres struct{}

// Name returns the driver identifier.
func (Postgres) Name() model.Driver { return model.DriverPostgres }

// Capabilities reports the PostgreSQL feature set.
func (Postgres) Capabilities() model.Capabilities {
	return model.Capabilities{
		Schemas:           true,
		ForeignKeys:       true,
		Indexes:           true,
		SSL:               true,
		SSH:               true,
		MultipleDatabases: true,
		Dialect:           "postgres",
		IdentifierQuote:   `"`,
	}
}

// Open creates a lazy connection pool. The pool is not considered usable until
// the manager pings it.
func (Postgres) Open(_ context.Context, cfg Config, opts Options) (*sql.DB, error) {
	db, err := sql.Open("pgx", postgresDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	applyPoolOptions(db, opts)
	return db, nil
}

// Ping verifies the pool can reach the server within ctx's deadline.
//
// A missing target database (SQLSTATE 3D000) or a CONNECT privilege denial
// (SQLSTATE 42501) is surfaced as a distinct sentinel so the API can return an
// actionable, sanitized error instead of a generic connection failure.
func (Postgres) Ping(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return postgresConnectError(err)
	}
	return nil
}

func postgresConnectError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "3D000": // invalid_catalog_name: database does not exist
			return fmt.Errorf("%w: %s", ErrDatabaseNotFound, pgErr.Message)
		case "42501": // insufficient_privilege: no CONNECT on the database
			return fmt.Errorf("%w: %s", ErrDatabaseConnectDenied, pgErr.Message)
		}
	}
	return err
}

// postgresDSN builds a postgres:// URL. The result contains the password and
// must never be logged.
func postgresDSN(cfg Config) string {
	dsn := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:   "/" + cfg.Database,
	}
	if cfg.Username != "" || cfg.Password != "" {
		dsn.User = url.UserPassword(cfg.Username, cfg.Password)
	}

	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	query := url.Values{}
	query.Set("sslmode", sslMode)
	dsn.RawQuery = query.Encode()

	return dsn.String()
}
