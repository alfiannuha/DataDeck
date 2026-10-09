package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// TableStructure reads the read-only structure of one relation (PRF-02/T13). It
// activates/reuses the pool for (id, cfg.Database), requires the driver to
// implement StructureReader, and binds the canonical result to the connection's
// database and requested schema (PostgreSQL only) so the payload is never
// ambiguous. A lost connection evicts the pool like the other data paths.
func (m *Manager) TableStructure(ctx context.Context, id string, cfg Config, schema, table string) (model.TableStructure, error) {
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return model.TableStructure{}, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	reader, ok := connector.(StructureReader)
	if !ok {
		return model.TableStructure{}, fmt.Errorf("%w: table structure introspection for %s", ErrNotImplemented, cfg.Driver)
	}

	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return model.TableStructure{}, err
	}

	structure, err := reader.TableStructure(ctx, db, schema, table)
	if err != nil {
		if errors.Is(err, ErrConnection) {
			m.evict(id, cfg)
		}
		return model.TableStructure{}, err
	}

	// PostgreSQL alone carries a schema level; MySQL/SQLite are bound by the
	// pool to cfg.Database and the schema is always empty.
	boundSchema := structure.Schema
	if cfg.Driver == model.DriverPostgres {
		boundSchema = schema
	}
	structure.Database = cfg.Database
	structure.Schema = boundSchema
	structure.Table = table
	return structure, nil
}
