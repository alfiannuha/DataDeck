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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// MaxResultBytes caps the accumulated JSON result payload (PRD §11.2). A row is
// only appended while the running JSON size stays within the cap; the first row
// that would exceed it is dropped and the result is flagged truncated.
const MaxResultBytes = 50 * 1024 * 1024

// Execute runs one SQL statement on the pool and returns a bounded,
// JSON-safe result.
//
// It uses the pgx connection directly (via database/sql's Raw bridge) because
// the command tag is the only reliable way to learn affected-row counts for
// non-row statements; sniffing the SQL text for a leading SELECT would be
// fooled by CTEs, comments and RETURNING clauses.
func (Postgres) Execute(ctx context.Context, db *sql.DB, sqlText string) (model.QueryResult, error) {
	var result model.QueryResult

	conn, err := db.Conn(ctx)
	if err != nil {
		// A cached pool can outlive its target database (dropped after the pool
		// was created). Classify the acquire failure the same way Ping does so
		// the API still returns DATABASE_NOT_FOUND / DATABASE_CONNECT_DENIED
		// instead of an internal error.
		if classified := postgresConnectError(err); !errors.Is(classified, err) {
			return result, classified
		}
		if isConnectionLoss(err) {
			return result, fmt.Errorf("%w: %w", ErrConnection, err)
		}
		return result, fmt.Errorf("acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	start := time.Now()
	err = conn.Raw(func(driverConn any) error {
		stdlibConn, ok := driverConn.(*stdlib.Conn)
		if !ok {
			return fmt.Errorf("unexpected driver connection type %T", driverConn)
		}
		return executePostgresQuery(ctx, stdlibConn.Conn(), sqlText, &result)
	})
	result.ExecutionTimeMS = time.Since(start).Milliseconds()
	if err != nil {
		converted := postgresSQLError(err)
		if _, ok := asSQLError(converted); !ok && isConnectionLoss(converted) {
			return result, fmt.Errorf("%w: %w", ErrConnection, converted)
		}
		return result, converted
	}
	return result, nil
}

// postgresSQLError converts a pgx error into the neutral SQLError when
// recognized; other errors pass through unchanged.
func postgresSQLError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// Server-side connection termination (e.g. the target database was
		// dropped with FORCE) must not surface as a SQL error: map it to
		// ErrConnection so the manager can evict the pool and reclassify.
		if isPGConnectionClass(pgErr.Code) {
			return fmt.Errorf("%w: %s", ErrConnection, pgErr.Message)
		}
		return &SQLError{
			Driver:   model.DriverPostgres,
			Code:     pgErr.Code,
			Message:  pgErr.Message,
			Position: int(pgErr.Position),
			Syntax:   strings.HasPrefix(pgErr.Code, "42"),
		}
	}
	return err
}

// isPGConnectionClass reports SQLSTATEs that mean the session connection was
// terminated or could not be established, rather than a statement failure.
func isPGConnectionClass(code string) bool {
	switch code {
	case "08000", "08001", "08003", "08006", "08007", // connection exceptions
		"57P01", "57P02", "57P03": // admin/crash shutdown, cannot connect now
		return true
	}
	return false
}

func executePostgresQuery(ctx context.Context, conn *pgx.Conn, sqlText string, result *model.QueryResult) error {
	// A cancellable child lets truncation stop the server sending more rows
	// instead of draining the whole result set.
	queryCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	rows, err := conn.Query(queryCtx, sqlText)
	if err != nil {
		return err
	}

	fields := rows.FieldDescriptions()
	typeNames := make([]string, len(fields))
	result.Columns = make([]model.QueryColumn, 0, len(fields))
	for i, field := range fields {
		typeName := ""
		if typ, ok := conn.TypeMap().TypeForOID(field.DataTypeOID); ok {
			typeName = typ.Name
		}
		typeNames[i] = strings.ToLower(typeName)
		result.Columns = append(result.Columns, model.QueryColumn{
			Name: field.Name,
			Type: strings.ToUpper(typeName),
		})
	}

	buffer := newResultBuffer(MaxResultBytes)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			rows.Close()
			return fmt.Errorf("scan row: %w", err)
		}
		row := make([]any, len(values))
		for i, value := range values {
			row[i] = encodeValue(typeNames[i], value)
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
	result.RowsAffected = rows.CommandTag().RowsAffected()
	if result.RowsAffected == 0 && len(buffer.rows) > 0 {
		result.RowsAffected = int64(len(buffer.rows))
	}
	return nil
}

