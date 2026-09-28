package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"

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
func (Postgres) Ping(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
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
