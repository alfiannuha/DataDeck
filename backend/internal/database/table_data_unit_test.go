package database

import (
	"context"
	"database/sql"
	"encoding/json"
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
	args   []any
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

func (f *browseFake) ExecuteArgs(_ context.Context, _ *sql.DB, sqlText string, args []any) (model.QueryResult, error) {
	f.sql = sqlText
	f.args = args
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

func TestBrowseTableSortOrderByBeforeLimit(t *testing.T) {
	fake := &browseFake{
		driver: model.DriverPostgres,
		quote:  `"`,
		dbs:    browseFixture(),
		result: model.QueryResult{Rows: [][]any{{"2", "Bob"}}},
	}
	m := NewManager(DefaultOptions(), fake)

	_, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverPostgres, Database: "alpha"}, TableBrowseRequest{
		Schema: "public", Table: "users", Limit: 10, Offset: 20,
		Sort: &model.TableSort{Column: "name", Direction: "desc"},
	})
	if err != nil {
		t.Fatalf("BrowseTable() error = %v", err)
	}
	// ORDER BY (with PK tie-break) must precede LIMIT/OFFSET.
	want := `SELECT "id", "name" FROM "public"."users" ORDER BY "name" DESC, "id" ASC LIMIT 11 OFFSET 20`
	if fake.sql != want {
		t.Errorf("SQL = %q, want %q", fake.sql, want)
	}
}

func TestBrowseTableSortUnknownColumn(t *testing.T) {
	fake := &browseFake{driver: model.DriverPostgres, quote: `"`, dbs: browseFixture()}
	m := NewManager(DefaultOptions(), fake)

	_, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverPostgres, Database: "alpha"}, TableBrowseRequest{
		Schema: "public", Table: "users", Limit: 10,
		Sort: &model.TableSort{Column: `name"; DROP TABLE users`, Direction: "asc"},
	})
	if !errors.Is(err, ErrSortColumnNotFound) {
		t.Fatalf("error = %v, want ErrSortColumnNotFound", err)
	}
	if fake.sql != "" {
		t.Errorf("no SQL must be generated for an unknown sort column, got %q", fake.sql)
	}
}

func TestBrowseTableSortCompositePKTieBreak(t *testing.T) {
	fake := &browseFake{
		driver: model.DriverSQLite,
		quote:  `"`,
		dbs: []model.Database{{
			Tables: []model.Table{{
				Name: "order_items",
				Type: "BASE TABLE",
				Columns: []model.Column{
					{Name: "order_id", DataType: "integer", OrdinalPosition: 1},
					{Name: "product_id", DataType: "integer", OrdinalPosition: 2},
					{Name: "qty", DataType: "integer", OrdinalPosition: 3},
				},
				PrimaryKey: &model.PrimaryKey{Name: "pk", Columns: []string{"order_id", "product_id"}},
			}},
		}},
	}
	m := NewManager(DefaultOptions(), fake)

	if _, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverSQLite}, TableBrowseRequest{
		Table: "order_items", Limit: 5, Sort: &model.TableSort{Column: "qty", Direction: "asc"},
	}); err != nil {
		t.Fatalf("BrowseTable() error = %v", err)
	}
	if fake.sql != `SELECT "order_id", "product_id", "qty" FROM "order_items" ORDER BY "qty" ASC, "order_id" ASC, "product_id" ASC LIMIT 6 OFFSET 0` {
		t.Errorf("SQL = %q", fake.sql)
	}
}

func TestBrowseTableSortNoPKNoTieBreak(t *testing.T) {
	fake := &browseFake{
		driver: model.DriverSQLite,
		quote:  `"`,
		dbs: []model.Database{{
			Tables: []model.Table{{
				Name:    "logs",
				Type:    "BASE TABLE",
				Columns: []model.Column{{Name: "msg", DataType: "text", OrdinalPosition: 1}},
			}},
		}},
	}
	m := NewManager(DefaultOptions(), fake)
	if _, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverSQLite}, TableBrowseRequest{
		Table: "logs", Limit: 5, Sort: &model.TableSort{Column: "msg", Direction: "asc"},
	}); err != nil {
		t.Fatalf("BrowseTable() error = %v", err)
	}
	if fake.sql != `SELECT "msg" FROM "logs" ORDER BY "msg" ASC LIMIT 6 OFFSET 0` {
		t.Errorf("SQL = %q", fake.sql)
	}
}

