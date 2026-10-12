package database

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/tableimport"
)

func importColumns() []model.Column {
	return []model.Column{
		{Name: "id", DataType: "bigint", Nullable: false, OrdinalPosition: 1},
		{Name: "name", DataType: "text", Nullable: true, OrdinalPosition: 2},
		{Name: "total", DataType: "numeric", Nullable: true, OrdinalPosition: 3, Generated: true},
		{Name: "active", DataType: "boolean", Nullable: false, OrdinalPosition: 4},
	}
}

func TestResolveMappingExplicit(t *testing.T) {
	plan, err := resolveImportPlan([]model.Database{{
		Schemas: []model.Schema{{Name: "public", Tables: []model.Table{{
			Schema: "public", Name: "users", Type: "BASE TABLE",
			Columns: importColumns(),
		}}}},
	}}, model.DriverPostgres, ImportPlanRequest{
		Schema: "public", Table: "users",
		SourceColumns: []string{"A", "B", "C"},
		Mapping: []model.ImportMapping{
			{Source: 2, Target: "active"},
			{Source: 0, Target: "id"},
			{Source: 1, Target: "name"},
		},
	})
	if err != nil {
		t.Fatalf("resolveImportPlan error = %v", err)
	}
	want := []model.ImportMappingResolved{
		{Source: 2, Target: "active"},
		{Source: 0, Target: "id"},
		{Source: 1, Target: "name"},
	}
	if !reflect.DeepEqual(plan.Mapping, want) {
		t.Fatalf("mapping = %#v, want %#v", plan.Mapping, want)
	}
	// Generated/identity columns are excluded from the insertable set.
	for _, name := range []string{"total"} {
		if canonicalColumn(plan.Columns, name) != nil {
			t.Errorf("generated column %q must not be insertable", name)
		}
	}
}

func TestResolveMappingAutoByName(t *testing.T) {
	plan, err := resolveImportPlan([]model.Database{{
		Schemas: []model.Schema{{Name: "public", Tables: []model.Table{{
			Schema: "public", Name: "users", Type: "BASE TABLE",
			Columns: importColumns(),
		}}}},
	}}, model.DriverPostgres, ImportPlanRequest{
		Schema:        "public",
		Table:         "users",
		SourceColumns: []string{"name", "id", "unknown"},
	})
	if err != nil {
		t.Fatalf("resolveImportPlan error = %v", err)
	}
	want := []model.ImportMappingResolved{
		{Source: 0, Target: "name"},
		{Source: 1, Target: "id"},
	}
	if !reflect.DeepEqual(plan.Mapping, want) {
		t.Fatalf("mapping = %#v, want %#v", plan.Mapping, want)
	}
}

func TestResolveMappingErrors(t *testing.T) {
	table := model.Table{Schema: "public", Name: "users", Type: "BASE TABLE", Columns: importColumns()}
	base := ImportPlanRequest{
		Schema: "public", Table: "users", SourceColumns: []string{"a", "b"},
	}
	errorCases := []struct {
		name    string
		mutate  func(*ImportPlanRequest)
		wantErr error
	}{
		{"out of range source", func(r *ImportPlanRequest) { r.Mapping = []model.ImportMapping{{Source: 9, Target: "id"}} }, ErrImportMappingInvalid},
		{"unknown target", func(r *ImportPlanRequest) { r.Mapping = []model.ImportMapping{{Source: 0, Target: "nope"}} }, ErrImportMappingInvalid},
		{"generated target", func(r *ImportPlanRequest) { r.Mapping = []model.ImportMapping{{Source: 0, Target: "total"}} }, ErrImportMappingInvalid},
		{"duplicate source", func(r *ImportPlanRequest) {
			r.Mapping = []model.ImportMapping{{Source: 0, Target: "id"}, {Source: 0, Target: "name"}}
		}, ErrImportMappingInvalid},
		{"no mapping match", func(r *ImportPlanRequest) { r.SourceColumns = []string{"zzz"} }, ErrImportMappingInvalid},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.mutate(&req)
			_, err := resolveImportPlan([]model.Database{{Schemas: []model.Schema{{Name: "public", Tables: []model.Table{table}}}}}, model.DriverPostgres, req)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}

	view := model.Table{Schema: "public", Name: "v", Type: "VIEW", Columns: importColumns()}
	_, err := resolveImportPlan([]model.Database{{Schemas: []model.Schema{{Name: "public", Tables: []model.Table{view}}}}}, model.DriverPostgres, ImportPlanRequest{
		Schema: "public", Table: "v", SourceColumns: []string{"id"},
	})
	if !errors.Is(err, ErrRowNotMutable) {
		t.Fatalf("view target error = %v, want ErrRowNotMutable", err)
	}
}

