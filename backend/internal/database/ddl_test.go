package database

import (
	"errors"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func statements(t *testing.T, driver model.Driver, change StructureChange) []string {
	t.Helper()
	quote := `"`
	if driver == model.DriverMySQL {
		quote = "`"
	}
	out, err := buildStructureStatements(driver, quote, change)
	if err != nil {
		t.Fatalf("buildStructureStatements(%s, %s) error = %v", driver, change.Operation, err)
	}
	return out
}

func TestBuildRenameTable(t *testing.T) {
	pg := statements(t, model.DriverPostgres, StructureChange{
		Operation: OpRenameTable, Schema: "public", Table: "orders", NewName: "orders_v2",
	})
	if len(pg) != 1 || pg[0] != `ALTER TABLE "public"."orders" RENAME TO "orders_v2"` {
		t.Errorf("postgres rename = %v", pg)
	}

	my := statements(t, model.DriverMySQL, StructureChange{
		Operation: OpRenameTable, Schema: "shop", Table: "orders", NewName: "orders_v2",
	})
	if len(my) != 1 || my[0] != "RENAME TABLE `shop`.`orders` TO `shop`.`orders_v2`" {
		t.Errorf("mysql rename = %v", my)
	}

	sq := statements(t, model.DriverSQLite, StructureChange{
		Operation: OpRenameTable, Table: "orders", NewName: "orders_v2",
	})
	if len(sq) != 1 || sq[0] != `ALTER TABLE "orders" RENAME TO "orders_v2"` {
		t.Errorf("sqlite rename = %v", sq)
	}
}

func TestBuildAddColumn(t *testing.T) {
	def := &ColumnDefinition{Name: "status", Type: "varchar(32)", Nullable: false, Default: strptr("active")}
	got := statements(t, model.DriverPostgres, StructureChange{
		Operation: OpAddColumn, Schema: "public", Table: "orders", Column: def,
	})
	want := `ALTER TABLE "public"."orders" ADD COLUMN "status" varchar(32) NOT NULL DEFAULT 'active'`
	if len(got) != 1 || got[0] != want {
		t.Errorf("add column = %v, want %q", got, want)
	}

	// Expression default is emitted verbatim; literal escapes single quotes.
	expr := statements(t, model.DriverPostgres, StructureChange{
		Operation: OpAddColumn, Table: "t",
		Column: &ColumnDefinition{Name: "created", Type: "timestamp", Nullable: true,
			Default: strptr("now()"), IsExpression: true},
	})
	if !strings.Contains(expr[0], "DEFAULT now()") {
		t.Errorf("expression default = %q", expr[0])
	}
	lit := statements(t, model.DriverPostgres, StructureChange{
		Operation: OpAddColumn, Table: "t",
		Column: &ColumnDefinition{Name: "note", Type: "text", Nullable: true, Default: strptr("it's ok")},
	})
	if !strings.Contains(lit[0], `DEFAULT 'it''s ok'`) {
		t.Errorf("literal default not escaped: %q", lit[0])
	}
}

func TestBuildAlterColumnPostgresMultiStatement(t *testing.T) {
	got := statements(t, model.DriverPostgres, StructureChange{
		Operation: OpAlterColumn, Schema: "public", Table: "orders", ColumnName: "status",
		DefaultAction: "drop",
		NewColumn:     &ColumnDefinition{Name: "state", Type: "text", Nullable: true},
	})
	joined := strings.Join(got, "; ")
	for _, want := range []string{
		`ALTER TABLE "public"."orders" ALTER COLUMN "status" TYPE text`,
		`ALTER TABLE "public"."orders" ALTER COLUMN "status" DROP DEFAULT`,
		`ALTER TABLE "public"."orders" ALTER COLUMN "status" DROP NOT NULL`,
		`ALTER TABLE "public"."orders" RENAME COLUMN "status" TO "state"`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
}

func TestBuildAlterColumnMySQLUsesModify(t *testing.T) {
	got := statements(t, model.DriverMySQL, StructureChange{
		Operation: OpAlterColumn, Schema: "shop", Table: "orders", ColumnName: "status",
		NewColumn: &ColumnDefinition{Name: "status", Type: "varchar(64)", Nullable: true},
	})
	want := "ALTER TABLE `shop`.`orders` MODIFY COLUMN `status` varchar(64)"
	if len(got) != 1 || got[0] != want {
		t.Errorf("mysql alter = %v, want %q", got, want)
	}
}

func TestBuildCreateIndex(t *testing.T) {
	got := statements(t, model.DriverPostgres, StructureChange{
		Operation: OpCreateIndex, Schema: "public", Table: "orders",
		Index: &IndexDefinition{Name: "idx_orders_created", Unique: true, Method: "btree",
			Columns: []IndexColumn{{Name: "created_at", Direction: "desc"}, {Name: "id"}},
			Where:   "status = 'open'"},
	})
	want := `CREATE UNIQUE INDEX "idx_orders_created" ON "public"."orders" USING btree ("created_at" DESC, "id") WHERE status = 'open'`
	if len(got) != 1 || got[0] != want {
		t.Errorf("create index = %v, want %q", got, want)
	}
}

func TestBuildDropIndex(t *testing.T) {
	pg := statements(t, model.DriverPostgres, StructureChange{
		Operation: OpDropIndex, Schema: "public", IndexName: "idx_a",
	})
	if pg[0] != `DROP INDEX "public"."idx_a"` {
		t.Errorf("postgres drop index = %q", pg[0])
	}
	my := statements(t, model.DriverMySQL, StructureChange{
		Operation: OpDropIndex, Schema: "shop", Table: "orders", IndexName: "idx_a",
	})
	if my[0] != "DROP INDEX `idx_a` ON `shop`.`orders`" {
		t.Errorf("mysql drop index = %q", my[0])
	}
}

func TestBuildAddForeignKey(t *testing.T) {
	got := statements(t, model.DriverPostgres, StructureChange{
		Operation: OpAddConstraint, Schema: "public", Table: "orders",
		Constraint: &ConstraintDefinition{
			Name: "fk_orders_customer", Type: "FOREIGN KEY", Columns: []string{"customer_id"},
			Reference: &ReferenceDefinition{Table: "customers", Columns: []string{"id"},
				OnDelete: "cascade", OnUpdate: "restrict"},
		},
	})
	want := `ALTER TABLE "public"."orders" ADD CONSTRAINT "fk_orders_customer" FOREIGN KEY ("customer_id") REFERENCES "customers" ("id") ON UPDATE RESTRICT ON DELETE CASCADE`
	if len(got) != 1 || got[0] != want {
		t.Errorf("add fk = %v, want %q", got, want)
	}
}

func TestBuildAddCheckAndPrimaryKey(t *testing.T) {
	check := statements(t, model.DriverPostgres, StructureChange{
		Operation: OpAddConstraint, Table: "orders",
		Constraint: &ConstraintDefinition{Name: "ck_qty", Type: "CHECK", Expression: "qty > 0"},
	})
	if check[0] != `ALTER TABLE "orders" ADD CONSTRAINT "ck_qty" CHECK (qty > 0)` {
		t.Errorf("check = %q", check[0])
	}
	pk := statements(t, model.DriverPostgres, StructureChange{
		Operation: OpAddConstraint, Table: "orders",
		Constraint: &ConstraintDefinition{Type: "PRIMARY KEY", Columns: []string{"id"}},
	})
	if pk[0] != `ALTER TABLE "orders" ADD PRIMARY KEY ("id")` {
		t.Errorf("pk = %q", pk[0])
	}
}

func TestBuildDropConstraintMySQLByType(t *testing.T) {
	fk := statements(t, model.DriverMySQL, StructureChange{
		Operation: OpDropConstraint, Schema: "shop", Table: "orders",
		ConstraintName: "fk_orders_customer", ConstraintType: "FOREIGN KEY",
	})
	if fk[0] != "ALTER TABLE `shop`.`orders` DROP FOREIGN KEY `fk_orders_customer`" {
		t.Errorf("mysql drop fk = %q", fk[0])
	}
	pk := statements(t, model.DriverMySQL, StructureChange{
		Operation: OpDropConstraint, Schema: "shop", Table: "orders", ConstraintType: "PRIMARY KEY",
		ConstraintName: "PRIMARY",
	})
	if pk[0] != "ALTER TABLE `shop`.`orders` DROP PRIMARY KEY" {
		t.Errorf("mysql drop pk = %q", pk[0])
	}
}

func TestBuildRejectsUnsafeOrUnsupportedChanges(t *testing.T) {
	cases := []struct {
		name    string
		driver  model.Driver
		change  StructureChange
		wantErr error
	}{
		{"semicolon in type", model.DriverPostgres, StructureChange{
			Operation: OpAddColumn, Table: "t",
			Column: &ColumnDefinition{Name: "c", Type: "text; DROP TABLE t"},
		}, ErrInvalidStructureChange},
		{"comment in where", model.DriverPostgres, StructureChange{
			Operation: OpCreateIndex, Table: "t",
			Index: &IndexDefinition{Name: "i", Columns: []IndexColumn{{Name: "c"}}, Where: "true -- x"},
		}, ErrInvalidStructureChange},
		{"fk column count mismatch", model.DriverPostgres, StructureChange{
			Operation: OpAddConstraint, Table: "t",
			Constraint: &ConstraintDefinition{Type: "FOREIGN KEY", Columns: []string{"a", "b"},
				Reference: &ReferenceDefinition{Table: "r", Columns: []string{"id"}}},
		}, ErrInvalidStructureChange},
		{"sqlite alter unsupported", model.DriverSQLite, StructureChange{
			Operation: OpAlterColumn, Table: "t", ColumnName: "c",
			NewColumn: &ColumnDefinition{Name: "c", Type: "text", Nullable: true},
		}, ErrNotImplemented},
		{"sqlite add constraint unsupported", model.DriverSQLite, StructureChange{
			Operation: OpAddConstraint, Table: "t",
			Constraint: &ConstraintDefinition{Type: "UNIQUE", Columns: []string{"a"}},
		}, ErrNotImplemented},
		{"mysql partial index unsupported", model.DriverMySQL, StructureChange{
			Operation: OpCreateIndex, Table: "t",
			Index: &IndexDefinition{Name: "i", Columns: []IndexColumn{{Name: "c"}}, Where: "c > 0"},
		}, ErrInvalidStructureChange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quote := `"`
			if tc.driver == model.DriverMySQL {
				quote = "`"
			}
			_, err := buildStructureStatements(tc.driver, quote, tc.change)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestStructureCapabilitiesMatrix(t *testing.T) {
	if caps := StructureCapabilitiesFor(model.DriverSQLite); caps.AlterColumn || caps.AddConstraint || caps.DropConstraint {
		t.Errorf("sqlite caps = %+v, want alter/constraint disabled", caps)
	}
	if caps := StructureCapabilitiesFor(model.DriverMySQL); caps.PartialIndex || caps.IndexMethod {
		t.Errorf("mysql caps = %+v, want partial/method disabled", caps)
	}
	if caps := StructureCapabilitiesFor(model.DriverPostgres); !caps.AlterColumn || !caps.PartialIndex || !caps.IndexMethod {
		t.Errorf("postgres caps = %+v, want full support", caps)
	}
}

func strptr(s string) *string { return &s }
