package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	gosqlmysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// errEmitAborted wraps errors returned by a stream emit callback so callers can
// tell them apart from database errors and avoid re-classifying them (e.g.
// SQLite's sqliteSQLError would otherwise swallow a writer's row-cap error).
var errEmitAborted = errors.New("database: export sink aborted the stream")

// streamSQLTable drives one bounded SELECT over a *sql.Conn for the drivers
// whose encoders key off rows.ColumnTypes() type names (MySQL, SQLite). Rows
// are canonical driver-encoded values; an emit error cancels the cursor and is
// returned unchanged (wrapped in errEmitAborted).
func streamSQLTable(ctx context.Context, conn *sql.Conn, sqlText string, args []any, encode func(typeName string, value any) any, emit func(row []any) error) error {
	queryCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	rows, err := conn.QueryContext(queryCtx, sqlText, args...)
	if err != nil {
		return err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		_ = rows.Close()
		return err
	}
	typeNames := make([]string, len(columnTypes))
	for i, columnType := range columnTypes {
		typeNames[i] = columnType.DatabaseTypeName()
	}

	values := make([]any, len(columnTypes))
	scanTargets := make([]any, len(columnTypes))
	for i := range values {
		scanTargets[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan row: %w", err)
		}
		row := make([]any, len(values))
		for i, value := range values {
			row[i] = encode(typeNames[i], value)
		}
		if err := emit(row); err != nil {
			cancel()
			_ = rows.Close()
			return fmt.Errorf("%w: %w", errEmitAborted, err)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	return rows.Close()
}

// streamPostgresTable drives one bounded SELECT directly on the pgx connection
// (via database/sql's Raw bridge), reusing the exact type-map + encoder path of
// executePostgresQuery.
func streamPostgresTable(ctx context.Context, conn *pgx.Conn, sqlText string, args []any, emit func(row []any) error) error {
	queryCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	rows, err := conn.Query(queryCtx, sqlText, args...)
	if err != nil {
		return err
	}

	fields := rows.FieldDescriptions()
	typeNames := make([]string, len(fields))
	for i, field := range fields {
		typeName := ""
		if typ, ok := conn.TypeMap().TypeForOID(field.DataTypeOID); ok {
			typeName = typ.Name
		}
		typeNames[i] = strings.ToLower(typeName)
	}

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
		if err := emit(row); err != nil {
			cancel()
			rows.Close()
			return fmt.Errorf("%w: %w", errEmitAborted, err)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	return nil
}

// StreamTable implements TableStreamer for PostgreSQL (PRF-02/T14). Connection
// acquire and error classification mirror ExecuteArgs so exports and browsing
// surface identical error semantics.
func (Postgres) StreamTable(ctx context.Context, db *sql.DB, sqlText string, args []any, emit func(row []any) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		// A cached pool can outlive its target database (dropped after the pool
		// was created). Classify the acquire failure the same way Ping does.
		if classified := postgresConnectError(err); !errors.Is(classified, err) {
			return classified
		}
		if isConnectionLoss(err) {
			return fmt.Errorf("%w: %w", ErrConnection, err)
		}
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	err = conn.Raw(func(driverConn any) error {
		stdlibConn, ok := driverConn.(*stdlib.Conn)
		if !ok {
			return fmt.Errorf("unexpected driver connection type %T", driverConn)
		}
		return streamPostgresTable(ctx, stdlibConn.Conn(), sqlText, args, emit)
	})
	if err != nil {
		converted := postgresSQLError(err)
		if _, ok := asSQLError(converted); !ok && isConnectionLoss(converted) {
			return fmt.Errorf("%w: %w", ErrConnection, converted)
		}
		return converted
	}
	return nil
}

// StreamTable implements TableStreamer for MySQL (PRF-02/T14).
func (MySQL) StreamTable(ctx context.Context, db *sql.DB, sqlText string, args []any, emit func(row []any) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		if isConnectionLoss(err) {
			return fmt.Errorf("%w: %w", ErrConnection, err)
		}
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	err = streamSQLTable(ctx, conn, sqlText, args, encodeMySQLValue, emit)
	if err != nil {
		converted := mysqlSQLError(err)
		if _, ok := asSQLError(converted); !ok &&
			(isConnectionLoss(converted) || errors.Is(converted, gosqlmysql.ErrInvalidConn)) {
			return fmt.Errorf("%w: %w", ErrConnection, converted)
		}
		return converted
	}
	return nil
}

// StreamTable implements TableStreamer for SQLite (PRF-02/T14).
func (SQLite) StreamTable(ctx context.Context, db *sql.DB, sqlText string, args []any, emit func(row []any) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		if isConnectionLoss(err) {
			return fmt.Errorf("%w: %w", ErrConnection, err)
		}
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	err = streamSQLTable(ctx, conn, sqlText, args, encodeSQLiteValue, emit)
	if err != nil {
		// sqliteSQLError must not swallow sink (emit) errors — a writer abort is
		// a neutral, caller-visable condition, not a SQL failure.
		if errors.Is(err, errEmitAborted) {
			return err
		}
		return sqliteSQLError(err)
	}
	return nil
}
