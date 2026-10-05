package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// RowMutationRequest is a structured single-row mutation (PRF-02/T07). The
// target (schema/table) is explicit; identity/expected/changes are validated
// against canonical metadata. There is deliberately no raw WHERE/SQL field.
type RowMutationRequest struct {
	Schema string
	Table  string
	// Identity holds PK column values. Keys must be exactly the PK columns and
	// values must be non-null.
	Identity map[string]any
	// Expected holds original column values for optimistic concurrency. Keys are
	// validated against metadata; nil means "IS NULL".
	Expected map[string]any
	// Changes holds the columns to set (Update only).
	Changes map[string]any
}

// RowMutationResult is the bounded outcome of a mutation.

// resolvedMutation is the validated, SQL-ready form of a mutation.
type resolvedMutation struct {
	table     *model.Table
	identity  []resolvedPredicate
	expected  []resolvedPredicate
	changes   []resolvedChange
	pkColumns []string
}

type resolvedPredicate struct {
	column string
	value  any
	isNull bool
}

type resolvedChange struct {
	column string
	value  any
}

// UpdateRow applies a validated, optimistic single-row UPDATE.
func (m *Manager) UpdateRow(ctx context.Context, id string, cfg Config, req RowMutationRequest) (model.RowMutationResult, error) {
	return m.mutateRow(ctx, id, cfg, req, true)
}

// DeleteRow applies a validated, optimistic single-row DELETE.
func (m *Manager) DeleteRow(ctx context.Context, id string, cfg Config, req RowMutationRequest) (model.RowMutationResult, error) {
	return m.mutateRow(ctx, id, cfg, req, false)
}

