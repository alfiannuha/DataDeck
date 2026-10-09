//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// TestPostgresTableStructureIntegration creates a deterministic schema and
// verifies the canonical TableStructure output for Postgres, including
// composite keys, named foreign keys with actions, CHECK/UNIQUE definitions,
// generated columns, and quoted identifiers.
func TestPostgresTableStructureIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_PG_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping Postgres structure integration test")
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
	db, err := manager.Open(ctx, "itest-structure-pg", cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	for _, stmt := range []string{
		`CREATE SCHEMA IF NOT EXISTS datadeck_sitest`,
		`CREATE TABLE datadeck_sitest.orders (
			id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			reference text NOT NULL,
			amount numeric(12,2) NOT NULL DEFAULT 0,
			status text NOT NULL DEFAULT 'pending',
			CONSTRAINT orders_status_check CHECK (status IN ('pending', 'shipped', 'cancelled')),
			CONSTRAINT orders_uk_reference UNIQUE (reference)
		)`,
		`CREATE TABLE datadeck_sitest.order_items (
			order_id bigint NOT NULL,
			sku text NOT NULL,
			qty int NOT NULL,
			CONSTRAINT order_items_pk PRIMARY KEY (order_id, sku),
			CONSTRAINT fk_items_order FOREIGN KEY (order_id) REFERENCES datadeck_sitest.orders(id) ON DELETE CASCADE ON UPDATE RESTRICT
		)`,
		`CREATE VIEW datadeck_sitest.open_orders AS SELECT id, reference FROM datadeck_sitest.orders WHERE status = 'pending'`,
		`CREATE TABLE "datadeck_sitest"."odd table" (
			id bigint PRIMARY KEY,
			"Ünïcode" text
		)`,
		`CREATE INDEX idx_orders_status ON datadeck_sitest.orders(status)`,
		`CREATE UNIQUE INDEX idx_order_items_unique_sku ON datadeck_sitest.order_items(sku)`,
		`CREATE INDEX idx_orders_lower_ref ON datadeck_sitest.orders((lower(reference)))`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS datadeck_sitest CASCADE`)
	}()

	orders, err := manager.TableStructure(ctx, "itest-structure-pg", cfg, "datadeck_sitest", "orders")
	if err != nil {
		t.Fatalf("TableStructure(orders) error = %v", err)
	}
	if orders.Schema != "datadeck_sitest" || orders.Table != "orders" || orders.ObjectType != "BASE TABLE" {
		t.Errorf("orders identity = %+v", orders)
	}

	// Columns: ordinal order, types, defaults, flags.
	col := findStructureColumn(t, orders, "id")
	if col.NormalizedType != "bigint" || col.DatabaseType == "" || col.Nullable {
		t.Errorf("id column = %+v", col)
	}
	if !col.Identity || col.Generated {
		t.Errorf("identity/generated flags = %+v", col)
	}
	if !col.PrimaryKey || col.PrimaryKeyPosition != 1 {
		t.Errorf("id pk flags = %+v", col)
	}
	col = findStructureColumn(t, orders, "amount")
	if col.NormalizedType != "numeric" || col.Precision != 12 || col.Scale != 2 {
		t.Errorf("amount column = %+v", col)
	}
	if col.Default == nil || *col.Default != "0" {
		t.Errorf("amount default = %v", col.Default)
	}
	col = findStructureColumn(t, orders, "reference")
	if col.PrimaryKey || col.Identity {
		t.Errorf("reference pk flags = %+v", col)
	}

	// Insertable/updatable consistency with T07 rules.
	idCol := findStructureColumn(t, orders, "id")
	if idCol.Insertable || idCol.Updatable {
		t.Error("identity pk column must not be insertable/updatable")
	}
	refCol := findStructureColumn(t, orders, "reference")
	if !refCol.Insertable || !refCol.Updatable {
		t.Error("reference must be insertable and updatable")
	}
	amountCol := findStructureColumn(t, orders, "amount")
	if !amountCol.Insertable || !amountCol.Updatable {
		t.Error("amount must be insertable and updatable")
	}

	// PK + constraints.
	if orders.PrimaryKey == nil || !equalStrings(orders.PrimaryKey.Columns, []string{"id"}) {
		t.Errorf("orders primary key = %+v", orders.PrimaryKey)
	}
	if findStructureConstraint(t, orders, "PRIMARY KEY", nil) == nil {
		t.Error("missing PRIMARY KEY constraint entry")
	}
	check := findStructureConstraint(t, orders, "CHECK", nil)
	if check == nil || check.Name != "orders_status_check" || check.Definition == "" {
		t.Errorf("CHECK constraint = %+v", check)
	}
	uq := findStructureConstraint(t, orders, "UNIQUE", []string{"reference"})
	if uq == nil || uq.Name != "orders_uk_reference" || uq.Definition == "" {
		t.Errorf("UNIQUE constraint = %+v", uq)
	}

	// Indexes: user index, unique index, expression index (empty column part).
	if idx := findStructureIndexByName(orders.Indexes, "idx_orders_status"); idx == nil || idx.Unique {
		t.Errorf("idx_orders_status = %+v", idx)
	}
	if idx := findStructureIndexByName(orders.Indexes, "idx_orders_lower_ref"); idx == nil || len(idx.Columns) != 1 || idx.Columns[0] != "" {
		t.Errorf("expression index = %+v (parts %v)", idx, idx.Columns)
	}

	items, err := manager.TableStructure(ctx, "itest-structure-pg", cfg, "datadeck_sitest", "order_items")
	if err != nil {
		t.Fatalf("TableStructure(order_items) error = %v", err)
	}
	if items.PrimaryKey == nil || !equalStrings(items.PrimaryKey.Columns, []string{"order_id", "sku"}) {
		t.Errorf("order_items primary key = %+v", items.PrimaryKey)
	}
	if len(items.ForeignKeys) != 1 {
		t.Fatalf("order_items FKs = %d, want 1", len(items.ForeignKeys))
	}
	fk := items.ForeignKeys[0]
	if fk.Name != "fk_items_order" || !equalStrings(fk.Columns, []string{"order_id"}) ||
		!equalStrings(fk.ReferencedColumns, []string{"id"}) ||
		fk.ReferencedTable != "orders" || fk.ReferencedSchema != "datadeck_sitest" {
		t.Errorf("items FK = %+v", fk)
	}
	if fk.OnDelete != "CASCADE" || fk.OnUpdate != "RESTRICT" {
		t.Errorf("items FK actions = update %q delete %q", fk.OnUpdate, fk.OnDelete)
	}
	// The inline REFERENCES also appears as a constraint entry.
	if findStructureConstraint(t, items, "FOREIGN KEY", []string{"order_id"}) == nil {
		t.Error("missing FOREIGN KEY constraint entry for order_items")
	}

	// Views report read-only columns and object_type.
	view, err := manager.TableStructure(ctx, "itest-structure-pg", cfg, "datadeck_sitest", "open_orders")
	if err != nil {
		t.Fatalf("TableStructure(open_orders) error = %v", err)
	}
	if view.ObjectType != "VIEW" {
		t.Errorf("open_orders object_type = %q", view.ObjectType)
	}
	for _, c := range view.Columns {
		if c.Insertable || c.Updatable {
			t.Errorf("view column %q must be read-only: %+v", c.Name, c)
		}
	}

	// Quoted identifiers with spaces and non-ASCII letters.
	odd, err := manager.TableStructure(ctx, "itest-structure-pg", cfg, "datadeck_sitest", "odd table")
	if err != nil {
		t.Fatalf("TableStructure(odd table) error = %v", err)
	}
	findStructureColumn(t, odd, "Ünïcode")

	// Missing relation must surface ErrTableNotFound.
	if _, err := manager.TableStructure(ctx, "itest-structure-pg", cfg, "datadeck_sitest", "no_such_table"); err == nil {
		t.Fatal("expected error for missing table")
	}
}

// TestMySQLTableStructureIntegration verifies the canonical structure output
// for MySQL, including named composite foreign keys, CHECK/UNIQUE constraints
// (when the server supports them), and the auto-increment identity flag.
func TestMySQLTableStructureIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_MYSQL_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_MYSQL_HOST not set; skipping MySQL structure integration test")
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
	db, err := manager.Open(ctx, "itest-structure-my", cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	for _, stmt := range []string{
		`DROP TABLE IF EXISTS s_order_items`,
		`DROP TABLE IF EXISTS s_orders`,
		`CREATE TABLE s_orders (
			id BIGINT NOT NULL AUTO_INCREMENT,
			reference VARCHAR(64) NOT NULL,
			amount DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			status VARCHAR(16) NOT NULL DEFAULT 'pending',
			CONSTRAINT s_orders_status_check CHECK (status IN ('pending', 'shipped', 'cancelled')),
			CONSTRAINT s_orders_uk_reference UNIQUE (reference),
			PRIMARY KEY (id)
		) ENGINE=InnoDB`,
		`CREATE TABLE s_order_items (
			order_id BIGINT NOT NULL,
			sku VARCHAR(32) NOT NULL,
			qty INT NOT NULL,
			PRIMARY KEY (order_id, sku),
			CONSTRAINT s_fk_items_order FOREIGN KEY (order_id) REFERENCES s_orders(id) ON DELETE CASCADE ON UPDATE RESTRICT
		) ENGINE=InnoDB`,
		`CREATE INDEX idx_s_orders_status ON s_orders(status)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	supportsCheck := mysqlSupportsCheck(t, ctx, db)

	orders, err := manager.TableStructure(ctx, "itest-structure-my", cfg, "", "s_orders")
	if err != nil {
		t.Fatalf("TableStructure(s_orders) error = %v", err)
	}
	if orders.Schema != "" || orders.Table != "s_orders" || orders.ObjectType != "BASE TABLE" {
		t.Errorf("s_orders identity = %+v", orders)
	}

	idCol := findStructureColumn(t, orders, "id")
	if !idCol.Identity || !idCol.PrimaryKey || idCol.Insertable || idCol.Updatable {
		t.Errorf("auto-increment id flags = %+v", idCol)
	}
	amountCol := findStructureColumn(t, orders, "amount")
	if amountCol.NormalizedType != "numeric" || !amountCol.Insertable || !amountCol.Updatable {
		t.Errorf("amount column = %+v", amountCol)
	}
	refCol := findStructureColumn(t, orders, "reference")
	if refCol.Length != 64 || !refCol.Insertable || !refCol.Updatable {
		t.Errorf("reference column = %+v", refCol)
	}
	if refCol.Default != nil {
		t.Errorf("reference default = %v, want nil", *refCol.Default)
	}

	if orders.PrimaryKey == nil || !equalStrings(orders.PrimaryKey.Columns, []string{"id"}) {
		t.Errorf("s_orders primary key = %+v", orders.PrimaryKey)
	}
	if findStructureConstraint(t, orders, "PRIMARY KEY", []string{"id"}) == nil {
		t.Error("missing PRIMARY KEY constraint entry")
	}
	uq := findStructureConstraint(t, orders, "UNIQUE", []string{"reference"})
	if uq == nil || uq.Name != "s_orders_uk_reference" {
		t.Errorf("UNIQUE constraint = %+v", uq)
	}
	if supportsCheck {
		check := findStructureConstraint(t, orders, "CHECK", nil)
		if check == nil || check.Name != "s_orders_status_check" || check.Definition == "" {
			t.Errorf("CHECK constraint = %+v", check)
		}
	}

	if idx := findStructureIndexByName(orders.Indexes, "idx_s_orders_status"); idx == nil || idx.Method == "" {
		t.Errorf("idx_s_orders_status = %+v", idx)
	}
	if pk := findStructureIndexByName(orders.Indexes, "PRIMARY"); pk == nil || !pk.Primary || !equalStrings(pk.Columns, []string{"id"}) {
		t.Errorf("PRIMARY index = %+v", pk)
	}

	items, err := manager.TableStructure(ctx, "itest-structure-my", cfg, "", "s_order_items")
	if err != nil {
		t.Fatalf("TableStructure(s_order_items) error = %v", err)
	}
	if items.PrimaryKey == nil || !equalStrings(items.PrimaryKey.Columns, []string{"order_id", "sku"}) {
		t.Errorf("s_order_items primary key = %+v", items.PrimaryKey)
	}
	if len(items.ForeignKeys) != 1 {
		t.Fatalf("s_order_items FKs = %d, want 1", len(items.ForeignKeys))
	}
	fk := items.ForeignKeys[0]
	if fk.Name != "s_fk_items_order" || !equalStrings(fk.Columns, []string{"order_id"}) ||
		!equalStrings(fk.ReferencedColumns, []string{"id"}) || fk.ReferencedTable != "s_orders" {
		t.Errorf("items FK = %+v", fk)
	}
	if fk.OnDelete != "CASCADE" || fk.OnUpdate != "RESTRICT" {
		t.Errorf("items FK actions = update %q delete %q", fk.OnUpdate, fk.OnDelete)
	}
	if findStructureConstraint(t, items, "FOREIGN KEY", []string{"order_id"}) == nil {
		t.Error("missing FOREIGN KEY constraint entry for s_order_items")
	}

	if _, err := manager.TableStructure(ctx, "itest-structure-my", cfg, "", "no_such_table"); err == nil {
		t.Fatal("expected error for missing table")
	}
}

func findStructureColumn(t *testing.T, structure model.TableStructure, name string) model.StructureColumn {
	t.Helper()
	for _, column := range structure.Columns {
		if column.Name == name {
			return column
		}
	}
	t.Fatalf("column %q not found in table %q", name, structure.Table)
	return model.StructureColumn{}
}

func findStructureConstraint(t *testing.T, structure model.TableStructure, typ string, columns []string) *model.StructureConstraint {
	t.Helper()
	for i := range structure.Constraints {
		c := &structure.Constraints[i]
		if c.Type != typ {
			continue
		}
		if columns == nil || equalStrings(c.Columns, columns) {
			return c
		}
	}
	return nil
}

func findStructureIndexByName(indexes []model.StructureIndex, name string) *model.StructureIndex {
	for i := range indexes {
		if indexes[i].Name == name {
			return &indexes[i]
		}
	}
	return nil
}

// mysqlSupportsCheck determines whether the MySQL/MariaDB server understands
// CHECK constraints (MySQL >= 8.0.16, MariaDB >= 10.2), mirroring the driver's
// own gating so assertions adapt to the test server.
func mysqlSupportsCheck(t *testing.T, ctx context.Context, db *sql.DB) bool {
	t.Helper()
	var version string
	if err := db.QueryRowContext(ctx, `SELECT VERSION()`).Scan(&version); err != nil {
		t.Fatalf("SELECT VERSION() = %v", err)
	}
	major, minor, ok := parseMySQLVersion(version)
	if !ok {
		t.Skipf("cannot parse server version %q", version)
	}
	if strings.HasSuffix(version, "MariaDB") {
		return major > 10 || (major == 10 && minor >= 2)
	}
	return major > 8 || (major == 8 && minor >= 16)
}

// parseMySQLVersion extracts the leading major.minor from a server version
// string such as "8.0.36" or "10.5.19-MariaDB".
func parseMySQLVersion(version string) (major, minor int, ok bool) {
	fields := strings.SplitN(version, "-", 2)
	parts := strings.SplitN(fields[0], ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	m, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return m, n, true
}