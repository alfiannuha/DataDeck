package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	gosqlmysql "github.com/go-sql-driver/mysql"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestMySQLName(t *testing.T) {
	if got := (MySQL{}).Name(); got != model.DriverMySQL {
		t.Errorf("Name() = %q, want %q", got, model.DriverMySQL)
	}
}

func TestMySQLCapabilities(t *testing.T) {
	caps := (MySQL{}).Capabilities()
	if caps.Schemas {
		t.Error("MySQL Schemas capability should be false")
	}
	if !caps.ForeignKeys || !caps.Indexes || !caps.SSL || !caps.SSH {
		t.Errorf("unexpected capabilities: %+v", caps)
	}
	if caps.Dialect != "mysql" {
		t.Errorf("Dialect = %q, want mysql", caps.Dialect)
	}
	if caps.IdentifierQuote != "`" {
		t.Errorf("IdentifierQuote = %q, want backtick", caps.IdentifierQuote)
	}
}

func TestMySQLDSN(t *testing.T) {
	dsn := mysqlDSN(Config{
		Driver:   model.DriverMySQL,
		Host:     "db.internal",
		Port:     3306,
		Database: "app",
		Username: "alice",
		Password: "s3cret",
		SSLMode:  "require",
	})

	config, err := gosqlmysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("generated DSN is not parseable: %v", err)
	}
	if config.User != "alice" || config.Passwd != "s3cret" {
		t.Errorf("credentials not encoded correctly: user=%q", config.User)
	}
	if config.DBName != "app" {
		t.Errorf("DBName = %q, want app", config.DBName)
	}
	if config.Addr != "db.internal:3306" {
		t.Errorf("Addr = %q, want db.internal:3306", config.Addr)
	}
	if config.TLSConfig != "skip-verify" {
		t.Errorf("TLSConfig = %q, want skip-verify", config.TLSConfig)
	}
	if !config.ParseTime {
		t.Error("ParseTime should be enabled")
	}
}

func TestMySQLTLSConfig(t *testing.T) {
	cases := map[string]string{
		"":            "false",
		"disable":     "false",
		"allow":       "false",
		"prefer":      "preferred",
		"require":     "skip-verify",
		"verify-ca":   "true",
		"verify-full": "true",
		"bogus":       "false",
	}
	for input, want := range cases {
		if got := mysqlTLSConfig(input); got != want {
			t.Errorf("mysqlTLSConfig(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMySQLEscapeInDSN(t *testing.T) {
	// The driver's FormatDSN handles escaping; a parse round-trip must recover
	// the exact password even with special characters.
	password := "p@ss:wo/rd?#"
	dsn := mysqlDSN(Config{Host: "localhost", Port: 3306, Database: "db", Username: "user", Password: password})
	config, err := gosqlmysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN: %v", err)
	}
	if config.Passwd != password {
		t.Errorf("Passwd = %q, want %q", config.Passwd, password)
	}
}

func TestMySQLUnreachableIsConnectionError(t *testing.T) {
	manager := NewManager(Options{PingTimeout: 300 * time.Millisecond}, MySQL{})
	_, err := manager.Open(context.Background(), "c1", Config{
		Driver:   model.DriverMySQL,
		Host:     "127.0.0.1",
		Port:     1,
		Database: "app",
		Username: "u",
	})
	if !errors.Is(err, ErrConnection) {
		t.Errorf("Open(unreachable) error = %v, want ErrConnection", err)
	}
}

func TestMySQLSQLError(t *testing.T) {
	err := mysqlSQLError(&gosqlmysql.MySQLError{Number: 1064, Message: "syntax error near"})
	var sqlErr *SQLError
	if !errors.As(err, &sqlErr) {
		t.Fatalf("mysqlSQLError() = %T, want *SQLError", err)
	}
	if !sqlErr.Syntax || sqlErr.Code != "1064" || sqlErr.Driver != model.DriverMySQL {
		t.Errorf("unexpected SQLError: %+v", sqlErr)
	}

	base := errors.New("boom")
	if got := mysqlSQLError(base); got != base {
		t.Errorf("non-MySQL error should pass through, got %v", got)
	}
	if got := mysqlSQLError(nil); got != nil {
		t.Errorf("nil should stay nil, got %v", got)
	}
}

func TestEncodeMySQLValue(t *testing.T) {
	timestamp := time.Date(2026, 9, 25, 14, 32, 0, 0, time.UTC)
	cases := []struct {
		name     string
		typeName string
		value    any
		want     any
	}{
		{"null", "VARCHAR", nil, nil},
		{"signed bigint", "BIGINT", int64(9007199254740993), "9007199254740993"},
		{"unsigned bigint", "UNSIGNED BIGINT", uint64(18446744073709551615), "18446744073709551615"},
		{"int", "INT", int64(42), int64(42)},
		{"tinyint boolean-like", "TINYINT", int64(1), int64(1)},
		{"decimal", "DECIMAL", []byte("12345.67"), "12345.67"},
		{"double", "DOUBLE", float64(1.5), 1.5},
		{"text", "VARCHAR", []byte("hello"), "hello"},
		{"date", "DATE", timestamp, "2026-09-25"},
		{"datetime", "DATETIME", timestamp, "2026-09-25T14:32:00Z"},
		{"blob base64", "BLOB", []byte{0x00, 0xff}, base64.StdEncoding.EncodeToString([]byte{0x00, 0xff})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := encodeMySQLValue(tc.typeName, tc.value); got != tc.want {
				t.Errorf("encodeMySQLValue(%q, %v) = %v (%T), want %v", tc.typeName, tc.value, got, got, tc.want)
			}
		})
	}

	jsonValue := encodeMySQLValue("JSON", []byte(`{"a":1}`))
	if raw, ok := jsonValue.(json.RawMessage); !ok || string(raw) != `{"a":1}` {
		t.Errorf("JSON value = %v (%T), want RawMessage", jsonValue, jsonValue)
	}
}
