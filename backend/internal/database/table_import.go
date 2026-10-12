package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// Table import (PRF-02/T15). Every import is value-only: source cells are
// converted against canonical metadata and bound as parameters in a single
// all-or-nothing transaction — there is never a raw SQL/expression path and
// never a partial commit. ResolveImportPlan and ExecuteImportRows both resolve
// the target fresh from introspection so a stale plan or a changed relation
// cannot slip between stages (TOCTOU).

// ImportPlanRequest is the resolved/validated shape of an import target plus
// the client-supplied source → target mapping. An empty Mapping requests
// auto-mapping by column name.
type ImportPlanRequest struct {
	Schema string
	Table  string
	// SourceColumns is the parsed source column list (file header or
	// column_1..N). Its length is the width every data row must match.
	SourceColumns []string
	// Mapping binds source column indexes to target column names.
	Mapping []model.ImportMapping
}

// importDefaultBatchSize bounds the number of rows in a single parameterized
// INSERT and therefore the per-statement argument count (placeholder budget).
const importDefaultBatchSize = 100

// ResolveImportPlan resolves the target relation and computes the effective
// source → target mapping against canonical metadata. It validates structure
// only and never reads or writes data; the validate stage uses it, and
// ExecuteImportRows re-resolves internally before touching data.
func (m *Manager) ResolveImportPlan(ctx context.Context, id string, cfg Config, req ImportPlanRequest) (*model.ImportPlan, error) {
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return nil, err
	}
	if err := connector.Ping(ctx, db); err != nil {
		return nil, err
	}
	databases, err := connector.Introspect(ctx, db)
	if err != nil {
		return nil, err
	}
	return resolveImportPlan(databases, cfg.Driver, req)
}

// resolveImportPlan computes the plan from already-fetched metadata.
func resolveImportPlan(databases []model.Database, driver model.Driver, req ImportPlanRequest) (*model.ImportPlan, error) {
	table, ok := findTable(databases, driver, req.Schema, req.Table)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTableNotFound, qualifiedLabel(req.Schema, req.Table))
	}
	if !isBaseTable(table.Type) {
		return nil, ErrRowNotMutable
	}
	columns, err := insertableColumns(sortedColumns(table.Columns))
	if err != nil {
		return nil, ErrRowNotMutable // nothing writable to import into
	}
	// Without an explicit source column list (SH column-implied INSERT scripts),
	// the source is the insertable columns in ordinal order.
	if len(req.SourceColumns) == 0 {
		req.SourceColumns = columnNames(columns)
	}

	mapping, err := resolveMapping(columns, req)
	if err != nil {
		return nil, err
	}
	return &model.ImportPlan{
		Schema:      req.Schema,
		Table:       req.Table,
		ObjectType:  table.Type,
		BaseTable:   true,
		SourceCount: len(req.SourceColumns),
		Columns:     columns,
		ColumnInfo:  columnInfo(columns, table.PrimaryKey),
		Mapping:     mapping,
	}, nil
}

// columnNames returns the insertable columns' names in ordinal order (the
// implied source of an INSERT without an explicit column list).
func columnNames(columns []model.Column) []string {
	out := make([]string, len(columns))
	for i, column := range columns {
		out[i] = column.Name
	}
	return out
}

// resolveMapping validates the client mapping (or auto-maps by name) and
// returns the resolved bindings ordered by source index.
func resolveMapping(columns []model.Column, req ImportPlanRequest) ([]model.ImportMappingResolved, error) {
	if len(req.Mapping) == 0 {
		byName := map[string][]model.Column{}
		for _, column := range columns {
			byName[column.Name] = append(byName[column.Name], column)
		}
		out := make([]model.ImportMappingResolved, 0, len(req.SourceColumns))
		for source, name := range req.SourceColumns {
			match := byName[name]
			if len(match) == 0 {
				continue
			}
			if len(match) > 1 {
				return nil, fmt.Errorf("%w: source column %q is ambiguous", ErrImportMappingInvalid, name)
			}
			out = append(out, model.ImportMappingResolved{Source: source, Target: match[0].Name})
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("%w: no source columns map to target table columns; choose a mapping", ErrImportMappingInvalid)
		}
		return out, nil
	}

	seenSource := map[int]bool{}
	out := make([]model.ImportMappingResolved, 0, len(req.Mapping))
	for _, entry := range req.Mapping {
		if entry.Source < 0 || entry.Source >= len(req.SourceColumns) {
			return nil, fmt.Errorf("%w: source column index %d is out of range", ErrImportMappingInvalid, entry.Source)
		}
		if seenSource[entry.Source] {
			return nil, fmt.Errorf("%w: source column index %d is mapped twice", ErrImportMappingInvalid, entry.Source)
		}
		seenSource[entry.Source] = true
		column := canonicalColumn(columns, entry.Target)
		if column == nil {
			return nil, fmt.Errorf("%w: %q is not an insertable column", ErrImportMappingInvalid, entry.Target)
		}
		out = append(out, model.ImportMappingResolved{Source: entry.Source, Target: column.Name})
	}
	return out, nil
}