func (m *Manager) mutateRow(ctx context.Context, id string, cfg Config, req RowMutationRequest, update bool) (model.RowMutationResult, error) {
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

	resolved, err := planMutation(cfg.Driver, table, req, update)
	if err != nil {
		return model.RowMutationResult{}, err
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

	var sqlText string
	args := make([]any, 0, len(resolved.changes)+len(resolved.identity)+len(resolved.expected))
	if update {
		sets := make([]string, len(resolved.changes))
		for i, change := range resolved.changes {
			args = append(args, change.value)
			sets[i] = quoteIdentifier(quote, change.column) + " = " + placeholder(string(cfg.Driver), len(args))
		}
		sqlText = "UPDATE " + target + " SET " + strings.Join(sets, ", ") + buildPredicates(quote, string(cfg.Driver), resolved, &args)
	} else {
		sqlText = "DELETE FROM " + target + buildPredicates(quote, string(cfg.Driver), resolved, &args)
	}

	result, err := m.executeMutationTx(ctx, db, executor, sqlText, args, resolved, req, cfg.Driver, target, quote, update)
	return result, err
}

// buildPredicates appends the identity (and optional expected) predicates and
// returns the WHERE clause. Values are bound; NULL uses IS NULL.
func buildPredicates(quote, driver string, resolved *resolvedMutation, args *[]any) string {
	parts := make([]string, 0, len(resolved.identity)+len(resolved.expected))
	for _, predicate := range resolved.identity {
		parts = append(parts, predicateFragment(quote, driver, predicate, args))
	}
	for _, predicate := range resolved.expected {
		parts = append(parts, predicateFragment(quote, driver, predicate, args))
	}
	return " WHERE " + strings.Join(parts, " AND ")
}

func predicateFragment(quote, driver string, predicate resolvedPredicate, args *[]any) string {
	column := quoteIdentifier(quote, predicate.column)
	if predicate.isNull {
		return column + " IS NULL"
	}
	*args = append(*args, predicate.value)
	return column + " = " + placeholder(driver, len(*args))
}

// executeMutationTx runs the mutation in a transaction and enforces affected-row
// safety: 1 = success, 0 = not found/conflict, >1 = safety failure + rollback.
func (m *Manager) executeMutationTx(
	ctx context.Context,
	db *sql.DB,
	executor ArgumentExecutor,
	sqlText string,
	args []any,
	resolved *resolvedMutation,
	req RowMutationRequest,
	driver model.Driver,
	target, quote string,
	update bool,
) (model.RowMutationResult, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		if isConnectionLoss(err) {
			return model.RowMutationResult{}, fmt.Errorf("%w: %w", ErrConnection, err)
		}
		return model.RowMutationResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	execResult, err := tx.ExecContext(ctx, sqlText, args...)
	if err != nil {
		return model.RowMutationResult{}, err
	}
	affected, err := execResult.RowsAffected()
	if err != nil {
		return model.RowMutationResult{}, err
	}
	if affected > 1 {
		// Safety abort: rollback happens via the defer.
		return model.RowMutationResult{}, ErrAffectedMultipleRows
	}
	if affected == 0 {
		// Distinguish "row missing" from "row exists but expected values no
		// longer match" with a bounded identity read (same transaction view).
		exists, readErr := rowExistsTx(ctx, tx, quote, string(driver), target, resolved)
		if readErr != nil {
			return model.RowMutationResult{}, readErr
		}
		if exists {
			return model.RowMutationResult{}, ErrRowConflict
		}
		return model.RowMutationResult{}, ErrRowNotFound
	}
	if err := tx.Commit(); err != nil {
		return model.RowMutationResult{}, err
	}
	committed = true

	result := model.RowMutationResult{AffectedRows: int(affected)}
	if update {
		if row, readErr := readRowByIdentity(ctx, executor, db, quote, string(driver), target, resolved); readErr == nil {
			result.Row = row
		}
	}
	return result, nil
}

// rowExistsTx checks whether the identity row is present (used to classify a
// 0-affected UPDATE as ROW_CONFLICT vs ROW_NOT_FOUND).
func rowExistsTx(ctx context.Context, tx *sql.Tx, quote, driver, target string, resolved *resolvedMutation) (bool, error) {
	args := make([]any, 0, len(resolved.identity))
	where := identityPredicateOnly(quote, driver, resolved, &args)
	var one int
	err := tx.QueryRowContext(ctx, "SELECT 1 FROM "+target+where+" LIMIT 1", args...).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func identityPredicateOnly(quote, driver string, resolved *resolvedMutation, args *[]any) string {
	parts := make([]string, 0, len(resolved.identity))
	for _, predicate := range resolved.identity {
		parts = append(parts, predicateFragment(quote, driver, predicate, args))
	}
	return " WHERE " + strings.Join(parts, " AND ")
}

// readRowByIdentity reloads the mutated row (metadata column order) bounded to
// one row, reusing the driver's result encoding.
func readRowByIdentity(ctx context.Context, executor ArgumentExecutor, db *sql.DB, quote, driver, target string, resolved *resolvedMutation) ([]any, error) {
	projection := make([]string, len(resolved.table.Columns))
	for i, column := range resolved.table.Columns {
		projection[i] = quoteIdentifier(quote, column.Name)
	}
	args := make([]any, 0, len(resolved.identity))
	where := identityPredicateOnly(quote, driver, resolved, &args)
	sqlText := "SELECT " + strings.Join(projection, ", ") + " FROM " + target + where + " LIMIT 1"
	page, err := executor.ExecuteArgs(ctx, db, sqlText, args)
	if err != nil {
		return nil, err
	}
	if len(page.Rows) != 1 {
		return nil, nil
	}
	return page.Rows[0], nil
}

// planMutation validates identity/expected/changes against canonical metadata.
func planMutation(driver model.Driver, table *model.Table, req RowMutationRequest, update bool) (*resolvedMutation, error) {
	if !isBaseTable(table.Type) {
		return nil, ErrRowNotMutable
	}
	if table.PrimaryKey == nil || len(table.PrimaryKey.Columns) == 0 {
		return nil, ErrRowIdentityRequired
	}
	pkColumns := table.PrimaryKey.Columns

	// Identity keys must be exactly the PK columns (no missing/extra).
	if len(req.Identity) != len(pkColumns) {
		return nil, fmt.Errorf("%w: identity must contain exactly the primary key columns", ErrRowIdentityInvalid)
	}
	for _, name := range pkColumns {
		if _, ok := req.Identity[name]; !ok {
			return nil, fmt.Errorf("%w: missing primary key column %q", ErrRowIdentityInvalid, name)
		}
	}
	for name := range req.Identity {
		if !containsString(pkColumns, name) {
			return nil, fmt.Errorf("%w: %q is not part of the primary key", ErrRowIdentityInvalid, name)
		}
	}

	resolved := &resolvedMutation{table: table, pkColumns: pkColumns}
	// Canonical PK order (never map iteration order).
	for _, name := range pkColumns {
		column := canonicalColumn(table.Columns, name)
		if column == nil {
			return nil, fmt.Errorf("%w: primary key column %q not found in metadata", ErrRowIdentityInvalid, name)
		}
		if req.Identity[name] == nil {
			return nil, fmt.Errorf("%w: primary key column %q is null", ErrRowIdentityInvalid, name)
		}
		value, err := convertValue(categorize(column.DataType), req.Identity[name])
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRowIdentityInvalid, err)
		}
		resolved.identity = append(resolved.identity, resolvedPredicate{column: name, value: value})
	}

	for _, name := range orderedNames(table, req.Expected) {
		raw := req.Expected[name]
		column := canonicalColumn(table.Columns, name)
		if column == nil {
			return nil, fmt.Errorf("%w: expected column %q not found", ErrRowIdentityInvalid, name)
		}
		if raw == nil {
			resolved.expected = append(resolved.expected, resolvedPredicate{column: name, isNull: true})
			continue
		}
		value, err := convertValue(categorize(column.DataType), raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRowIdentityInvalid, err)
		}
		resolved.expected = append(resolved.expected, resolvedPredicate{column: name, value: value})
	}

	if update {
		if len(req.Changes) == 0 {
			return nil, fmt.Errorf("%w: no changes supplied", ErrRowIdentityInvalid)
		}
		for _, name := range orderedNames(table, req.Changes) {
			raw := req.Changes[name]
			column := canonicalColumn(table.Columns, name)
			if column == nil {
				return nil, fmt.Errorf("%w: change column %q not found", ErrRowIdentityInvalid, name)
			}
			if containsString(pkColumns, name) || column.Generated || column.Identity {
				return nil, fmt.Errorf("%w: %q cannot be modified", ErrColumnReadOnly, name)
			}
			value, err := convertValue(categorize(column.DataType), raw)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrRowIdentityInvalid, err)
			}
			resolved.changes = append(resolved.changes, resolvedChange{column: name, value: value})
		}
	}
	return resolved, nil
}

// orderedNames returns the map keys sorted by canonical metadata ordinal so
// generated SET/expected predicates are deterministic and composite-safe.
func orderedNames(table *model.Table, values map[string]any) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	ordinal := map[string]int{}
	for _, column := range table.Columns {
		ordinal[column.Name] = column.OrdinalPosition
	}
	sortNames(names, ordinal)
	return names
}

func sortNames(names []string, ordinal map[string]int) {
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && ordinal[names[j-1]] > ordinal[names[j]]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
