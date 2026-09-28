package database

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	gosqlmysql "github.com/go-sql-driver/mysql"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// Execute runs one SQL statement on the MySQL pool and returns a bounded,
// JSON-safe result. Row vs non-row is decided by the database (a result set has
// columns; a command does not) — never by parsing the SQL text.
func (MySQL) Execute(ctx context.Context, db *sql.DB, sqlText string) (model.QueryResult, error) {
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
	err = executeMySQLQuery(ctx, conn, sqlText, &result)
	result.ExecutionTimeMS = time.Since(start).Milliseconds()
	if err != nil {
		converted := mysqlSQLError(err)
		if _, ok := asSQLError(converted); !ok &&
			(isConnectionLoss(converted) || errors.Is(converted, gosqlmysql.ErrInvalidConn)) {
			// ErrInvalidConn is go-sql-driver's "invalid connection" signal; it
			// is what a broken connection frequently surfaces as (M6-T00).
			return result, fmt.Errorf("%w: %w", ErrConnection, converted)
		}
		return result, converted
	}
	return result, nil
}

func executeMySQLQuery(ctx context.Context, conn *sql.Conn, sqlText string, result *model.QueryResult) error {
	// A cancellable child lets truncation stop the server sending more rows.
	queryCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	rows, err := conn.QueryContext(queryCtx, sqlText)
	if err != nil {
		return err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		rows.Close()
		return err
	}
	typeNames := make([]string, len(columnTypes))
	result.Columns = make([]model.QueryColumn, 0, len(columnTypes))
	for i, columnType := range columnTypes {
		typeName := columnType.DatabaseTypeName()
		typeNames[i] = typeName
		result.Columns = append(result.Columns, model.QueryColumn{
			Name: columnType.Name(),
			Type: strings.ToUpper(typeName),
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
			row[i] = encodeMySQLValue(typeNames[i], value)
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
		// Row-returning statement: affected rows = returned row count.
		result.RowsAffected = int64(len(buffer.rows))
		return nil
	}

	// Non-row statement: read the affected count for the previous statement on
	// the same connection (MySQL ROW_COUNT()). Skipped after truncation because
	// the query context was canceled.
	if !result.Truncated {
		var affected sql.NullInt64
		if err := conn.QueryRowContext(ctx, "SELECT ROW_COUNT()").Scan(&affected); err == nil &&
			affected.Valid && affected.Int64 >= 0 {
			result.RowsAffected = affected.Int64
		}
	}
	return nil
}

// mysqlSQLError converts a driver error into the neutral SQLError when
// recognized; context errors and others pass through unchanged.
func mysqlSQLError(err error) error {
	if err == nil {
		return nil
	}
	var myErr *gosqlmysql.MySQLError
	if errors.As(err, &myErr) {
		return &SQLError{
			Driver:  model.DriverMySQL,
			Code:    strconv.FormatUint(uint64(myErr.Number), 10),
			Message: myErr.Message,
			Syntax:  myErr.Number == 1064, // ER_PARSE_ERROR
		}
	}
	return err
}

// encodeMySQLValue converts a decoded MySQL value into a JSON-safe value using
// the column's database type name.
func encodeMySQLValue(typeName string, value any) any {
	if value == nil {
		return nil
	}
	switch typeName {
	case "BIGINT", "UNSIGNED BIGINT":
		return mysqlIntegerToString(value)
	case "DECIMAL":
		return mysqlToString(value)
	case "FLOAT", "DOUBLE":
		switch v := value.(type) {
		case float32:
			return float64(v)
		case float64:
			return v
		}
		return fmt.Sprintf("%v", value)
	case "JSON":
		return mysqlRawJSON(value)
	case "DATE":
		if t, ok := value.(time.Time); ok {
			return t.Format("2006-01-02")
		}
		return mysqlToString(value)
	case "DATETIME", "TIMESTAMP":
		if t, ok := value.(time.Time); ok {
			return t.Format(time.RFC3339Nano)
		}
		return mysqlToString(value)
	case "TIME":
		return mysqlToString(value)
	case "BINARY", "VARBINARY", "TINYBLOB", "BLOB", "MEDIUMBLOB", "LONGBLOB", "GEOMETRY":
		if b, ok := value.([]byte); ok {
			return base64.StdEncoding.EncodeToString(b)
		}
		return fmt.Sprintf("%v", value)
	case "BIT":
		if b, ok := value.([]byte); ok {
			return base64.StdEncoding.EncodeToString(b)
		}
		return fmt.Sprintf("%v", value)
	case "CHAR", "VARCHAR", "TINYTEXT", "TEXT", "MEDIUMTEXT", "LONGTEXT", "ENUM", "SET":
		return mysqlToString(value)
	}
	return defaultEncodeMySQL(value)
}

func mysqlIntegerToString(value any) any {
	switch v := value.(type) {
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case int:
		return strconv.Itoa(v)
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}

func mysqlToString(value any) any {
	switch v := value.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	case time.Time:
		return v.Format(time.RFC3339Nano)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func mysqlRawJSON(value any) any {
	switch v := value.(type) {
	case json.RawMessage:
		return v
	case []byte:
		return json.RawMessage(v)
	case string:
		return json.RawMessage(v)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return json.RawMessage(encoded)
	}
}

func defaultEncodeMySQL(value any) any {
	switch v := value.(type) {
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
		return base64.StdEncoding.EncodeToString(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case uint64:
		// Never let an unsigned 64-bit value lose precision as a JSON number.
		return strconv.FormatUint(v, 10)
	case string, bool, int, int8, int16, int32, int64, float32, float64:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
