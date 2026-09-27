package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestSQLiteIntrospection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.db")
	manager := NewManager(DefaultOptions(), SQLite{})
	cfg := Config{Driver: model.DriverSQLite, Database: path}

	db, err := manager.Open(context.Background(), "s1", cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	for _, stmt := range []string{
		`CREATE TABLE roles (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE)`,
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL, nickname TEXT, created_at TEXT)`,
		`CREATE INDEX idx_users_email ON users(email)`,
		`CREATE TABLE user_roles (
			user_id INTEGER NOT NULL,
			role_id INTEGER NOT NULL,
			PRIMARY KEY (user_id, role_id),
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (role_id) REFERENCES roles(id)
		)`,
		`CREATE TABLE seq_demo (id INTEGER PRIMARY KEY AUTOINCREMENT)`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	databases, err := manager.Introspect(context.Background(), "s1", cfg)
	if err != nil {
		t.Fatalf("Introspect() error = %v", err)
	}
	if len(databases) != 1 {
		t.Fatalf("databases = %d, want 1", len(databases))
	}
	databaseNode := databases[0]
	if len(databaseNode.Schemas) != 0 {
		t.Errorf("SQLite should have no schema level, got %d", len(databaseNode.Schemas))
	}

	names := map[string]bool{}
	for _, table := range databaseNode.Tables {
		names[table.Name] = true
	}
	for _, want := range []string{"users", "roles", "user_roles"} {
		if !names[want] {
			t.Errorf("expected table %q", want)
		}
	}
	for _, internal := range []string{"sqlite_master", "sqlite_sequence"} {
		if names[internal] {
			t.Errorf("internal object %q should be filtered", internal)
		}
	}

	users := findTableByName(t, databaseNode, "users")
	roles := findTableByName(t, databaseNode, "roles")
	userRoles := findTableByName(t, databaseNode, "user_roles")

	if col := findColumnByName(t, users, "email"); col.Nullable {
		t.Error("users.email should be NOT NULL")
	}
	if col := findColumnByName(t, users, "nickname"); !col.Nullable {
		t.Error("users.nickname should be nullable")
	}

	if users.PrimaryKey == nil || !equalStrings(users.PrimaryKey.Columns, []string{"id"}) {
		t.Errorf("users PK = %+v, want [id]", users.PrimaryKey)
	}
	if roles.PrimaryKey == nil || !equalStrings(roles.PrimaryKey.Columns, []string{"id"}) {
		t.Errorf("roles PK = %+v, want [id]", roles.PrimaryKey)
	}
	if userRoles.PrimaryKey == nil || !equalStrings(userRoles.PrimaryKey.Columns, []string{"user_id", "role_id"}) {
		t.Errorf("user_roles PK = %+v, want [user_id role_id]", userRoles.PrimaryKey)
	}

	if len(userRoles.ForeignKeys) != 2 {
		t.Fatalf("user_roles FKs = %d, want 2", len(userRoles.ForeignKeys))
	}
	if fk := findForeignKeyTo(userRoles, "users"); fk == nil || !equalStrings(fk.Columns, []string{"user_id"}) {
		t.Errorf("user_roles -> users FK = %+v", fk)
	}
	if fk := findForeignKeyTo(userRoles, "roles"); fk == nil || !equalStrings(fk.Columns, []string{"role_id"}) {
		t.Errorf("user_roles -> roles FK = %+v", fk)
	}

	if findIndexByName(users, "idx_users_email") == nil {
		t.Error("users index idx_users_email not found")
	}
	// A composite non-rowid PK produces a primary index (normalized to PRIMARY).
	if primary := findIndexByName(userRoles, "PRIMARY"); primary == nil || !primary.Primary {
		t.Error("user_roles PRIMARY index missing or not marked primary")
	}
	for _, table := range databaseNode.Tables {
		for _, index := range table.Indexes {
			if len(index.Name) >= 16 && index.Name[:16] == "sqlite_autoindex" {
				t.Errorf("autoindex %q should be filtered", index.Name)
			}
		}
	}
}
