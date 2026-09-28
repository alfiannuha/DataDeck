package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// ListDatabases implements DatabaseLister for PostgreSQL (PRF-01).
//
// It reads cluster catalog metadata only — it never opens a pool per discovered
// database. Filtering rules (ADR-009 §4):
//
//   - `datallowconn` excludes databases refusing connections,
//   - `NOT datistemplate` excludes template0/template1,
//   - `has_database_privilege(oid,'CONNECT')` hides databases the credentials
//     cannot connect to.
//
// Errors are wrapped without echoing the DSN or credentials.
func (Postgres) ListDatabases(ctx context.Context, db *sql.DB) ([]model.DatabaseInfo, error) {
	const query = `
		SELECT datname, (datname = 'postgres') AS bootstrap_candidate
		FROM pg_database
		WHERE datallowconn
		  AND NOT datistemplate
		  AND has_database_privilege(oid, 'CONNECT')
		ORDER BY datname`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list postgres databases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	databases := make([]model.DatabaseInfo, 0)
	for rows.Next() {
		var (
			name      string
			candidate bool
		)
		if err := rows.Scan(&name, &candidate); err != nil {
			return nil, fmt.Errorf("scan postgres database: %w", err)
		}
		databases = append(databases, model.DatabaseInfo{
			Name:               name,
			BootstrapCandidate: candidate,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres databases: %w", err)
	}
	return databases, nil
}
