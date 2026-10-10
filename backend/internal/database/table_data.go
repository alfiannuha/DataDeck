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
	// Sort is optional. When set, the column is validated against metadata and
	// the backend appends ORDER BY (never raw SQL from the caller). PRF-02/T05.
	Sort *model.TableSort
	// Filters are optional structured predicates ANDed together (PRF-02/T06).
	Filters []model.TableFilter
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

	columns := sortedColumns(table.Columns)

	page := model.TableDataPage{
		Schema:          req.Schema,
		Table:           req.Table,
		ObjectType:      table.Type,
		Columns:         columnInfo(columns, table.PrimaryKey),
		Rows:            [][]any{},
		Pagination:      model.TablePagination{Page: 1, PageSize: req.Limit},
		RowCapabilities: rowCapabilities(table),
		RowIdentity:     rowIdentity(table),
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

	// Structured filters → ANDed, fully parameterized WHERE (before ORDER BY);
	// server-side ORDER BY with a validated column + PK tie-break (PRF-02/T05/T06).
	whereClause, filterArgs, orderBy, err := tableWhereAndOrder(quote, string(cfg.Driver), table, columns, req.Sort, req.Filters)
	if err != nil {
		return model.TableDataPage{}, err
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
	// are quoted metadata names. Filter values are the only bound parameters.
	sqlText := fmt.Sprintf(
		"SELECT %s FROM %s%s%s LIMIT %d OFFSET %d",
		strings.Join(projection, ", "),
		qualifiedTable(quote, schemaForSQL, req.Table),
		whereClause,
		orderBy,
		limit+1,
		offset,
	)

	var result model.QueryResult
	if len(filterArgs) > 0 {
		executor, ok := connector.(ArgumentExecutor)
		if !ok {
			return model.TableDataPage{}, fmt.Errorf("%w: parameterized browsing", ErrNotImplemented)
		}
		result, err = executor.ExecuteArgs(ctx, db, sqlText, filterArgs)
	} else {
		result, err = connector.Execute(ctx, db, sqlText)
	}
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
		insertable := !column.Generated && !column.Identity
		out[i] = model.TableColumnInfo{
			Name:            column.Name,
			DatabaseType:    column.DataType,
			Nullable:        column.Nullable,
			OrdinalPosition: column.OrdinalPosition,
			PrimaryKey:      pk[column.Name],
			Insertable:      insertable,
			Updatable:       insertable && !pk[column.Name],
			HasDefault:      column.Default != nil,
		}
	}
	return out
}

// isBaseTable reports whether the relation is a mutable base table (views,
// materialized views and foreign tables are read-only in v0.1.0).
func isBaseTable(tableType string) bool {
	upper := strings.ToUpper(tableType)
	return strings.Contains(upper, "BASE TABLE") || strings.Contains(upper, "PARTITIONED TABLE")
}

// rowCapabilities derives the effective table-level mutation capability. Update
// and Delete require a declared primary key (v0.1.0); no unique-index fallback.
func rowCapabilities(table *model.Table) model.RowCapabilities {
	base := isBaseTable(table.Type)
	hasPK := table.PrimaryKey != nil && len(table.PrimaryKey.Columns) > 0
	return model.RowCapabilities{
		Insert:    base,
		Update:    base && hasPK,
		Delete:    base && hasPK,
		Duplicate: base,
	}
}

func rowIdentity(table *model.Table) *model.RowIdentityInfo {
	if table.PrimaryKey == nil || len(table.PrimaryKey.Columns) == 0 {
		return nil
	}
	return &model.RowIdentityInfo{
		Kind:    "primary_key",
		Columns: append([]string(nil), table.PrimaryKey.Columns...),
	}
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

// canonicalColumn returns the metadata column with the exact name, or nil.
func canonicalColumn(columns []model.Column, name string) *model.Column {
	for i := range columns {
		if columns[i].Name == name {
			return &columns[i]
		}
	}
	return nil
}

// directionKeyword maps the validated enum to a SQL keyword. Anything other
// than the approved "desc" resolves to ASC (the handler rejects invalid values
// before this point, so this is defense in depth only).
func directionKeyword(direction string) string {
	if direction == "desc" {
		return "DESC"
	}
	return "ASC"
}

// tableWhereAndOrder builds the shared " WHERE …"/" ORDER BY …" fragments used
// by both paged browsing (BrowseTable) and streaming table export (ExportTable).
// Identifiers are quoted metadata names — never user text — and filter values are
// the only bound parameters (filterClause). A nil sort yields no ORDER BY.
func tableWhereAndOrder(quote, driver string, table *model.Table, columns []model.Column, sort *model.TableSort, filters []model.TableFilter) (whereClause string, args []any, orderBy string, err error) {
	whereSQL, args, err := filterClause(quote, driver, filters, columns)
	if err != nil {
		return "", nil, "", err
	}
	if whereSQL != "" {
		whereClause = " WHERE " + whereSQL
	}
	if sort != nil {
		column := canonicalColumn(columns, sort.Column)
		if column == nil {
			return "", nil, "", fmt.Errorf("%w: %s", ErrSortColumnNotFound, sort.Column)
		}
		order := []string{quoteIdentifier(quote, column.Name) + " " + directionKeyword(sort.Direction)}
		// Deterministic tie-break by primary key (ascending) so exports and
		// pages do not shift when sort values have duplicates.
		if table.PrimaryKey != nil {
			for _, pk := range table.PrimaryKey.Columns {
				if pk == "" || pk == column.Name {
					continue
				}
				order = append(order, quoteIdentifier(quote, pk)+" ASC")
			}
		}
		orderBy = " ORDER BY " + strings.Join(order, ", ")
	}
	return whereClause, args, orderBy, nil
}

// sortedColumns returns the columns ordered by their ordinal position. A fresh
// slice is returned so callers never mutate shared introspection metadata.
func sortedColumns(columns []model.Column) []model.Column {
	out := append([]model.Column(nil), columns...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].OrdinalPosition < out[j].OrdinalPosition })
	return out
}

// insertableColumns returns only the columns that should be INSERTed on export
// (generated and identity columns are excluded). Empty result → ErrNoExportableColumns.
func insertableColumns(columns []model.Column) ([]model.Column, error) {
	out := make([]model.Column, 0, len(columns))
	for _, column := range columns {
		if column.Generated || column.Identity {
			continue
		}
		out = append(out, column)
	}
	if len(out) == 0 {
		return nil, ErrNoExportableColumns
	}
	return out, nil
}
