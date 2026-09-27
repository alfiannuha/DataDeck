package database

import (
	"errors"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestPostgresName(t *testing.T) {
	if got := (Postgres{}).Name(); got != model.DriverPostgres {
		t.Errorf("Name() = %q, want %q", got, model.DriverPostgres)
	}
}

func TestPostgresDSN(t *testing.T) {
	dsn := postgresDSN(Config{
		Driver:   model.DriverPostgres,
		Host:     "db.internal",
		Port:     5432,
		Database: "app",
		Username: "alice",
		Password: "s3cret",
		SSLMode:  "require",
	})

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("postgresDSN() produced an unparsable URL: %v", err)
	}
	if parsed.Scheme != "postgres" {
		t.Errorf("scheme = %q, want postgres", parsed.Scheme)
	}
	if parsed.Host != "db.internal:5432" {
		t.Errorf("host = %q, want db.internal:5432", parsed.Host)
	}
	if parsed.Path != "/app" {
		t.Errorf("path = %q, want /app", parsed.Path)
	}
	if parsed.User.Username() != "alice" {
		t.Errorf("username = %q, want alice", parsed.User.Username())
	}
	if pw, _ := parsed.User.Password(); pw != "s3cret" {
		t.Errorf("password not encoded/decoded correctly")
	}
	if got := parsed.Query().Get("sslmode"); got != "require" {
		t.Errorf("sslmode = %q, want require", got)
	}
}

func TestPostgresDSNDefaultsSSLMode(t *testing.T) {
	dsn := postgresDSN(Config{Host: "localhost", Port: 5432, Database: "db"})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	if got := parsed.Query().Get("sslmode"); got != "disable" {
		t.Errorf("sslmode = %q, want disable", got)
	}
}

func TestPostgresDSNWithoutCredentials(t *testing.T) {
	dsn := postgresDSN(Config{Host: "localhost", Port: 5432, Database: "db"})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	if parsed.User != nil {
		t.Errorf("userinfo = %v, want nil when no credentials", parsed.User)
	}
}

func TestPostgresDSNEscapesSpecialCharacters(t *testing.T) {
	const password = "p@ss:w/rd?#"
	dsn := postgresDSN(Config{
		Host: "localhost", Port: 5432, Database: "db",
		Username: "user", Password: password,
	})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	if pw, _ := parsed.User.Password(); pw != password {
		t.Errorf("password = %q, want %q", pw, password)
	}
}

func TestPostgresCapabilities(t *testing.T) {
	caps := (Postgres{}).Capabilities()
	if !caps.Schemas || !caps.ForeignKeys || !caps.Indexes || !caps.SSL || !caps.SSH {
		t.Errorf("unexpected capabilities: %+v", caps)
	}
	if caps.Dialect != "postgres" {
		t.Errorf("Dialect = %q, want postgres", caps.Dialect)
	}
	if caps.IdentifierQuote != `"` {
		t.Errorf("IdentifierQuote = %q, want double quote", caps.IdentifierQuote)
	}
}

func TestPostgresSQLError(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "42601", Message: "syntax error at or near \"WHERRE\"", Position: 8}
	err := postgresSQLError(pgErr)

	var sqlErr *SQLError
	if !errors.As(err, &sqlErr) {
		t.Fatalf("toSQLError() = %T, want *SQLError", err)
	}
	if sqlErr.Code != "42601" || sqlErr.Position != 8 || sqlErr.Driver != model.DriverPostgres {
		t.Errorf("unexpected SQLError: %+v", sqlErr)
	}
	if sqlErr.Message != pgErr.Message {
		t.Errorf("Message = %q, want %q", sqlErr.Message, pgErr.Message)
	}
	if !sqlErr.Syntax {
		t.Error("SQLSTATE 42601 should be classified as a syntax error")
	}

	base := errors.New("network boom")
	if got := postgresSQLError(base); got != base {
		t.Errorf("non-PgError should pass through unchanged, got %v", got)
	}
	if got := postgresSQLError(nil); got != nil {
		t.Errorf("nil should stay nil, got %v", got)
	}
}
