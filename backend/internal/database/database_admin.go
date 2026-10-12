package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// Destructive database management (PRF-02/T18).
//
// Delete Database is supported only for engines where it can be performed
// safely and verified: PostgreSQL and MySQL. SQLite is deliberately unsupported
// (a database is a local file; safe file deletion needs a separate approved
// design), so it fails closed.

// DropSupported reports whether DROP DATABASE is implemented for a driver.
func DropSupported(driver model.Driver) bool {
	return driver == model.DriverPostgres || driver == model.DriverMySQL
}

// IsProtectedDatabase reports whether a database name is a protected
// system/internal database for the driver. The check is case-insensitive only
// for the fixed engine-internal names; it is an explicit allowlist-of-forbidden
// names, never a pattern that could be bypassed.
func IsProtectedDatabase(driver model.Driver, name string) bool {
	switch driver {
	case model.DriverPostgres:
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "postgres", "template0", "template1":
			return true
		}
	case model.DriverMySQL:
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "mysql", "information_schema", "performance_schema", "sys":
			return true
		}
	default:
		// SQLite (and any future engine) is not a DROP DATABASE target.
		return true
	}
	return false
}

// DropDatabase executes `DROP DATABASE <quoted target>` on the pool for
// execCfg. The caller MUST supply an execution context that is not the target
// database where the engine requires it (PostgreSQL), and must have already
// closed the target pool. The identifier is dialect-quoted; the engine still
// enforces its own privileges. No FORCE/CASCADE is used.
func (m *Manager) DropDatabase(ctx context.Context, id string, execCfg Config, target string) error {
	connector, ok := m.connectors[execCfg.Driver]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedDriver, execCfg.Driver)
	}
	if !DropSupported(execCfg.Driver) {
		return fmt.Errorf("%w: drop database for %s", ErrNotImplemented, execCfg.Driver)
	}
	quote := connector.Capabilities().IdentifierQuote
	if quote == "" {
		quote = `"`
	}
	statement := "DROP DATABASE " + quoteIdentifier(quote, target)

	db, err := m.Open(ctx, id, execCfg)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("%w: %w", ErrDatabaseDropFailed, normalizeRestoreError(execCfg.Driver, err))
	}
	return nil
}

// DatabaseExists independently verifies whether a database is still visible to
// the maintenance connection. It never trusts the DROP statement's success.
func (m *Manager) DatabaseExists(ctx context.Context, id string, execCfg Config, target string) (bool, error) {
	switch execCfg.Driver {
	case model.DriverPostgres:
		databases, err := m.ListDatabases(ctx, id, execCfg)
		if err != nil {
			return false, err
		}
		for _, database := range databases {
			if database.Name == target {
				return true, nil
			}
		}
		return false, nil
	case model.DriverMySQL:
		result, err := m.Execute(ctx, id, execCfg, "SHOW DATABASES")
		if err != nil {
			return false, err
		}
		for _, row := range result.Rows {
			if len(row) == 1 {
				if name, ok := row[0].(string); ok && name == target {
					return true, nil
				}
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("%w: database existence check for %s", ErrNotImplemented, execCfg.Driver)
	}
}
