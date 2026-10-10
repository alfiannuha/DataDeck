package database

import (
	"errors"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestInsertableColumns(t *testing.T) {
	columns := []model.Column{
		{Name: "id", DataType: "bigint", OrdinalPosition: 1, Identity: true},
		{Name: "name", DataType: "text", OrdinalPosition: 2},
		{Name: "summary", DataType: "text", OrdinalPosition: 3, Generated: true},
		{Name: "created_at", DataType: "timestamptz", OrdinalPosition: 4},
	}
	got, err := insertableColumns(columns)
	if err != nil {
		t.Fatalf("insertableColumns: %v", err)
	}
	var names []string
	for _, column := range got {
		names = append(names, column.Name)
	}
	if strings.Join(names, ",") != "name,created_at" {
		t.Errorf("insertable columns = %v, want name,created_at", names)
	}

	if _, err := insertableColumns([]model.Column{
		{Name: "id", DataType: "bigint", Identity: true},
	}); !errors.Is(err, ErrNoExportableColumns) {
		t.Errorf("err = %v, want ErrNoExportableColumns", err)
	}
	if _, err := insertableColumns(nil); !errors.Is(err, ErrNoExportableColumns) {
		t.Errorf("empty columns err = %v, want ErrNoExportableColumns", err)
	}
}

func TestTableWhereAndOrder(t *testing.T) {
	columns := []model.Column{
		{Name: "id", DataType: "int", OrdinalPosition: 1},
		{Name: "created_at", DataType: "timestamp", OrdinalPosition: 2},
		{Name: "status", DataType: "text", OrdinalPosition: 3},
	}
	table := &model.Table{
		Schema: "public", Name: "t", Type: "BASE TABLE", Columns: columns,
		PrimaryKey: &model.PrimaryKey{Columns: []string{"id"}},
	}

	where, args, order, err := tableWhereAndOrder(`"`, "postgres", table, columns,
		&model.TableSort{Column: "status", Direction: "desc"},
		[]model.TableFilter{{Column: "status", Operator: "equals", Value: "active"}})
	if err != nil {
		t.Fatalf("tableWhereAndOrder: %v", err)
	}
	if !strings.HasPrefix(where, " WHERE ") || !strings.Contains(where, `"status"`) {
		t.Errorf("where = %q, want parameterized status filter", where)
	}
	if len(args) != 1 || args[0] != "active" {
		t.Errorf("args = %v, want [active]", args)
	}
	if !strings.Contains(order, `"status" DESC`) || !strings.Contains(order, `"id" ASC`) {
		t.Errorf("order = %q, want status DESC + pk tiebreak", order)
	}

	// No sort/filters → no clauses at all.
	where, args, order, err = tableWhereAndOrder("`", "mysql", table, columns, nil, nil)
	if err != nil {
		t.Fatalf("no-op tableWhereAndOrder: %v", err)
	}
	if where != "" || args != nil || order != "" {
		t.Errorf("no-op clauses = (%q, %v, %q), want empty", where, args, order)
	}

	// Unknown sort column surfaces ErrSortColumnNotFound.
	if _, _, _, err := tableWhereAndOrder(`"`, "postgres", table, columns,
		&model.TableSort{Column: "nope", Direction: "asc"}, nil); !errors.Is(err, ErrSortColumnNotFound) {
		t.Errorf("unknown sort = %v, want ErrSortColumnNotFound", err)
	}
}

func TestSortedColumnsCopy(t *testing.T) {
	columns := []model.Column{
		{Name: "b", OrdinalPosition: 2},
		{Name: "a", OrdinalPosition: 1},
	}
	got := sortedColumns(columns)
	if got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("sortedColumns = %+v, want a,b", got)
	}
	// Must not mutate the source slice.
	if columns[0].Name != "b" {
		t.Errorf("source was mutated: %+v", columns)
	}
}