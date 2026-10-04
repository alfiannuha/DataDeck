package database

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// TableBrowseRequest is the validated, bounded browse request for one table.
// The handler resolves `database` into Config.Database before calling here; the
// schema/table pair is validated against introspection metadata.
type TableBrowseRequest struct {
	Schema string
	Table  string
	Limit  int
	Offset int
}

// BrowseTable reads one bounded page of rows from a specific table without the
// caller supplying SQL. It:
//
//  1. activates/reuses the PRF-01 pool for (id, cfg.Database),
//  2. resolves the requested schema/table against introspection metadata,
//  3. builds an explicit, dialect-quoted SELECT over the metadata columns,
//  4. fetches limit+1 rows (never the whole table) to derive has_more,
//  5. reuses the driver's existing result encoding (BIGINT/NULL/bytea/JSON).
//
// It never runs SELECT *, never runs COUNT(*), and never accepts identifiers
// that are not present in metadata.
func (m *Manager) BrowseTable(ctx context.Context, id string, cfg Config, req TableBrowseRequest) (model.TableDataPage, error) {
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return model.TableDataPage{}, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}

	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return model.TableDataPage{}, err
	}

	databases, err := connector.Introspect(ctx, db)
	if err != nil {
		return model.TableDataPage{}, err
	}

	table, ok := findTable(databases, cfg.Driver, req.Schema, req.Table)
	if !ok {
		return model.TableDataPage{}, fmt.Errorf("%w: %s", ErrTableNotFound, qualifiedLabel(req.Schema, req.Table))
	}

	columns := append([]model.Column(nil), table.Columns...)
	sort.SliceStable(columns, func(i, j int) bool {
		return columns[i].OrdinalPosition < columns[j].OrdinalPosition
	})

	page := model.TableDataPage{
		Schema:     req.Schema,
		Table:      req.Table,
		ObjectType: table.Type,
		Columns:    columnInfo(columns, table.PrimaryKey),
		Rows:       [][]any{},
		Pagination: model.TablePagination{Page: 1, PageSize: req.Limit},
	}
	if len(columns) == 0 {
		return page, nil
	}

	quote := connector.Capabilities().IdentifierQuote
	if quote == "" {
		quote = `"`
	}
	// Only PostgreSQL has a schema level; MySQL/SQLite tables are addressed
	// unqualified because the pool is already bound to the target database/file.
	schemaForSQL := req.Schema
	if cfg.Driver != model.DriverPostgres {
		schemaForSQL = ""
	}
	projection := make([]string, len(columns))
	for i, column := range columns {
		projection[i] = quoteIdentifier(quote, column.Name)
	}

	limit := req.Limit
	if limit < 1 {
		limit = 1
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// LIMIT/OFFSET are server-validated integers (never user text); identifiers
	// are quoted metadata names. No value placeholder is needed or accepted.
	sqlText := fmt.Sprintf(
		"SELECT %s FROM %s LIMIT %d OFFSET %d",
		strings.Join(projection, ", "),
		qualifiedTable(quote, schemaForSQL, req.Table),
		limit+1,
		offset,
	)

	result, err := connector.Execute(ctx, db, sqlText)
	if err != nil {
		return model.TableDataPage{}, err
	}

	hasMore := len(result.Rows) > limit
	rows := result.Rows
	if hasMore {
		rows = rows[:limit]
	}
	page.Rows = rows
	page.Truncated = result.Truncated
	page.Pagination.HasMore = hasMore
	return page, nil
}

// findTable resolves a relation inside the introspection tree. PostgreSQL
// requires the schema to match; engines without a schema level match on name.
func findTable(databases []model.Database, driver model.Driver, schema, table string) (*model.Table, bool) {
	for i := range databases {
		db := &databases[i]
		for s := range db.Schemas {
			if driver == model.DriverPostgres && schema != "" && db.Schemas[s].Name != schema {
				continue
			}
			for t := range db.Schemas[s].Tables {
				candidate := &db.Schemas[s].Tables[t]
				if candidate.Name == table {
					return candidate, true
				}
			}
		}
		for t := range db.Tables {
			candidate := &db.Tables[t]
			if candidate.Name == table {
				return candidate, true
			}
		}
	}
	return nil, false
}

func columnInfo(columns []model.Column, primaryKey *model.PrimaryKey) []model.TableColumnInfo {
	pk := map[string]bool{}
	if primaryKey != nil {
		for _, name := range primaryKey.Columns {
			pk[name] = true
		}
	}
	out := make([]model.TableColumnInfo, len(columns))
	for i, column := range columns {
		out[i] = model.TableColumnInfo{
			Name:            column.Name,
			DatabaseType:    column.DataType,
			Nullable:        column.Nullable,
			OrdinalPosition: column.OrdinalPosition,
			PrimaryKey:      pk[column.Name],
		}
	}
	return out
}

// quoteIdentifier quotes one metadata-validated identifier for the engine.
func quoteIdentifier(quote, identifier string) string {
	return quote + strings.ReplaceAll(identifier, quote, quote+quote) + quote
}

// qualifiedTable quotes the table, prefixing the schema where one applies.
func qualifiedTable(quote, schema, table string) string {
	if schema == "" {
		return quoteIdentifier(quote, table)
	}
	return quoteIdentifier(quote, schema) + "." + quoteIdentifier(quote, table)
}

func qualifiedLabel(schema, table string) string {
	if schema == "" {
		return table
	}
	return schema + "." + table
}