// ValidateImportRows converts every source row against the plan without writing
// anything and returns the first maxErrors localized findings. Validation stops
// early once the error budget is exhausted. The stream callback re-reads the
// source file; per-row conversion reuses the same path as execution, so the
// validate stage reports exactly what execute would do. rowCount counts
// converted rows regardless of findings.
func (m *Manager) ValidateImportRows(ctx context.Context, plan *model.ImportPlan, maxErrors int, stream func(emit func(row []model.ImportCell) error) error) ([]model.ImportValidationError, int, error) {
	if maxErrors <= 0 {
		maxErrors = 50
	}
	var findings []model.ImportValidationError
	var cancelled bool
	rowCount := 0
	err := stream(func(row []model.ImportCell) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rowCount++
		if _, convErr := importCellsToArgs(plan, row); convErr != nil {
			var localized importValueError
			if errors.As(convErr, &localized) {
				findings = append(findings, model.ImportValidationError{
					Row:     rowCount,
					Column:  localized.column,
					Message: localized.message,
				})
			} else {
				findings = append(findings, model.ImportValidationError{Row: rowCount, Message: convErr.Error()})
			}
			if len(findings) >= maxErrors {
				cancelled = true
				return errValidationBudgetExhausted
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errValidationBudgetExhausted) || (cancelled && err == nil) {
			return findings, rowCount, nil
		}
		return findings, rowCount, err
	}
	return findings, rowCount, nil
}

var errValidationBudgetExhausted = errors.New("tableimport: validation budget exhausted")

// ExecuteImportRows streams source rows into one all-or-nothing transaction.
// The target and mapping are re-resolved fresh from introspection (a relation
// cannot drift between the validate stage and here), rows are converted and
// inserted in parameterized batches, and any single failure rolls the whole
// import back. The engine is the final gate for NOT NULL/UNIQUE/FK/CHECK.
func (m *Manager) ExecuteImportRows(ctx context.Context, id string, cfg Config, req ImportPlanRequest, stream func(emit func(row []model.ImportCell) error) error) (model.TableImportExecutionResult, error) {
	start := time.Now()
	plan, db, connector, err := m.prepareImport(ctx, id, cfg, req)
	if err != nil {
		return model.TableImportExecutionResult{}, err
	}

	quote := connector.Capabilities().IdentifierQuote
	if quote == "" {
		quote = `"`
	}
	schemaForSQL := plan.Schema
	if cfg.Driver != model.DriverPostgres {
		schemaForSQL = ""
	}
	target := qualifiedTable(quote, schemaForSQL, plan.Table)
	columnNames := make([]string, 0, len(plan.Mapping))
	for _, binding := range plan.Mapping {
		columnNames = append(columnNames, quoteIdentifier(quote, binding.Target))
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		if isConnectionLoss(err) {
			return model.TableImportExecutionResult{}, fmt.Errorf("%w: %w", ErrConnection, err)
		}
		return model.TableImportExecutionResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	batch := make([]any, 0, importDefaultBatchSize*len(columnNames))
	batchRows := 0
	rowCount := 0
	inserted := 0

	err = stream(func(row []model.ImportCell) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rowCount++
		args, convErr := importCellsToArgs(plan, row)
		if convErr != nil {
			return convErr
		}
		batch = append(batch, args...)
		batchRows++
		if batchRows >= importDefaultBatchSize {
			if err := flushImportBatch(ctx, tx, target, columnNames, string(cfg.Driver), batchRows, batch); err != nil {
				return batchError(string(cfg.Driver), rowCount, err)
			}
			inserted += batchRows
			batch = batch[:0]
			batchRows = 0
		}
		return nil
	})
	if err != nil {
		return model.TableImportExecutionResult{}, err
	}
	if batchRows > 0 {
		if err := flushImportBatch(ctx, tx, target, columnNames, string(cfg.Driver), batchRows, batch); err != nil {
			return model.TableImportExecutionResult{}, batchError(string(cfg.Driver), rowCount, err)
		}
		inserted += batchRows
	}

	if err := tx.Commit(); err != nil {
		return model.TableImportExecutionResult{}, err
	}
	committed = true

	return model.TableImportExecutionResult{
		Database:  cfg.Database,
		Schema:    plan.Schema,
		Table:     plan.Table,
		Inserted:  inserted,
		RowCount:  rowCount,
		ElapsedMS: time.Since(start).Milliseconds(),
	}, nil
}

// prepareImport re-resolves the target/mapping against fresh introspection and
// returns the active pool for execution.
func (m *Manager) prepareImport(ctx context.Context, id string, cfg Config, req ImportPlanRequest) (*model.ImportPlan, *sql.DB, Connector, error) {
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return nil, nil, nil, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := connector.Ping(ctx, db); err != nil {
		return nil, nil, nil, err
	}
	databases, err := connector.Introspect(ctx, db)
	if err != nil {
		return nil, nil, nil, err
	}
	plan, err := resolveImportPlan(databases, cfg.Driver, req)
	if err != nil {
		return nil, nil, nil, err
	}
	return plan, db, connector, nil
}

// batchError wraps an engine failure with the last row of the failed batch so
// callers can point at the data responsible. The driver error is first
// normalized to the neutral SQLError (the batch INSERT bypasses the
// argument-executor path where Query already does this conversion).
func batchError(driver string, row int, err error) error {
	switch driver {
	case string(model.DriverPostgres):
		err = postgresSQLError(err)
	case string(model.DriverMySQL):
		err = mysqlSQLError(err)
	case string(model.DriverSQLite):
		err = sqliteSQLError(err)
	}
	return fmt.Errorf("row %d: %w", row, classifyInsertError(err))
}

// flushImportBatch executes one parameterized multi-row INSERT.
func flushImportBatch(ctx context.Context, tx *sql.Tx, target string, columnNames []string, driver string, rows int, args []any) error {
	if len(columnNames) == 0 || rows == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, importBatchSQL(driver, target, columnNames, rows), args...)
	return err
}

