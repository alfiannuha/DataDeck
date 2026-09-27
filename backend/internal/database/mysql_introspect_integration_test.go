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

// TestMySQLIntrospectionIntegration creates a deterministic fixture and
// verifies the neutral introspection output. Skips (NOT RUN) without
// DATADECK_TEST_MYSQL_HOST.
func TestMySQLIntrospectionIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_MYSQL_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_MYSQL_HOST not set; skipping MySQL introspection integration test")
	}

	port := 3306
	if raw := os.Getenv("DATADECK_TEST_MYSQL_PORT"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_MYSQL_PORT %q: %v", raw, err)
		}
		port = value
	}

	cfg := database.Config{
		Driver:   model.DriverMySQL,
		Host:     host,
		Port:     port,
		Database: envOr("DATADECK_TEST_MYSQL_DATABASE", "datadeck_test"),
		Username: envOr("DATADECK_TEST_MYSQL_USER", "root"),
		Password: os.Getenv("DATADECK_TEST_MYSQL_PASSWORD"),
		SSLMode:  envOr("DATADECK_TEST_MYSQL_SSLMODE", "disable"),
	}

	ctx := context.Background()
	manager := database.NewManager(database.DefaultOptions(), database.MySQL{})
	db, err := manager.Open(ctx, "mit-introspect", cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	for _, stmt := range []string{
		`DROP TABLE IF EXISTS user_roles`,
		`DROP TABLE IF EXISTS users`,
		`DROP TABLE IF EXISTS roles`,
		`CREATE TABLE roles (id BIGINT PRIMARY KEY, name VARCHAR(64) NOT NULL UNIQUE)`,
		`CREATE TABLE users (
			id BIGINT PRIMARY KEY,
			email VARCHAR(255) NOT NULL,
			nickname VARCHAR(255) NULL,
			metadata JSON NULL,
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX idx_users_email ON users(email)`,
		`CREATE TABLE user_roles (
			user_id BIGINT NOT NULL,
			role_id BIGINT NOT NULL,
			PRIMARY KEY (user_id, role_id),
			CONSTRAINT fk_ur_user FOREIGN KEY (user_id) REFERENCES users(id),
			CONSTRAINT fk_ur_role FOREIGN KEY (role_id) REFERENCES roles(id)
		)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	databases, err := manager.Introspect(ctx, "mit-introspect", cfg)
	if err != nil {
		t.Fatalf("Introspect() error = %v", err)
	}
	if len(databases) != 1 {
		t.Fatalf("databases = %d, want 1", len(databases))
	}
	databaseNode := databases[0]
	if databaseNode.Name != cfg.Database {
		t.Errorf("database name = %q, want %q", databaseNode.Name, cfg.Database)
	}
	if len(databaseNode.Schemas) != 0 {
		t.Errorf("MySQL should have no schema level, got %d schemas", len(databaseNode.Schemas))
	}

	users := findTableByName(t, databaseNode, "users")
	roles := findTableByName(t, databaseNode, "roles")
	userRoles := findTableByName(t, databaseNode, "user_roles")

	// Columns, types, nullability.
	if col := findColumnByName(t, users, "id"); col.Nullable {
		t.Error("users.id should not be nullable")
	}
	if col := findColumnByName(t, users, "email"); col.Nullable || col.DataType == "" {
		t.Errorf("users.email = %+v", col)
	}
	if col := findColumnByName(t, users, "nickname"); !col.Nullable {
		t.Error("users.nickname should be nullable")
	}
	if col := findColumnByName(t, users, "metadata"); col.DataType == "" {
		t.Error("users.metadata data type is empty")
	}

	// Primary keys.
	if users.PrimaryKey == nil || !equalStrings(users.PrimaryKey.Columns, []string{"id"}) {
		t.Errorf("users primary key = %+v, want [id]", users.PrimaryKey)
	}
	if roles.PrimaryKey == nil || !equalStrings(roles.PrimaryKey.Columns, []string{"id"}) {
		t.Errorf("roles primary key = %+v, want [id]", roles.PrimaryKey)
	}
	if userRoles.PrimaryKey == nil || !equalStrings(userRoles.PrimaryKey.Columns, []string{"user_id", "role_id"}) {
		t.Errorf("user_roles primary key = %+v, want [user_id role_id]", userRoles.PrimaryKey)
	}

	// Foreign keys.
	if len(userRoles.ForeignKeys) != 2 {
		t.Fatalf("user_roles foreign keys = %d, want 2", len(userRoles.ForeignKeys))
	}
	if fk := findForeignKeyTo(userRoles, "users"); fk == nil || !equalStrings(fk.Columns, []string{"user_id"}) {
		t.Errorf("user_roles -> users FK = %+v", fk)
	}
	if fk := findForeignKeyTo(userRoles, "roles"); fk == nil || !equalStrings(fk.Columns, []string{"role_id"}) {
		t.Errorf("user_roles -> roles FK = %+v", fk)
	}

	// Indexes (including PRIMARY).
	if findIndexByName(users, "idx_users_email") == nil {
		t.Error("users index idx_users_email not found")
	}
	if primary := findIndexByName(users, "PRIMARY"); primary == nil || !primary.Primary {
		t.Error("users PRIMARY index not found or not marked primary")
	}
}

func findTableByName(t *testing.T, database model.Database, name string) model.Table {
	t.Helper()
	for _, table := range database.Tables {
		if table.Name == name {
			return table
		}
	}
	t.Fatalf("table %q not found in database %q", name, database.Name)
	return model.Table{}
}

func findColumnByName(t *testing.T, table model.Table, name string) model.Column {
	t.Helper()
	for _, column := range table.Columns {
		if column.Name == name {
			return column
		}
	}
	t.Fatalf("column %q not found in table %q", name, table.Name)
	return model.Column{}
}

func findForeignKeyTo(table model.Table, referencedTable string) *model.ForeignKey {
	for i := range table.ForeignKeys {
		if table.ForeignKeys[i].ReferencedTable == referencedTable {
			return &table.ForeignKeys[i]
		}
	}
	return nil
}

func findIndexByName(table model.Table, name string) *model.Index {
	for i := range table.Indexes {
		if table.Indexes[i].Name == name {
			return &table.Indexes[i]
		}
	}
	return nil
}
