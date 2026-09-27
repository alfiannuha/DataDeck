//go:build integration

package database_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// TestPostgresIntrospectionIntegration creates a deterministic schema and
// verifies the database-neutral introspection output. Opt-in via build tag and
// DATADECK_TEST_PG_HOST (see postgres_integration_test.go for the variables).
func TestPostgresIntrospectionIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_PG_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping introspection integration test")
	}

	port := 5432
	if raw := os.Getenv("DATADECK_TEST_PG_PORT"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_PG_PORT %q: %v", raw, err)
		}
		port = value
	}

	cfg := database.Config{
		Driver:   model.DriverPostgres,
		Host:     host,
		Port:     port,
		Database: envOr("DATADECK_TEST_PG_DATABASE", "postgres"),
		Username: os.Getenv("DATADECK_TEST_PG_USER"),
		Password: os.Getenv("DATADECK_TEST_PG_PASSWORD"),
		SSLMode:  envOr("DATADECK_TEST_PG_SSLMODE", "disable"),
	}

	ctx := context.Background()
	manager := database.NewManager(database.DefaultOptions(), database.Postgres{})
	db, err := manager.Open(ctx, "itest-introspect", cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	for _, stmt := range []string{
		`CREATE SCHEMA IF NOT EXISTS datadeck_itest`,
		`CREATE TABLE datadeck_itest.users (
			id bigint PRIMARY KEY,
			email varchar(255) NOT NULL,
			nickname varchar(255),
			metadata jsonb,
			created_at timestamptz
		)`,
		`CREATE INDEX idx_users_email ON datadeck_itest.users(email)`,
		`CREATE TABLE datadeck_itest.roles (
			id bigint PRIMARY KEY,
			name text NOT NULL UNIQUE
		)`,
		`CREATE TABLE datadeck_itest.user_roles (
			user_id bigint NOT NULL REFERENCES datadeck_itest.users(id) ON DELETE CASCADE,
			role_id bigint NOT NULL REFERENCES datadeck_itest.roles(id) ON DELETE CASCADE,
			PRIMARY KEY (user_id, role_id)
		)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS datadeck_itest CASCADE`)
	}()

	databases, err := manager.Introspect(ctx, "itest-introspect", cfg)
	if err != nil {
		t.Fatalf("Introspect() error = %v", err)
	}
	if len(databases) != 1 {
		t.Fatalf("databases = %d, want 1", len(databases))
	}

	schema := findSchema(t, databases[0], "datadeck_itest")
	users := findTable(t, schema, "users")
	roles := findTable(t, schema, "roles")
	userRoles := findTable(t, schema, "user_roles")

	// Columns and nullability.
	if col := findColumn(t, users, "id"); col.Nullable {
		t.Error("users.id should not be nullable")
	}
	if col := findColumn(t, users, "email"); col.Nullable {
		t.Error("users.email should not be nullable")
	}
	if col := findColumn(t, users, "nickname"); !col.Nullable {
		t.Error("users.nickname should be nullable")
	}
	if col := findColumn(t, users, "email"); col.DataType == "" {
		t.Error("users.email data type is empty")
	}

	// Primary keys.
	if users.PrimaryKey == nil || !equalStrings(users.PrimaryKey.Columns, []string{"id"}) {
		t.Errorf("users primary key = %+v, want [id]", users.PrimaryKey)
	}
	if userRoles.PrimaryKey == nil || !equalStrings(userRoles.PrimaryKey.Columns, []string{"user_id", "role_id"}) {
		t.Errorf("user_roles primary key = %+v, want [user_id role_id]", userRoles.PrimaryKey)
	}
	if roles.PrimaryKey == nil || !equalStrings(roles.PrimaryKey.Columns, []string{"id"}) {
		t.Errorf("roles primary key = %+v, want [id]", roles.PrimaryKey)
	}

	// Foreign keys (composite-aware, column-by-column).
	if len(userRoles.ForeignKeys) != 2 {
		t.Fatalf("user_roles foreign keys = %d, want 2", len(userRoles.ForeignKeys))
	}
	if fk := findForeignKey(t, userRoles, "users"); fk == nil || !equalStrings(fk.Columns, []string{"user_id"}) ||
		!equalStrings(fk.ReferencedColumns, []string{"id"}) {
		t.Errorf("user_roles -> users FK = %+v", fk)
	}
	if fk := findForeignKey(t, userRoles, "roles"); fk == nil || !equalStrings(fk.Columns, []string{"role_id"}) ||
		!equalStrings(fk.ReferencedColumns, []string{"id"}) {
		t.Errorf("user_roles -> roles FK = %+v", fk)
	}

	// Index metadata.
	if findIndex(users, "idx_users_email") == nil {
		t.Error("users index idx_users_email not found")
	}
}

func findSchema(t *testing.T, database model.Database, name string) model.Schema {
	t.Helper()
	for _, schema := range database.Schemas {
		if schema.Name == name {
			return schema
		}
	}
	t.Fatalf("schema %q not found in %d schemas", name, len(database.Schemas))
	return model.Schema{}
}

func findTable(t *testing.T, schema model.Schema, name string) model.Table {
	t.Helper()
	for _, table := range schema.Tables {
		if table.Name == name {
			return table
		}
	}
	t.Fatalf("table %q not found in schema %q", name, schema.Name)
	return model.Table{}
}

func findColumn(t *testing.T, table model.Table, name string) model.Column {
	t.Helper()
	for _, column := range table.Columns {
		if column.Name == name {
			return column
		}
	}
	t.Fatalf("column %q not found in table %q", name, table.Name)
	return model.Column{}
}

func findForeignKey(t *testing.T, table model.Table, referencedTable string) *model.ForeignKey {
	t.Helper()
	for i := range table.ForeignKeys {
		if table.ForeignKeys[i].ReferencedTable == referencedTable {
			return &table.ForeignKeys[i]
		}
	}
	return nil
}

func findIndex(table model.Table, name string) *model.Index {
	for i := range table.Indexes {
		if table.Indexes[i].Name == name {
			return &table.Indexes[i]
		}
	}
	return nil
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