func filterFake() *browseFake {
	return &browseFake{driver: model.DriverPostgres, quote: `"`, dbs: browseFixture()}
}

func browseWith(t *testing.T, fake *browseFake, filters []model.TableFilter) error {
	t.Helper()
	m := NewManager(DefaultOptions(), fake)
	_, err := m.BrowseTable(context.Background(), "c1", Config{Driver: fake.driver, Database: "alpha"}, TableBrowseRequest{
		Schema: "public", Table: "users", Limit: 10, Filters: filters,
	})
	return err
}

func TestFilterEqualsAndPlaceholders(t *testing.T) {
	fake := filterFake()
	if err := browseWith(t, fake, []model.TableFilter{{Column: "name", Operator: "equals", Value: "Alfian"}}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if fake.sql != `SELECT "id", "name" FROM "public"."users" WHERE "name" = $1 LIMIT 11 OFFSET 0` {
		t.Errorf("SQL = %q", fake.sql)
	}
	if len(fake.args) != 1 || fake.args[0] != "Alfian" {
		t.Errorf("args = %#v, want [Alfian]", fake.args)
	}
}

func TestFilterMultipleAndTagsAndSortOrder(t *testing.T) {
	fake := filterFake()
	filters := []model.TableFilter{
		{Column: "name", Operator: "equals", Value: "A"},
		{Column: "id", Operator: "greater_or_equal", Value: json.Number("18")},
	}
	m := NewManager(DefaultOptions(), fake)
	if _, err := m.BrowseTable(context.Background(), "c1", Config{Driver: model.DriverPostgres, Database: "alpha"}, TableBrowseRequest{
		Schema: "public", Table: "users", Limit: 5, Offset: 10, Filters: filters,
		Sort: &model.TableSort{Column: "name", Direction: "desc"},
	}); err != nil {
		t.Fatalf("error = %v", err)
	}
	want := `SELECT "id", "name" FROM "public"."users" WHERE "name" = $1 AND "id" >= $2 ORDER BY "name" DESC, "id" ASC LIMIT 6 OFFSET 10`
	if fake.sql != want {
		t.Errorf("SQL = %q, want %q", fake.sql, want)
	}
	if len(fake.args) != 2 || fake.args[1] != "18" {
		t.Errorf("args = %#v, want exact string 18", fake.args)
	}
}

func TestFilterContainsEscapesWildcards(t *testing.T) {
	fake := filterFake()
	if err := browseWith(t, fake, []model.TableFilter{{Column: "name", Operator: "contains", Value: "100%_"}}); err != nil {
		t.Fatalf("error = %v", err)
	}
	want := `SELECT "id", "name" FROM "public"."users" WHERE "name" LIKE $1 ESCAPE '\' LIMIT 11 OFFSET 0`
	if fake.sql != want {
		t.Errorf("SQL = %q, want %q", fake.sql, want)
	}
	if len(fake.args) != 1 || fake.args[0] != `%100\%\_%` {
		t.Errorf("args = %#v, want escaped pattern", fake.args)
	}
}

func TestFilterMySQLPlaceholder(t *testing.T) {
	fake := &browseFake{
		driver: model.DriverMySQL,
		quote:  "`",
		dbs: []model.Database{{Tables: []model.Table{{
			Name:    "users",
			Type:    "BASE TABLE",
			Columns: []model.Column{{Name: "status", DataType: "varchar", OrdinalPosition: 1}},
		}}}},
	}
	if err := browseWith(t, fake, []model.TableFilter{{Column: "status", Operator: "equals", Value: "active"}}); err != nil {
		t.Fatalf("error = %v", err)
	}
	want := "SELECT `status` FROM `users` WHERE `status` = ? LIMIT 11 OFFSET 0"
	if fake.sql != want {
		t.Errorf("SQL = %q, want %q", fake.sql, want)
	}
}

func TestFilterNullOperators(t *testing.T) {
	fake := filterFake()
	if err := browseWith(t, fake, []model.TableFilter{{Column: "name", Operator: "is_null"}}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if fake.sql != `SELECT "id", "name" FROM "public"."users" WHERE "name" IS NULL LIMIT 11 OFFSET 0` {
		t.Errorf("SQL = %q", fake.sql)
	}
	if len(fake.args) != 0 {
		t.Errorf("args = %#v, want none", fake.args)
	}

	if err := browseWith(t, fake, []model.TableFilter{{Column: "name", Operator: "is_null", Value: "x"}}); !errors.Is(err, ErrInvalidFilter) {
		t.Fatalf("is_null with value error = %v, want ErrInvalidFilter", err)
	}
}

func TestFilterInBounds(t *testing.T) {
	fake := filterFake()
	if err := browseWith(t, fake, []model.TableFilter{{Column: "name", Operator: "in", Values: []any{"a", "b"}}}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if fake.sql != `SELECT "id", "name" FROM "public"."users" WHERE "name" IN ($1, $2) LIMIT 11 OFFSET 0` {
		t.Errorf("SQL = %q", fake.sql)
	}

	if err := browseWith(t, fake, []model.TableFilter{{Column: "name", Operator: "in"}}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("empty IN error = %v, want ErrInvalidFilter", err)
	}

	values := make([]any, maxInValues+1)
	for i := range values {
		values[i] = i
	}
	if err := browseWith(t, fake, []model.TableFilter{{Column: "name", Operator: "in", Values: values}}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("oversized IN error = %v, want ErrInvalidFilter", err)
	}
}

func TestFilterValidationErrors(t *testing.T) {
	cases := map[string][]model.TableFilter{
		"unknown column":        {{Column: "nope", Operator: "equals", Value: "x"}},
		"malicious column":      {{Column: `name"; DROP TABLE users`, Operator: "equals", Value: "x"}},
		"unknown operator":      {{Column: "name", Operator: "raw", Value: "x"}},
		"injection operator":    {{Column: "name", Operator: "= 1 OR 1=1", Value: "x"}},
		"missing value":         {{Column: "name", Operator: "equals"}},
		"incompatible contains": {{Column: "id", Operator: "contains", Value: "12"}},
		"null equals":           {{Column: "name", Operator: "equals", Value: nil}},
	}
	for name, filters := range cases {
		t.Run(name, func(t *testing.T) {
			fake := filterFake()
			err := browseWith(t, fake, filters)
			if !errors.Is(err, ErrInvalidFilter) && !errors.Is(err, ErrFilterColumnNotFound) {
				t.Fatalf("error = %v, want ErrInvalidFilter/ErrFilterColumnNotFound", err)
			}
			if fake.sql != "" {
				t.Errorf("no SQL expected on invalid filter, got %q", fake.sql)
			}
		})
	}
}

func TestFilterBigintExactValue(t *testing.T) {
	fake := filterFake()
	if err := browseWith(t, fake, []model.TableFilter{{Column: "id", Operator: "equals", Value: json.Number("9223372036854775807")}}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(fake.args) != 1 || fake.args[0] != "9223372036854775807" {
		t.Errorf("args = %#v, want exact BIGINT string", fake.args)
	}
}

func insertFake(driver model.Driver, quote string) *browseFake {
	return &browseFake{
		driver: driver, quote: quote,
		dbs: []model.Database{{Schemas: []model.Schema{{Name: "public", Tables: []model.Table{{
			Schema: "public", Name: "users", Type: "BASE TABLE",
			Columns: []model.Column{
				{Name: "id", DataType: "bigint", OrdinalPosition: 1, Identity: true},
				{Name: "name", DataType: "text", OrdinalPosition: 2},
				{Name: "nickname", DataType: "text", Nullable: true, OrdinalPosition: 3},
				{Name: "total", DataType: "numeric", OrdinalPosition: 4, Generated: true},
			},
			PrimaryKey: &model.PrimaryKey{Name: "pk", Columns: []string{"id"}},
		}}}}}},
		result: model.QueryResult{RowsAffected: 1, Rows: [][]any{{"1", "Alfie", nil, "10"}}},
	}
}

func insertInto(t *testing.T, fake *browseFake, values map[string]model.InsertValue) error {
	t.Helper()
	m := NewManager(DefaultOptions(), fake)
	_, err := m.InsertRow(context.Background(), "c1", Config{Driver: fake.driver, Database: "alpha"}, InsertRowRequest{
		Schema: "public", Table: "users", Values: values,
	})
	return err
}

func TestInsertRowModesAndCanonicalOrder(t *testing.T) {
	fake := insertFake(model.DriverPostgres, `"`)
	err := insertInto(t, fake, map[string]model.InsertValue{
		"nickname": {Mode: "null"},
		"name":     {Mode: "value", Value: "Alfie"},
		"id":       {Mode: "default"},
	})
	if err != nil {
		t.Fatalf("InsertRow error = %v", err)
	}
	want := `INSERT INTO "public"."users" ("name", "nickname") VALUES ($1, $2) RETURNING "id", "name", "nickname", "total"`
	if fake.sql != want {
		t.Errorf("SQL = %q, want %q", fake.sql, want)
	}
	if len(fake.args) != 2 || fake.args[0] != "Alfie" || fake.args[1] != nil {
		t.Errorf("args = %#v, want [Alfie nil]", fake.args)
	}
}

func TestInsertRowAllDefaultsAndMySQLPlaceholder(t *testing.T) {
	fake := insertFake(model.DriverMySQL, "`")
	fake.result = model.QueryResult{RowsAffected: 1}
	if err := insertInto(t, fake, map[string]model.InsertValue{"id": {Mode: "default"}}); err != nil {
		t.Fatalf("InsertRow error = %v", err)
	}
	if fake.sql != "INSERT INTO `users` () VALUES ()" {
		t.Errorf("SQL = %q, want MySQL default-values form", fake.sql)
	}

	fake = insertFake(model.DriverMySQL, "`")
	fake.result = model.QueryResult{RowsAffected: 1}
	if err := insertInto(t, fake, map[string]model.InsertValue{"name": {Mode: "value", Value: "x"}}); err != nil {
		t.Fatalf("InsertRow error = %v", err)
	}
	if fake.sql != "INSERT INTO `users` (`name`) VALUES (?)" {
		t.Errorf("SQL = %q", fake.sql)
	}
}

func TestInsertRowRejections(t *testing.T) {
	cases := map[string]map[string]model.InsertValue{
		"generated":      {"total": {Mode: "value", Value: "1"}},
		"identity":       {"id": {Mode: "value", Value: "1"}},
		"null not null":  {"name": {Mode: "null"}},
		"unknown mode":   {"name": {Mode: "raw", Value: "x"}},
		"value missing":  {"name": {Mode: "value"}},
		"unknown column": {"nope": {Mode: "value", Value: "x"}},
	}
	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			fake := insertFake(model.DriverPostgres, `"`)
			err := insertInto(t, fake, values)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !errors.Is(err, ErrColumnReadOnly) && !errors.Is(err, ErrInvalidColumnValue) {
				t.Fatalf("error = %v, want read-only/invalid-value", err)
			}
		})
	}
}

func TestInsertRowAllowsNoPKBaseTableAndRejectsView(t *testing.T) {
	noPK := &browseFake{driver: model.DriverSQLite, quote: `"`, dbs: []model.Database{{
		Tables: []model.Table{{Name: "logs", Type: "BASE TABLE", Columns: []model.Column{{Name: "message", DataType: "text", OrdinalPosition: 1}}}},
	}}, result: model.QueryResult{RowsAffected: 1}}
	m := NewManager(DefaultOptions(), noPK)
	if _, err := m.InsertRow(context.Background(), "c1", Config{Driver: model.DriverSQLite}, InsertRowRequest{
		Table: "logs", Values: map[string]model.InsertValue{"message": {Mode: "value", Value: "hi"}},
	}); err != nil {
		t.Fatalf("no-PK insert error = %v", err)
	}

	view := insertFake(model.DriverPostgres, `"`)
	view.dbs[0].Schemas[0].Tables[0].Type = "VIEW"
	if err := insertInto(t, view, map[string]model.InsertValue{"name": {Mode: "value", Value: "x"}}); !errors.Is(err, ErrRowNotMutable) {
		t.Fatalf("view insert error = %v, want ErrRowNotMutable", err)
	}
}
