package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// ExportTable streams every row of one relation through the sink (PRF-02/T14).
// The relation is first resolved against introspection metadata; the SELECT is
// built from quoted identifiers and the driver's canonical row encoder — the
// same values (BIGINT-as-string, NULL, JSON) the browser shows. The stream is
// bounded by the caller's context (handler timeout) and the driver cursor.
//
// SQL export requires a mutable base table and always excludes generated and
// identity columns (they cannot be part of an INSERT); views and materialized
// views map to ErrExportFormatUnsupported. Filter/sort semantics are identical
// to BrowseTable (tableWhereAndOrder).
func (m *Manager) ExportTable(ctx context.Context, id string, cfg Config, req model.TableExportRequest, sink ExportSink) error {
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}

	// Exporting is not streaming without a connector that implements it. This
	// fails fast before any pool is opened.
	streamer, ok := connector.(TableStreamer)
	if !ok {
		return fmt.Errorf("%w: table streaming is not implemented for %s", ErrNotImplemented, cfg.Driver)
	}

	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return err
	}

	databases, err := connector.Introspect(ctx, db)
	if err != nil && (errors.Is(err, ErrConnection) || errors.Is(err, ErrDatabaseNotFound) || errors.Is(err, ErrDatabaseConnectDenied)) {
		m.evict(id, cfg)
	}
	if err != nil {
		return err
	}
	entry, ok := findTable(databases, cfg.Driver, req.Schema, req.Table)
	if !ok {
		return fmt.Errorf("%w: %s", ErrTableNotFound, qualifiedLabel(req.Schema, req.Table))
	}

	columns := sortedColumns(entry.Columns)
	if len(columns) == 0 {
		return ErrNoExportableColumns
	}
	// SQL scripts can only replay INSERTable data; defensive even though the
	// handler sets InsertableOnly for the sql format.
	if req.Format == "sql" || req.InsertableOnly {
		if !isBaseTable(entry.Type) {
			return fmt.Errorf("%w: %s", ErrExportFormatUnsupported, qualifiedLabel(req.Schema, req.Table))
		}
		columns, err = insertableColumns(columns)
		if err != nil {
			return err
		}
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

	whereClause, filterArgs, orderBy, err := tableWhereAndOrder(quote, string(cfg.Driver), entry, columns, req.Sort, req.Filters)
	if err != nil {
		return err
	}
	sqlText := fmt.Sprintf("SELECT %s FROM %s%s%s",
		strings.Join(projection, ", "),
		qualifiedTable(quote, schemaForSQL, entry.Name),
		whereClause,
		orderBy,
	)

	qcols := make([]model.QueryColumn, len(columns))
	for i, column := range columns {
		qcols[i] = model.QueryColumn{Name: column.Name, Type: strings.ToUpper(column.DataType)}
	}
	if err := sink.Begin(qcols); err != nil {
		return fmt.Errorf("export start: %w", err)
	}

	// A sink error cancels the cursor and is returned unchanged by the
	// streamer, so an export-aborting condition (e.g. the XLSX row cap) arrives
	// intact for the handler to map to an error code.
	err = streamer.StreamTable(ctx, db, sqlText, filterArgs, func(row []any) error {
		return sink.Write(row)
	})
	if err != nil && (errors.Is(err, ErrConnection) || errors.Is(err, ErrDatabaseNotFound) || errors.Is(err, ErrDatabaseConnectDenied)) {
		m.evict(id, cfg)
	}
	return err
}