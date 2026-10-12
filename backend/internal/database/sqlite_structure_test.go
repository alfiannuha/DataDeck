package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestSQLiteTableStructure(t *testing.T) {
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
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			nickname TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			email_upper TEXT GENERATED ALWAYS AS (upper(email)) STORED
		)`,
		`CREATE TABLE posts (
			id INTEGER PRIMARY KEY,
			author_id INTEGER NOT NULL,
			slug TEXT NOT NULL,
			body TEXT,
			UNIQUE (author_id, slug),
			CONSTRAINT fk_posts_author FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE RESTRICT
		)`,
		`CREATE INDEX idx_posts_body ON posts(body) WHERE body IS NOT NULL`,
		`CREATE VIEW active_users AS SELECT id, email FROM users`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	users, err := manager.TableStructure(context.Background(), "s1", cfg, "", "users")
	if err != nil {
		t.Fatalf("TableStructure(users) error = %v", err)
	}
	if users.Table != "users" {
		t.Errorf("table = %q, want users", users.Table)
	}
	if users.Database != path {
		t.Errorf("database = %q, want %q", users.Database, path)
	}
	if users.ObjectType != "BASE TABLE" {
		t.Errorf("object_type = %q, want BASE TABLE", users.ObjectType)
	}

	// Columns in ordinal order with mutability mirroring Table Data.
	if len(users.Columns) != 5 {
		t.Fatalf("columns = %d, want 5", len(users.Columns))
	}
	if users.Columns[0].Name != "id" || users.Columns[0].OrdinalPosition != 1 {
		t.Errorf("first column = %+v", users.Columns[0])
	}
	if !users.Columns[0].PrimaryKey || users.Columns[0].PrimaryKeyPosition != 1 {
		t.Errorf("id PK flags = %+v", users.Columns[0])
	}
	if !users.Columns[0].Identity {
		t.Error("id should be identity (rowid alias)")
	}
	if users.Columns[0].Updatable {
		t.Error("id (identity+PK) must not be updatable")
	}
	if users.Columns[1].NormalizedType != "text" {
		t.Errorf("email normalized type = %q, want text", users.Columns[1].NormalizedType)
	}
	if !users.Columns[2].Nullable {
		t.Error("nickname should be nullable")
	}
	if users.Columns[2].Insertable != true || users.Columns[2].Updatable != true {
		t.Errorf("nickname mutability = insertable %v, updatable %v", users.Columns[2].Insertable, users.Columns[2].Updatable)
	}
	if !users.Columns[4].Generated {
		t.Error("email_upper should be generated")
	}
	if users.Columns[4].Insertable || users.Columns[4].Updatable {
		t.Error("generated column must not be insertable or updatable")
	}

	// Primary key.
	if users.PrimaryKey == nil || !equalStrings(users.PrimaryKey.Columns, []string{"id"}) {
		t.Errorf("users PK = %+v", users.PrimaryKey)
	}

	// Constraints: unique (email), primary, foreign key (posts→users).
	var emailConstraint, pkConstraint *model.StructureConstraint
	for i := range users.Constraints {
		c := &users.Constraints[i]
		if c.Type == "UNIQUE" && len(c.Columns) == 1 && c.Columns[0] == "email" {
			emailConstraint = c
		}
		if c.Type == "PRIMARY KEY" {
			pkConstraint = c
		}
	}
	if emailConstraint == nil {
		t.Error("expected a UNIQUE constraint on email")
	}
	if pkConstraint == nil || !equalStrings(pkConstraint.Columns, []string{"id"}) {
		t.Errorf("PRIMARY KEY constraint = %+v", pkConstraint)
	}

	// No fabricated CHECK constraints on SQLite.
	for i := range users.Constraints {
		if users.Constraints[i].Type == "CHECK" {
			t.Errorf("CHECK constraint should not be fabricated: %+v", users.Constraints[i])
		}
	}

	// Indexes: PRIMARY + the user index, no autoindexes, partial flag set.
	if findStructureIndex(users.Indexes, "PRIMARY") == nil {
		t.Error("expected PRIMARY index")
	}
	if idx := findStructureIndex(users.Indexes, "idx_posts_body"); idx != nil {
		t.Error("idx_posts_body is defined on posts, not users")
	}
	for _, index := range users.Indexes {
		if len(index.Name) >= 16 && index.Name[:16] == "sqlite_autoindex" {
			t.Errorf("autoindex %q should be filtered from indexes", index.Name)
		}
	}

	posts, err := manager.TableStructure(context.Background(), "s1", cfg, "", "posts")
	if err != nil {
		t.Fatalf("TableStructure(posts) error = %v", err)
	}
	if findStructureIndex(posts.Indexes, "idx_posts_body") == nil {
		t.Error("expected idx_posts_body on posts")
	} else {
		idx := findStructureIndex(posts.Indexes, "idx_posts_body")
		if !idx.Partial {
			t.Error("idx_posts_body should be partial")
		}
	}
	if len(posts.ForeignKeys) != 1 {
		t.Fatalf("posts FKs = %d, want 1", len(posts.ForeignKeys))
	}
	fk := posts.ForeignKeys[0]
	if fk.Name != "fk_posts_0" || !equalStrings(fk.Columns, []string{"author_id"}) {
		t.Errorf("posts FK = %+v", fk)
	}
	if fk.ReferencedTable != "users" || !equalStrings(fk.ReferencedColumns, []string{"id"}) {
		t.Errorf("posts FK references = table %q columns %v", fk.ReferencedTable, fk.ReferencedColumns)
	}
	if fk.OnDelete != "CASCADE" || fk.OnUpdate != "RESTRICT" {
		t.Errorf("posts FK actions = update %q delete %q, want RESTRICT/CASCADE", fk.OnUpdate, fk.OnDelete)
	}
	var fkConstraint *model.StructureConstraint
	for i := range posts.Constraints {
		c := &posts.Constraints[i]
		if c.Type == "FOREIGN KEY" && equalStrings(c.Columns, []string{"author_id"}) {
			fkConstraint = c
		}
	}
	if fkConstraint == nil {
		t.Error("expected a FOREIGN KEY constraint entry for posts")
	}

	// Views are read-only but still introspectable.
	view, err := manager.TableStructure(context.Background(), "s1", cfg, "", "active_users")
	if err != nil {
		t.Fatalf("TableStructure(view) error = %v", err)
	}
	if view.ObjectType != "VIEW" {
		t.Errorf("view object_type = %q, want VIEW", view.ObjectType)
	}
	if len(view.Columns) != 2 {
		t.Errorf("view columns = %d, want 2", len(view.Columns))
	}
	for _, col := range view.Columns {
		if col.Insertable || col.Updatable {
			t.Errorf("view column %q should not be insertable/updatable", col.Name)
		}
	}

	if _, err := manager.TableStructure(context.Background(), "s1", cfg, "", "missing_table"); err == nil {
		t.Fatal("missing table should return an error")
	} else if err.Error() == "" {
		t.Fatal("missing table error should not be empty")
	}
}

func findStructureIndex(indexes []model.StructureIndex, name string) *model.StructureIndex {
	for i := range indexes {
		if indexes[i].Name == name {
			return &indexes[i]
		}
	}
	return nil
}

func TestStructureColumnMutability(t *testing.T) {
	cases := []struct {
		name           string
		generated      bool
		identity       bool
		primaryKey     bool
		wantInsertable bool
		wantUpdatable  bool
	}{
		{name: "plain", generated: false, identity: false, primaryKey: false, wantInsertable: true, wantUpdatable: true},
		{name: "pk", primaryKey: true, wantInsertable: true, wantUpdatable: false},
		{name: "identity", identity: true, wantInsertable: false, wantUpdatable: false},
		{name: "generated", generated: true, wantInsertable: false, wantUpdatable: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			insertable, updatable := structureColumnMutability(tc.generated, tc.identity, tc.primaryKey)
			if insertable != tc.wantInsertable || updatable != tc.wantUpdatable {
				t.Errorf("mutability = (%v, %v), want (%v, %v)", insertable, updatable, tc.wantInsertable, tc.wantUpdatable)
			}
		})
	}
}

func TestNormalizedType(t *testing.T) {
	cases := map[string]string{
		"character varying(20)":    "varchar",
		"varchar(255)":             "varchar",
		"int4":                     "integer",
		"int8":                     "bigint",
		"double precision":         "double",
		"timestamp with time zone": "timestamp",
		"numeric(10,2)":            "numeric",
		"UNKNOWN_TYPE":             "unknown_type",
		"citext":                   "text",
		"jsonb":                    "json",
	}
	for input, want := range cases {
		if got := normalizedType(input); got != want {
			t.Errorf("normalizedType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMySQLCheckConstraintSupport(t *testing.T) {
	cases := map[string]bool{
		"8.0.15":                  false,
		"8.0.16":                  true,
		"8.0.36":                  true,
		"5.7.44":                  false,
		"9.0.1":                   true,
		"10.1.26-MariaDB":         false,
		"10.2.0-MariaDB":          true,
		"10.5.19-MariaDB":         true,
		"10.6.7-MariaDB":          true,
		"5.5.5-10.11.2-MariaDB":   true,
		"5.5.62-MariaDB":          false,
		"8.0.36-0ubuntu0.24.04.1": true,
	}
	for version, want := range cases {
		if got := mysqlSupportsCheckConstraints(version); got != want {
			t.Errorf("mysqlSupportsCheckConstraints(%q) = %v, want %v", version, got, want)
		}
	}
}