// resultBuffer accumulates rows while enforcing a JSON-payload byte budget.
type resultBuffer struct {
	rows [][]any
	size int
	max  int
}

func newResultBuffer(max int) *resultBuffer {
	return &resultBuffer{rows: make([][]any, 0), max: max}
}

// add appends row when its encoded size still fits. The size estimate is the
// exact JSON encoding length of the row plus one byte for the separating comma,
// so the accumulated payload never exceeds max by more than a single row.
func (b *resultBuffer) add(row []any) bool {
	encoded, err := json.Marshal(row)
	size := len(encoded)
	if err != nil {
		size = len(row) * 8
	}
	if b.size+size+1 > b.max {
		return false
	}
	b.rows = append(b.rows, row)
	b.size += size + 1
	return true
}

// encodeValue converts a decoded PostgreSQL value into a JSON-safe value.
// Column type names come from the connection's type map.
func encodeValue(typeName string, value any) any {
	if value == nil {
		return nil
	}
	switch typeName {
	case "int8", "bigint":
		return formatInteger(value)
	case "numeric", "decimal", "money":
		return formatNumeric(value)
	case "uuid":
		return formatUUID(value)
	case "json", "jsonb":
		return rawJSON(value)
	case "bytea":
		if b, ok := value.([]byte); ok {
			return base64.StdEncoding.EncodeToString(b)
		}
		return fmt.Sprintf("%v", value)
	case "date":
		if t, ok := value.(time.Time); ok {
			return t.Format("2006-01-02")
		}
	case "timestamp", "timestamptz":
		if t, ok := value.(time.Time); ok {
			return t.Format(time.RFC3339Nano)
		}
	}
	return defaultEncode(value)
}

// formatInteger serializes 64-bit integers as strings to preserve precision
// beyond JavaScript's 53-bit safe integer range (PRD §11.4).
func formatInteger(value any) any {
	switch n := value.(type) {
	case int64:
		return strconv.FormatInt(n, 10)
	case int32:
		return strconv.FormatInt(int64(n), 10)
	case int16:
		return strconv.FormatInt(int64(n), 10)
	case int:
		return strconv.Itoa(n)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func formatNumeric(value any) any {
	if numeric, ok := value.(pgtype.Numeric); ok {
		driverValue, err := numeric.Value()
		if err != nil || driverValue == nil {
			return nil
		}
		if s, ok := driverValue.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", driverValue)
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func formatUUID(value any) any {
	switch v := value.(type) {
	case [16]byte:
		return pgtype.UUID{Bytes: v, Valid: true}.String()
	case []byte:
		if len(v) == 16 {
			var raw [16]byte
			copy(raw[:], v)
			return pgtype.UUID{Bytes: raw, Valid: true}.String()
		}
		return base64.StdEncoding.EncodeToString(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}

// rawJSON returns JSON/JSONB as an embedded JSON value rather than a string.
func rawJSON(value any) any {
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

// defaultEncode handles bools, numbers, text and time, and makes deliberate,
// type-aware choices for raw bytes: valid UTF-8 becomes text, other byte
// payloads become base64 (distinct from the explicit BYTEA rule above).
func defaultEncode(value any) any {
	switch v := value.(type) {
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
		return base64.StdEncoding.EncodeToString(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case []any:
		out := make([]any, len(v))
		for i, element := range v {
			out[i] = defaultEncode(element)
		}
		return out
	case string, bool, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, float32, float64:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