// importBatchSQL builds one parameterized multi-row INSERT. PG placeholders are
// numbered across the whole batch; every other engine uses positional ?.
func importBatchSQL(driver, target string, columnNames []string, rows int) string {
	var b strings.Builder
	b.WriteString("INSERT INTO ")
	b.WriteString(target)
	b.WriteString(" (")
	b.WriteString(strings.Join(columnNames, ", "))
	b.WriteString(") VALUES ")
	cols := len(columnNames)
	for r := 0; r < rows; r++ {
		if r > 0 {
			b.WriteString(", ")
		}
		b.WriteByte('(')
		for c := 1; c <= cols; c++ {
			if c > 1 {
				b.WriteString(", ")
			}
			if driver == string(model.DriverPostgres) {
				fmt.Fprintf(&b, "$%d", r*cols+c)
			} else {
				b.WriteByte('?')
			}
		}
		b.WriteByte(')')
	}
	return b.String()
}

// importCellsToArgs converts one source row into the parameter arguments for
// the mapped columns, in plan.Mapping order. Any conversion failure is a
// localized importValueError naming the failing column.
func importCellsToArgs(plan *model.ImportPlan, row []model.ImportCell) ([]any, error) {
	if len(row) != plan.SourceCount {
		return nil, fmt.Errorf("row has %d values, expected %d", len(row), plan.SourceCount)
	}
	args := make([]any, 0, len(plan.Mapping))
	for _, binding := range plan.Mapping {
		cell := row[binding.Source]
		column := canonicalColumn(plan.Columns, binding.Target)
		if column == nil {
			return nil, importValueError{column: binding.Target, message: "target column is not insertable"}
		}
		value, err := importCellValue(*column, cell)
		if err != nil {
			return nil, err
		}
		args = append(args, value)
	}
	return args, nil
}

// importCellValue converts one source cell to a parameter value for the column.
// NULL is only valid for nullable columns; strings are validated against the
// column category (numbers, booleans, JSON) while keeping BIGINT/decimal
// exactness as strings. Text and unsupported-typed columns pass through — the
// engine is the final gate.
func importCellValue(column model.Column, cell model.ImportCell) (any, error) {
	if cell.IsNull {
		if !column.Nullable {
			return nil, importValueError{column: column.Name, message: "column is NOT NULL"}
		}
		return nil, nil
	}
	switch categorize(column.DataType) {
	case categoryBoolean:
		value, ok := parseBoolean(cell.Value)
		if !ok {
			return nil, importValueError{column: column.Name, message: "value is not a boolean (use true/false/1/0)"}
		}
		return value, nil
	case categoryNumeric:
		if !parseNumber(cell.Value) {
			return nil, importValueError{column: column.Name, message: "value is not numeric"}
		}
		return cell.Value, nil
	default:
		if isJSONType(column.DataType) && !json.Valid([]byte(cell.Value)) {
			return nil, importValueError{column: column.Name, message: "value is not valid JSON"}
		}
		return cell.Value, nil
	}
}

func parseBoolean(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "t", "1":
		return true, true
	case "false", "f", "0":
		return false, true
	default:
		return false, false
	}
}

// parseNumber reports whether value is a finite SQL number literal (exactness
// is preserved: the original string is kept, never a float conversion).
func parseNumber(value string) bool {
	if value == "" {
		return false
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return false
	}
	return true
}

func isJSONType(dataType string) bool {
	base := strings.ToLower(strings.TrimSpace(dataType))
	if i := strings.IndexAny(base, "( "); i >= 0 {
		base = base[:i]
	}
	return base == "json" || base == "jsonb"
}

// importValueError is a localized, sanitized conversion failure.
type importValueError struct {
	column  string
	message string
}

func (e importValueError) Error() string { return e.message + " (column " + e.column + ")" }
