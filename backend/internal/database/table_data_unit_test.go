package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// browseFake is a Connector stub that returns fixed introspection metadata and
// records the generated SQL so browse behaviour can be asserted without a DB.
type browseFake struct {
	driver model.Driver
	quote  string
	dbs    []model.Database
	result model.QueryResult
	err    error
	sql    string
}

func (f *browseFake) Name() model.Driver { return f.driver }

func (f *browseFake) Open(context.Context, Config, Options) (*sql.DB, error) {
	return sql.OpenDB(fakeSQLConnector{d: fakeSQLDriver{}}), nil
}
func (f *browseFake) Ping(ctx context.Context, db *sql.DB) error { return db.PingContext(ctx) }
func (f *browseFake) Introspect(context.Context, *sql.DB) ([]model.Database, error) {
	return f.dbs, nil
}
func (f *browseFake) Execute(_ context.Context, _ *sql.DB, sqlText string) (model.QueryResult, error) {
	f.sql = sqlText
	if f.err != nil {
		return model.QueryResult{}, f.err
	}
	return f.result, nil
}
func (f *browseFake) Capabilities() model.Capabilities {
	return model.Capabilities{Dialect: string(f.driver), IdentifierQuote: f.quote}
}

func browseFixture() []model.Database {
	return []model.Database{{
		Name: "alpha",
		Schemas: []model.Schema{{
			Name: "public",
			Tables: []model.Table{{
				Schema: "public",
				Name:   "users",
				Type:   "BASE TABLE",
				Columns: []model.Column{
					{Name: "name", DataType: "text", Nullable: true, OrdinalPosition: 2},
					{Name: "id", DataType: "bigint", Nullable: false, OrdinalPosition: 1},
				},
				PrimaryKey: &model.PrimaryKey{Name: "users_pkey", Columns: []string{"id"}},
			}},
		}},
	}}
}

func TestBrowseTableGeneratesExplicitQuotedSelect(t *testing.T) {
	fake := &browseFake{
		driver: model.DriverPostgres,
		quote:  `"`,
		dbs:    browseFixture(),
		result: model.QueryResult{
			Columns: []model.QueryColumn{{Name: "id"}, {Name: "name"}},
			Rows:    [][]any{{"1", "Alfian"}, {"2", "Bob"}, {"3", "Cara"}},
		},
	}
	m := NewManager(DefaultOptions(), fake)

	page, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverPostgres, Database: "alpha"}, TableBrowseRequest{
		Schema: "public", Table: "users", Limit: 2, Offset: 0,
	})
	if err != nil {
		t.Fatalf("BrowseTable() error = %v", err)
	}

	// Explicit quoted columns in ordinal order; no SELECT *.
	wantSQL := `SELECT "id", "name" FROM "public"."users" LIMIT 3 OFFSET 0`
	if fake.sql != wantSQL {
		t.Errorf("SQL = %q, want %q", fake.sql, wantSQL)
	}
	if strings.Contains(fake.sql, "*") {
		t.Errorf("SQL must not use SELECT *: %q", fake.sql)
	}
	if len(page.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (page_size bound)", len(page.Rows))
	}
	if !page.Pagination.HasMore {
		t.Error("has_more = false, want true (limit+1 row present)")
	}
	if page.Pagination.PageSize != 2 {
		t.Errorf("page_size = %d, want 2", page.Pagination.PageSize)
	}
	if len(page.Columns) != 2 || page.Columns[0].Name != "id" || !page.Columns[0].PrimaryKey {
		t.Errorf("columns = %+v, want id first with primary_key=true", page.Columns)
	}
	if page.Columns[1].Name != "name" || !page.Columns[1].Nullable {
		t.Errorf("columns[1] = %+v, want name nullable", page.Columns[1])
	}
	if page.ObjectType != "BASE TABLE" {
		t.Errorf("object_type = %q", page.ObjectType)
	}
}

func TestBrowseTableOffsetAndTruncation(t *testing.T) {
	fake := &browseFake{
		driver: model.DriverPostgres,
		quote:  `"`,
		dbs:    browseFixture(),
		result: model.QueryResult{
			Rows:      [][]any{{"1", "x"}},
			Truncated: true,
		},
	}
	m := NewManager(DefaultOptions(), fake)

	page, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverPostgres, Database: "alpha"}, TableBrowseRequest{
		Schema: "public", Table: "users", Limit: 100, Offset: 200,
	})
	if err != nil {
		t.Fatalf("BrowseTable() error = %v", err)
	}
	if !strings.HasSuffix(fake.sql, "LIMIT 101 OFFSET 200") {
		t.Errorf("SQL = %q, want LIMIT 101 OFFSET 200", fake.sql)
	}
	if !page.Truncated {
		t.Error("truncated = false, want propagated from the result cap")
	}
	if page.Pagination.HasMore {
		t.Error("has_more = true, want false (single row returned)")
	}
}

func TestBrowseTableQuotePerDriver(t *testing.T) {
	fake := &browseFake{
		driver: model.DriverMySQL,
		quote:  "`",
		dbs: []model.Database{{
			Name: "app",
			Tables: []model.Table{{
				Name:    "orders",
				Type:    "BASE TABLE",
				Columns: []model.Column{{Name: "id", DataType: "int", OrdinalPosition: 1}},
			}},
		}},
	}
	m := NewManager(DefaultOptions(), fake)

	if _, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverMySQL, Database: "app"}, TableBrowseRequest{
		Schema: "ignored", Table: "orders", Limit: 10, Offset: 0,
	}); err != nil {
		t.Fatalf("BrowseTable() error = %v", err)
	}
	if fake.sql != "SELECT `id` FROM `orders` LIMIT 11 OFFSET 0" {
		t.Errorf("SQL = %q, want backtick-quoted MySQL", fake.sql)
	}
}

func TestBrowseTableUnknownTable(t *testing.T) {
	fake := &browseFake{driver: model.DriverPostgres, quote: `"`, dbs: browseFixture()}
	m := NewManager(DefaultOptions(), fake)

	_, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverPostgres, Database: "alpha"}, TableBrowseRequest{
		Schema: "public", Table: "users; DROP TABLE users", Limit: 10,
	})
	if !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("error = %v, want ErrTableNotFound (identifier treated as metadata lookup)", err)
	}
	if fake.sql != "" {
		t.Errorf("no SQL must be generated for an unknown table, got %q", fake.sql)
	}
}

func TestBrowseTableUnknownSchema(t *testing.T) {
	fake := &browseFake{driver: model.DriverPostgres, quote: `"`, dbs: browseFixture()}
	m := NewManager(DefaultOptions(), fake)

	_, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverPostgres, Database: "alpha"}, TableBrowseRequest{
		Schema: "private", Table: "users", Limit: 10,
	})
	if !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("error = %v, want ErrTableNotFound", err)
	}
}

func TestBrowseTableEmptyRelationDoesNotExecute(t *testing.T) {
	fake := &browseFake{
		driver: model.DriverPostgres,
		quote:  `"`,
		dbs: []model.Database{{
			Schemas: []model.Schema{{Name: "public", Tables: []model.Table{{Name: "empty"}}}},
		}},
	}
	m := NewManager(DefaultOptions(), fake)

	page, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverPostgres}, TableBrowseRequest{
		Schema: "public", Table: "empty", Limit: 10,
	})
	if err != nil {
		t.Fatalf("BrowseTable() error = %v", err)
	}
	if fake.sql != "" {
		t.Errorf("no SQL expected for a relation with no columns, got %q", fake.sql)
	}
	if len(page.Rows) != 0 {
		t.Errorf("rows = %d, want 0", len(page.Rows))
	}
}
