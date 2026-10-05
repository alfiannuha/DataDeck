package database

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func mutationTable() *model.Table {
	return &model.Table{
		Name: "users",
		Type: "BASE TABLE",
		Columns: []model.Column{
			{Name: "id", DataType: "bigint", OrdinalPosition: 1},
			{Name: "name", DataType: "text", OrdinalPosition: 2, Nullable: true},
			{Name: "total", DataType: "numeric", OrdinalPosition: 3, Generated: true},
			{Name: "seq", DataType: "int", OrdinalPosition: 4, Identity: true},
		},
		PrimaryKey: &model.PrimaryKey{Name: "pk", Columns: []string{"id"}},
	}
}

func TestPlanMutationIdentityCanonicalOrderComposite(t *testing.T) {
	table := mutationTable()
	table.PrimaryKey = &model.PrimaryKey{Name: "pk", Columns: []string{"tenant_id", "user_id"}}
	table.Columns = append([]model.Column{
		{Name: "tenant_id", DataType: "uuid", OrdinalPosition: 0},
		{Name: "user_id", DataType: "bigint", OrdinalPosition: 1},
	}, table.Columns...)

	resolved, err := planMutation(model.DriverPostgres, table, RowMutationRequest{
		Identity: map[string]any{"user_id": json.Number("9223372036854775807"), "tenant_id": "tenant-a"},
	}, false)
	if err != nil {
		t.Fatalf("planMutation error = %v", err)
	}
	if len(resolved.identity) != 2 || resolved.identity[0].column != "tenant_id" || resolved.identity[1].column != "user_id" {
		t.Fatalf("identity order = %+v, want tenant_id then user_id", resolved.identity)
	}
	if resolved.identity[1].value != "9223372036854775807" {
		t.Errorf("bigint identity = %#v, want exact string", resolved.identity[1].value)
	}
}

func TestPlanMutationIdentityErrors(t *testing.T) {
	cases := map[string]map[string]any{
		"missing": {"other": 1},
		"extra":   {"id": 1, "name": "x"},
		"null":    {"id": nil},
	}
	for name, identity := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := planMutation(model.DriverPostgres, mutationTable(), RowMutationRequest{Identity: identity}, false)
			if !errors.Is(err, ErrRowIdentityInvalid) {
				t.Fatalf("error = %v, want ErrRowIdentityInvalid", err)
			}
		})
	}
}

func TestPlanMutationRequiresPrimaryKey(t *testing.T) {
	table := mutationTable()
	table.PrimaryKey = nil
	_, err := planMutation(model.DriverPostgres, table, RowMutationRequest{Identity: map[string]any{"id": 1}}, false)
	if !errors.Is(err, ErrRowIdentityRequired) {
		t.Fatalf("error = %v, want ErrRowIdentityRequired", err)
	}
}

func TestPlanMutationRejectsReadOnlyColumns(t *testing.T) {
	for _, change := range []map[string]any{
		{"id": 2},    // PK edit
		{"total": 1}, // generated
		{"seq": 3},   // identity
	} {
		_, err := planMutation(model.DriverPostgres, mutationTable(), RowMutationRequest{
			Identity: map[string]any{"id": 1}, Changes: change,
		}, true)
		if !errors.Is(err, ErrColumnReadOnly) {
			t.Fatalf("change %v error = %v, want ErrColumnReadOnly", change, err)
		}
	}
}

func TestPlanMutationRejectsNonBaseTable(t *testing.T) {
	table := mutationTable()
	table.Type = "VIEW"
	_, err := planMutation(model.DriverPostgres, table, RowMutationRequest{Identity: map[string]any{"id": 1}}, false)
	if !errors.Is(err, ErrRowNotMutable) {
		t.Fatalf("error = %v, want ErrRowNotMutable", err)
	}
}

func TestPlanMutationExpectedNull(t *testing.T) {
	resolved, err := planMutation(model.DriverPostgres, mutationTable(), RowMutationRequest{
		Identity: map[string]any{"id": 1},
		Expected: map[string]any{"name": nil},
	}, false)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(resolved.expected) != 1 || !resolved.expected[0].isNull {
		t.Fatalf("expected = %+v, want IS NULL predicate", resolved.expected)
	}
}

func TestRowCapabilitiesMetadata(t *testing.T) {
	caps := rowCapabilities(mutationTable())
	if !caps.Insert || !caps.Update || !caps.Delete || !caps.Duplicate {
		t.Errorf("base table caps = %+v, want all true", caps)
	}

	noPK := mutationTable()
	noPK.PrimaryKey = nil
	caps = rowCapabilities(noPK)
	if !caps.Insert || caps.Update || caps.Delete {
		t.Errorf("no-PK caps = %+v, want insert true, update/delete false", caps)
	}

	view := mutationTable()
	view.Type = "VIEW"
	if caps := rowCapabilities(view); caps.Update || caps.Delete {
		t.Errorf("view caps = %+v, want read-only", caps)
	}
}
