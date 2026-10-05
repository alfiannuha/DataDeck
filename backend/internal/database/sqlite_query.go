package database

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// maxSafeInteger is JavaScript's Number.MAX_SAFE_INTEGER; larger SQLite
// integers are serialized as strings to preserve precision.
const maxSafeInteger = int64(9007199254740991)

// Execute runs one SQL statement on the target SQLite pool. Row vs non-row is
// decided by the database (result set columns vs none); affected rows come from
// SQLite's changes() on the same connection.
func (SQLite) Execute(ctx context.Context, db *sql.DB, sqlText string) (model.QueryResult, error) {
	return (SQLite{}).ExecuteArgs(ctx, db, sqlText, nil)
}

// ExecuteArgs is Execute with bound parameters (PRF-02 table filters).
func (SQLite) ExecuteArgs(ctx context.Context, db *sql.DB, sqlText string, args []any) (model.QueryResult, error) {
	var result model.QueryResult

	conn, err := db.Conn(ctx)
	if err != nil {
		if isConnectionLoss(err) {
			return result, fmt.Errorf("%w: %w", ErrConnection, err)
		}
		return result, fmt.Errorf("acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	start := time.Now()
	err = executeSQLiteQuery(ctx, conn, sqlText, args, &result)
	result.ExecutionTimeMS = time.Since(start).Milliseconds()
	if err != nil {
		return result, sqliteSQLError(err)
	}
	return result, nil
}

func executeSQLiteQuery(ctx context.Context, conn *sql.Conn, sqlText string, args []any, result *model.QueryResult) error {
	queryCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	rows, err := conn.QueryContext(queryCtx, sqlText, args...)
	if err != nil {
		return err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		rows.Close()
		return err
	}
	declaredTypes := make([]string, len(columnTypes))
	result.Columns = make([]model.QueryColumn, 0, len(columnTypes))
	for i, columnType := range columnTypes {
		declared := columnType.DatabaseTypeName()
		declaredTypes[i] = declared
		result.Columns = append(result.Columns, model.QueryColumn{
			Name: columnType.Name(),
			Type: strings.ToUpper(declared),
		})
	}

	buffer := newResultBuffer(MaxResultBytes)
	values := make([]any, len(columnTypes))
	scanTargets := make([]any, len(columnTypes))
	for i := range values {
		scanTargets[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			rows.Close()
			return fmt.Errorf("scan row: %w", err)
		}
		row := make([]any, len(values))
		for i, value := range values {
			row[i] = encodeSQLiteValue(declaredTypes[i], value)
		}
		if !buffer.add(row) {
			result.Truncated = true
			cancel()
			break
		}
	}
	if !result.Truncated {
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
	}
	rows.Close()

	result.Rows = buffer.rows
	if len(columnTypes) > 0 {
		result.RowsAffected = int64(len(buffer.rows))
		return nil
	}

	// Non-row statement (INSERT/UPDATE/DELETE/DDL): changes() reports the rows
	// affected by the most recent statement on this connection.
	if !result.Truncated {
		var affected sql.NullInt64
		if err := conn.QueryRowContext(ctx, "SELECT changes()").Scan(&affected); err == nil &&
			affected.Valid && affected.Int64 >= 0 {
			result.RowsAffected = affected.Int64
		}
	}
	return nil
}

// sqliteSQLError wraps statement errors into the neutral SQLError. Context
// errors pass through so the handler can map timeouts/cancellation.
func sqliteSQLError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	message := err.Error()
	return &SQLError{
		Driver:  model.DriverSQLite,
		Message: message,
		Syntax:  strings.Contains(strings.ToLower(message), "syntax error"),
	}
}

// encodeSQLiteValue converts a dynamically-typed SQLite value into JSON-safe
// data. Integers outside the JS safe range become strings.
func encodeSQLiteValue(declaredType string, value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case int64:
		if v > maxSafeInteger || v < -maxSafeInteger {
			return strconv.FormatInt(v, 10)
		}
		return v
	case float64:
		return v
	case bool:
		return v
	case string:
		return v
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case []byte:
		if strings.Contains(strings.ToUpper(declaredType), "BLOB") || !utf8.Valid(v) {
			return base64.StdEncoding.EncodeToString(v)
		}
		return string(v)
	default:
		return fmt.Sprintf("%v", value)
	}
}
