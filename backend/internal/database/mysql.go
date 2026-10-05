package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	gosqlmysql "github.com/go-sql-driver/mysql"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// MySQL is the MySQL connector, backed by go-sql-driver/mysql.
//
// Connection lifecycle is fully implemented; schema introspection and query
// execution are intentionally deferred to a later milestone and return
// ErrNotImplemented.
type MySQL struct{}

// Name returns the driver identifier.
func (MySQL) Name() model.Driver { return model.DriverMySQL }

// Capabilities reports the MySQL feature set. MySQL has no separate schema
// level (a database is addressed directly), so Schemas is false.
func (MySQL) Capabilities() model.Capabilities {
	return model.Capabilities{
		Schemas:         false,
		ForeignKeys:     true,
		Indexes:         true,
		SSL:             true,
		SSH:             true,
		Dialect:         "mysql",
		IdentifierQuote: "`",
	}
}

// Open creates a lazy connection pool. The pool is not usable until pinged.
func (MySQL) Open(_ context.Context, cfg Config, opts Options) (*sql.DB, error) {
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	applyPoolOptions(db, opts)
	return db, nil
}

// Ping verifies the pool can reach the server within ctx's deadline.
func (MySQL) Ping(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
}

// Introspect reads the connected database's tables into the neutral model
// (no schema level; tables hang directly off model.Database).
func (MySQL) Introspect(ctx context.Context, db *sql.DB) ([]model.Database, error) {
	return introspectMySQL(ctx, db)
}

// mysqlDSN builds a DSN through the driver's Config.FormatDSN so credential
// escaping is handled by the driver. The result contains the password and must
// never be logged.
func mysqlDSN(cfg Config) string {
	config := gosqlmysql.NewConfig()
	config.User = cfg.Username
	config.Passwd = cfg.Password
	config.Net = "tcp"
	config.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	config.DBName = cfg.Database
	config.TLSConfig = mysqlTLSConfig(cfg.SSLMode)
	config.Timeout = 5 * time.Second
	config.ParseTime = true
	// Report MATCHED rows on UPDATE (not just changed rows) so the mutation
	// safety layer can distinguish "row not found" from "row unchanged" and
	// enforce exactly-one-row semantics (PRF-02/T07).
	config.ClientFoundRows = true
	config.Params = map[string]string{"charset": "utf8mb4"}
	return config.FormatDSN()
}

// mysqlTLSConfig maps DataDeck SSL modes to go-sql-driver `tls` values.
func mysqlTLSConfig(sslMode string) string {
	switch sslMode {
	case "prefer":
		return "preferred"
	case "require":
		return "skip-verify"
	case "verify-ca", "verify-full":
		return "true"
	case "", "disable", "allow":
		return "false"
	default:
		return "false"
	}
}
