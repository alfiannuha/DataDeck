package database

import (
	"context"
	"fmt"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// RestoreScript executes an already-validated sequence of DDL/DML statements in
// one transaction on the target database (PRF-02/T17). Statement validation
// (allowlist, manifest binding, empty-target preflight) is the caller's
// responsibility; this method only guarantees a single all-or-nothing
// transaction where the engine supports DDL in a transaction (PostgreSQL,
// SQLite). It is never used for MySQL, whose DDL implicitly commits.
func (m *Manager) RestoreScript(ctx context.Context, id string, cfg Config, statements []string) error {
	if _, ok := m.connectors[cfg.Driver]; !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	if cfg.Driver == model.DriverMySQL {
		return fmt.Errorf("%w: atomic restore is not supported for mysql", ErrNotImplemented)
	}

	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return normalizeRestoreError(cfg.Driver, err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("statement %d: %w", i+1, normalizeRestoreError(cfg.Driver, err))
		}
	}
	if err := tx.Commit(); err != nil {
		return normalizeRestoreError(cfg.Driver, err)
	}
	return nil
}

// normalizeRestoreError maps a raw driver error to the neutral SQLError so the
// handler can render a sanitized message with the driver's native code/position.
func normalizeRestoreError(driver model.Driver, err error) error {
	switch driver {
	case model.DriverPostgres:
		return postgresSQLError(err)
	case model.DriverMySQL:
		return mysqlSQLError(err)
	case model.DriverSQLite:
		return sqliteSQLError(err)
	}
	return err
}
