package database

import (
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// Shared introspection assertion helpers for the (untagged) driver tests.

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