func TestResolveMappingImpliedSourceColumns(t *testing.T) {
	plan, err := resolveImportPlan([]model.Database{{Schemas: []model.Schema{{Name: "public", Tables: []model.Table{{
		Schema: "public", Name: "users", Type: "BASE TABLE", Columns: importColumns(),
	}}}}}}, model.DriverPostgres, ImportPlanRequest{Schema: "public", Table: "users"})
	if err != nil {
		t.Fatalf("resolveImportPlan error = %v", err)
	}
	// Empty source columns (SH column-implied INSERT) resolve to the insertable
	// columns in ordinal order, auto-mapped by name.
	want := []model.ImportMappingResolved{
		{Source: 0, Target: "id"},
		{Source: 1, Target: "name"},
		{Source: 2, Target: "active"},
	}
	if !reflect.DeepEqual(plan.Mapping, want) {
		t.Fatalf("mapping = %#v, want %#v", plan.Mapping, want)
	}
	if plan.SourceCount != 3 {
		t.Fatalf("source count = %d, want 3", plan.SourceCount)
	}
}

func TestImportCellValue(t *testing.T) {
	bigint := model.Column{Name: "id", DataType: "bigint", Nullable: false}
	real := model.Column{Name: "r", DataType: "numeric", Nullable: true}
	boolean := model.Column{Name: "a", DataType: "boolean", Nullable: true}
	jsonb := model.Column{Name: "j", DataType: "jsonb", Nullable: true}
	text := model.Column{Name: "t", DataType: "text", Nullable: true}

	cases := []struct {
		name    string
		column  model.Column
		cell    model.ImportCell
		want    any
		wantErr bool
	}{
		{"exact bigint string", bigint, model.ImportCell{Value: "9223372036854775807"}, "9223372036854775807", false},
		{"numeric decimal", real, model.ImportCell{Value: "3.14"}, "3.14", false},
		{"numeric exponent", real, model.ImportCell{Value: "1e10"}, "1e10", false},
		{"numeric garbage", real, model.ImportCell{Value: "abc"}, nil, true},
		{"numeric empty", real, model.ImportCell{Value: ""}, nil, true},
		{"null allowed", text, model.ImportCell{IsNull: true}, nil, false},
		{"null on NOT NULL", bigint, model.ImportCell{IsNull: true}, nil, true},
		{"boolean true", boolean, model.ImportCell{Value: "true"}, true, false},
		{"boolean 1", boolean, model.ImportCell{Value: "1"}, true, false},
		{"boolean bad", boolean, model.ImportCell{Value: "maybe"}, nil, true},
		{"json valid", jsonb, model.ImportCell{Value: `{"a":1}`}, `{"a":1}`, false},
		{"json invalid", jsonb, model.ImportCell{Value: `{oops`}, nil, true},
		{"text passthrough", text, model.ImportCell{Value: "anything"}, "anything", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := importCellValue(tc.column, tc.cell)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("importCellValue = %v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("importCellValue error = %v", err)
			}
			if got != tc.want && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("importCellValue = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestImportCellsToArgs(t *testing.T) {
	plan := &model.ImportPlan{
		SourceCount: 3,
		Columns: []model.Column{
			{Name: "id", DataType: "bigint", Nullable: false, OrdinalPosition: 1},
			{Name: "name", DataType: "text", Nullable: true, OrdinalPosition: 2},
		},
		Mapping: []model.ImportMappingResolved{
			{Source: 2, Target: "id"},
			{Source: 0, Target: "name"},
		},
	}
	args, err := importCellsToArgs(plan, []model.ImportCell{
		{Value: "Bob"},
		{IsNull: true}, // unmapped source → dropped
		{Value: "42"},
	})
	if err != nil {
		t.Fatalf("importCellsToArgs error = %v", err)
	}
	if want := []any{"42", "Bob"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}

	bad := []model.ImportCell{{Value: "x"}, {Value: "y"}, {Value: "z"}}
	if _, err := importCellsToArgs(plan, bad); err == nil {
		t.Fatal("expected numeric conversion error for id='z'")
	}

	if _, err := importCellsToArgs(plan, []model.ImportCell{{Value: "1"}, {Value: "2"}}); err == nil {
		t.Fatal("expected width error for a 2-value row on a 3-column source")
	}
}

// TestImportBatchSQLGeneration verifies the generated multi-row INSERT text.
func TestImportBatchSQLGeneration(t *testing.T) {
	pg := importBatchSQL("postgres", `"public"."users"`, []string{`"id"`, `"name"`}, 2)
	if want := `INSERT INTO "public"."users" ("id", "name") VALUES ($1, $2), ($3, $4)`; pg != want {
		t.Errorf("pg SQL = %q, want %q", pg, want)
	}
	my := importBatchSQL("mysql", "users", []string{"`id`", "`name`"}, 3)
	if want := "INSERT INTO users (`id`, `name`) VALUES (?, ?), (?, ?), (?, ?)"; my != want {
		t.Errorf("mysql SQL = %q, want %q", my, want)
	}
}

// TestSQLiteImportRoundTrip runs the export → import round-trip gate on a real
// SQLite file: insert via the SQL path of the tableimport parser (the same
// restricted-INSERT grammar users upload) and verify rows come back identical.
func TestSQLiteImportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "import.db")
	manager := NewManager(DefaultOptions(), SQLite{})
	cfg := Config{Driver: model.DriverSQLite, Database: path}

	ctx := context.Background()
	run := func(sql string) (model.QueryResult, error) {
		return manager.Execute(ctx, "s1", cfg, sql)
	}
	if _, err := run(`CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		big TEXT NOT NULL,
		active BOOLEAN,
		joined TEXT
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	script := `-- data for "users"
INSERT INTO "users" (id, big, active, joined) VALUES
  (1, '9223372036854775807', TRUE, '2024-01-02'),
  (2, '42', FALSE, NULL),
  (3, '7', NULL, '');
`
	var rows [][]model.ImportCell
	_, _, n, err := tableimport.ParseSQLInserts(strings.NewReader(script), tableimport.SQLOptions{Dialect: "sqlite"}, func(row []model.ImportCell) error {
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		t.Fatalf("ParseSQLInserts: %v", err)
	}

	plan, err := manager.ResolveImportPlan(ctx, "s1", cfg, ImportPlanRequest{
		Table:         "users",
		SourceColumns: []string{"id", "big", "active", "joined"},
	})
	if err != nil {
		t.Fatalf("ResolveImportPlan: %v", err)
	}
	findings, count, err := manager.ValidateImportRows(ctx, plan, 10, func(emit func([]model.ImportCell) error) error {
		for i := 0; i < n; i++ {
			if err := emit(rows[i]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ValidateImportRows: %v", err)
	}
	if count != 3 || len(findings) != 0 {
		t.Fatalf("validation: count=%d findings=%v", count, findings)
	}

	result, err := manager.ExecuteImportRows(ctx, "s1", cfg, ImportPlanRequest{
		Table:         "users",
		SourceColumns: []string{"id", "big", "active", "joined"},
	}, func(emit func([]model.ImportCell) error) error {
		for i := 0; i < n; i++ {
			if err := emit(rows[i]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteImportRows: %v", err)
	}
	if result.Inserted != 3 || result.RowCount != 3 {
		t.Fatalf("result = %+v", result)
	}

	read, err := run(`SELECT id, big, active, joined FROM users ORDER BY id`)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := [][]any{
		{int64(1), "9223372036854775807", int64(1), "2024-01-02"},
		{int64(2), "42", int64(0), nil},
		{int64(3), "7", nil, ""},
	}
	if !reflect.DeepEqual(read.Rows, want) {
		t.Fatalf("round-trip rows = %#v, want %#v", read.Rows, want)
	}
}

// TestSQLiteImportAllOrNothing verifies a mid-file constraint failure rolls back
// every inserted row (no partial commit).
func TestSQLiteImportAllOrNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.db")
	manager := NewManager(DefaultOptions(), SQLite{})
	cfg := Config{Driver: model.DriverSQLite, Database: path}

	ctx := context.Background()
	if _, err := manager.Execute(ctx, "s1", cfg, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT UNIQUE)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	rows := [][]model.ImportCell{
		{{Value: "1"}, {Value: "a"}},
		{{Value: "2"}, {Value: "b"}},
		{{Value: "3"}, {Value: "a"}}, // duplicate unique value
		{{Value: "4"}, {Value: "c"}},
	}
	runImport := func() error {
		_, err := manager.ExecuteImportRows(ctx, "s1", cfg, ImportPlanRequest{
			Table:         "t",
			SourceColumns: []string{"id", "v"},
		}, func(emit func([]model.ImportCell) error) error {
			for i := range rows {
				if err := emit(rows[i]); err != nil {
					return err
				}
			}
			return nil
		})
		return err
	}
	err := runImport()
	if err == nil {
		t.Fatal("expected unique-constraint failure on import")
	}
	if !errors.Is(err, ErrConstraintViolation) {
		t.Fatalf("error = %v, want ErrConstraintViolation", err)
	}
	read, err := manager.Execute(ctx, "s1", cfg, `SELECT count(*) FROM t`)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count := read.Rows[0][0]; count != int64(0) {
		t.Fatalf("rows after rollback = %v, want 0", count)
	}
}
