package database

import (
	"context"
	"errors"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestDropSupported(t *testing.T) {
	if !DropSupported(model.DriverPostgres) || !DropSupported(model.DriverMySQL) {
		t.Error("postgres/mysql must support drop")
	}
	if DropSupported(model.DriverSQLite) {
		t.Error("sqlite must not support drop")
	}
}

func TestIsProtectedDatabase(t *testing.T) {
	pgProtected := []string{"postgres", "POSTGRES", "template0", "template1"}
	for _, name := range pgProtected {
		if !IsProtectedDatabase(model.DriverPostgres, name) {
			t.Errorf("postgres %q should be protected", name)
		}
	}
	if IsProtectedDatabase(model.DriverPostgres, "alpha") {
		t.Error("postgres alpha should not be protected")
	}

	mysqlProtected := []string{"mysql", "information_schema", "performance_schema", "sys", "SYS"}
	for _, name := range mysqlProtected {
		if !IsProtectedDatabase(model.DriverMySQL, name) {
			t.Errorf("mysql %q should be protected", name)
		}
	}
	if IsProtectedDatabase(model.DriverMySQL, "app") {
		t.Error("mysql app should not be protected")
	}

	// SQLite is never a drop target.
	if !IsProtectedDatabase(model.DriverSQLite, "anything") {
		t.Error("sqlite must be treated as protected/unsupported")
	}
}

func TestDropDatabaseUnsupportedEngine(t *testing.T) {
	m := NewManager(DefaultOptions()) // no connectors registered
	err := m.DropDatabase(context.Background(), "c1", Config{Driver: model.DriverSQLite, Database: "x"}, "x")
	if !errors.Is(err, ErrUnsupportedDriver) && !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("err = %v, want unsupported/not-implemented", err)
	}
}
