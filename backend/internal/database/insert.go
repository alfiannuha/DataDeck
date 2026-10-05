package database

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// InsertRowRequest is a structured single-row INSERT (PRF-02/T08). Each value is
// a column → {mode,value}; mode distinguishes VALUE/NULL/DEFAULT. There is no
// SQL/expression field.
type InsertRowRequest struct {
	Schema string
	Table  string
	Values map[string]model.InsertValue
}

// InsertRow inserts one row through a metadata-validated, parameterized INSERT.
// Generated/identity columns are rejected for explicit VALUE; NULL mode is only
// allowed for nullable columns; DEFAULT/omitted columns are left out so the
// database applies its default. Exactly one affected row is required.
func (m *Manager) InsertRow(ctx context.Context, id string, cfg Config, req InsertRowRequest) (model.RowMutationResult, error) {
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return model.RowMutationResult{}, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	executor, ok := connector.(ArgumentExecutor)
	if !ok {
		return model.RowMutationResult{}, fmt.Errorf("%w: parameterized mutation", ErrNotImplemented)
	}

	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return model.RowMutationResult{}, err
	}
	databases, err := connector.Introspect(ctx, db)
	if err != nil {
		return model.RowMutationResult{}, err
	}
	table, ok := findTable(databases, cfg.Driver, req.Schema, req.Table)
	if !ok {
		return model.RowMutationResult{}, fmt.Errorf("%w: %s", ErrTableNotFound, qualifiedLabel(req.Schema, req.Table))
	}
	if !isBaseTable(table.Type) {
		return model.RowMutationResult{}, ErrRowNotMutable
	}

	quote := connector.Capabilities().IdentifierQuote
	if quote == "" {
		quote = `"`
	}
	schemaForSQL := req.Schema
	if cfg.Driver != model.DriverPostgres {
		schemaForSQL = ""
	}
	target := qualifiedTable(quote, schemaForSQL, req.Table)

	// Canonical column order (metadata ordinal), never map iteration order.
	ordered := append([]model.Column(nil), table.Columns...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].OrdinalPosition < ordered[j].OrdinalPosition })

	// Reject any requested column that is not in canonical metadata.
	for name := range req.Values {
		if canonicalColumn(ordered, name) == nil {
			return model.RowMutationResult{}, fmt.Errorf("%w: unknown column %q", ErrInvalidColumnValue, name)
		}
	}

	columns := make([]string, 0, len(ordered))
	args := make([]any, 0, len(ordered))
	for _, column := range ordered {
		value, present, err := insertValue(column, req.Values)
		if err != nil {
			return model.RowMutationResult{}, err
		}
		if !present {
			continue // DEFAULT or omitted: let the database decide.
		}
		columns = append(columns, quoteIdentifier(quote, column.Name))
		args = append(args, value)
	}

	var sqlText string
	switch {
	case len(columns) > 0:
		placeholders := make([]string, len(columns))
		for i := range columns {
			placeholders[i] = placeholder(string(cfg.Driver), i+1)
		}
		sqlText = "INSERT INTO " + target + " (" + strings.Join(columns, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	default:
		// All columns use their database default.
		if cfg.Driver == model.DriverMySQL {
			sqlText = "INSERT INTO " + target + " () VALUES ()"
		} else {
			sqlText = "INSERT INTO " + target + " DEFAULT VALUES"
		}
	}

	returning := cfg.Driver == model.DriverPostgres || cfg.Driver == model.DriverSQLite
	if returning {
		projection := make([]string, len(ordered))
		for i, column := range ordered {
			projection[i] = quoteIdentifier(quote, column.Name)
		}
		sqlText += " RETURNING " + strings.Join(projection, ", ")
	}

	result, err := executor.ExecuteArgs(ctx, db, sqlText, args)
	if err != nil {
		return model.RowMutationResult{}, classifyInsertError(err)
	}
	if result.RowsAffected != 1 {
		return model.RowMutationResult{}, ErrInsertFailed
	}
	if returning && len(result.Rows) == 1 {
		return model.RowMutationResult{AffectedRows: 1, Row: result.Rows[0]}, nil
	}
	return model.RowMutationResult{AffectedRows: 1}, nil
}

// insertValue resolves one column's input. present=false means "omit from the
// INSERT column list" (DEFAULT or absent). present=true with value=nil means an
// explicit NULL.
func insertValue(column model.Column, values map[string]model.InsertValue) (any, bool, error) {
	input, ok := values[column.Name]
	if !ok {
		return nil, false, nil // omitted → database default
	}
	switch strings.ToLower(strings.TrimSpace(input.Mode)) {
	case "", "default":
		return nil, false, nil
	case "null":
		if !column.Nullable {
			return nil, false, fmt.Errorf("%w: column %q is NOT NULL", ErrInvalidColumnValue, column.Name)
		}
		return nil, true, nil
	case "value":
		if column.Generated || column.Identity {
			return nil, false, fmt.Errorf("%w: %q cannot be set explicitly", ErrColumnReadOnly, column.Name)
		}
		if input.Value == nil {
			return nil, false, fmt.Errorf("%w: %q requires a value (use null mode for NULL)", ErrInvalidColumnValue, column.Name)
		}
		converted, err := convertValue(categorize(column.DataType), input.Value)
		if err != nil {
			return nil, false, fmt.Errorf("%w: %v", ErrInvalidColumnValue, err)
		}
		return converted, true, nil
	default:
		return nil, false, fmt.Errorf("%w: unknown mode %q", ErrInvalidColumnValue, input.Mode)
	}
}

// classifyInsertError maps engine constraint failures to sanitized sentinels.
func classifyInsertError(err error) error {
	var sqlErr *SQLError
	if !errors.As(err, &sqlErr) {
		if strings.Contains(strings.ToLower(err.Error()), "constraint") {
			return ErrConstraintViolation
		}
		return err
	}
	switch sqlErr.Code {
	case // PostgreSQL
		"23502", "23503", "23505", "23514",
		// MySQL
		"1048", "1062", "1451", "1452", "3819",
		// SQLite constraint
		"19", "1555", "2067", "787", "1299":
		return fmt.Errorf("%w: %s", ErrConstraintViolation, sqlErr.Message)
	case "22P02", "22003", "22007", "1366":
		return fmt.Errorf("%w: %s", ErrInvalidColumnValue, sqlErr.Message)
	default:
		if strings.Contains(strings.ToLower(sqlErr.Message), "constraint") {
			return fmt.Errorf("%w: %s", ErrConstraintViolation, sqlErr.Message)
		}
		return err
	}
}
