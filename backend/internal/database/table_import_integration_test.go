//go:build integration

// Round-trip import integration tests (PG/MySQL): export a table to CSV and to
// SQL, then replay BOTH into fresh copies through the table-import pipeline,
// verifying value fidelity (BIGINT/booleans/NULL/comma+quote text) and the
// all-or-nothing constraint rollback. Skipped unless the DATADECK_TEST_*
// variables point at a reachable server, so plain `go test ./...` never needs
// one. Run with:
//
//	go test -tags=integration ./internal/database/ -run TestImportRoundTrip -v
package database_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/export"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/tableimport"
)

const deepBigInt = "9007199254740993" // 2^53+1: must survive the round trip exactly

func TestImportRoundTripPostgres(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_PG_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping PostgreSQL import round-trip")
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
	runImportRoundTrip(t, cfg, importRT{
		quote: func(s string) string { return `"` + s + `"` },
		src:   "dd_import_src_pg", dstSQL: "dd_import_dst_sql_pg", dstCSV: "dd_import_dst_csv_pg",
		schema: "public",
	})
}

func TestImportRoundTripMySQL(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_MYSQL_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_MYSQL_HOST not set; skipping MySQL import round-trip")
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
	runImportRoundTrip(t, cfg, importRT{
		quote: func(s string) string { return "`" + s + "`" },
		src:   "dd_import_src_my", dstSQL: "dd_import_dst_sql_my", dstCSV: "dd_import_dst_csv_my",
	})
}

type importRT struct {
	quote       func(s string) string
	src, dstSQL string
	dstCSV      string
	schema      string
}

func runImportRoundTrip(t *testing.T, cfg database.Config, rt importRT) {
	t.Helper()
	ctx := context.Background()
	manager := database.NewManager(database.DefaultOptions(), database.Postgres{}, database.MySQL{}, database.SQLite{})
	defer func() { _ = manager.CloseAll() }()

	q := func(sql string) {
		if _, err := manager.Execute(ctx, "rt", cfg, sql); err != nil {
			t.Fatalf("exec %q: %v", sql, err)
		}
	}

	for _, name := range []string{rt.src, rt.dstSQL, rt.dstCSV} {
		_, _ = manager.Execute(ctx, "rt", cfg, "DROP TABLE IF EXISTS "+rt.quote(name))
	}
	t.Cleanup(func() {
		for _, name := range []string{rt.src, rt.dstSQL, rt.dstCSV} {
			_, _ = manager.Execute(ctx, "rt", cfg, "DROP TABLE IF EXISTS "+rt.quote(name))
		}
	})

	ddl := func(name string) string {
		if cfg.Driver == model.DriverMySQL {
			return "CREATE TABLE " + rt.quote(name) + " (id BIGINT PRIMARY KEY, name VARCHAR(255), active TINYINT(1), joined DATE, note TEXT)"
		}
		return "CREATE TABLE " + rt.quote(name) + " (id BIGINT PRIMARY KEY, name TEXT, active BOOLEAN, joined DATE, note TEXT)"
	}
	for _, name := range []string{rt.src, rt.dstCSV, rt.dstSQL} {
		q(ddl(name))
	}

	seed := `INSERT INTO ` + rt.quote(rt.src) + ` (id, name, active, joined, note) VALUES
		(` + deepBigInt + `, 'Ada', ` + boolLit(cfg.Driver, true) + `, '2024-01-02', 'comma, "quote"'),
		(2, 'Bob', ` + boolLit(cfg.Driver, false) + `, NULL, NULL),
		(3, 'Cara', ` + boolLit(cfg.Driver, true) + `, '2025-05-06', 'plain')`
	q(seed)

	// 1) CSV round-trip.
	csv := exportTo(t, manager, "rt", cfg, rt.schema, rt.src, "csv")
	importFrom(t, manager, "rt", cfg, rt.schema, rt.dstCSV, csv, "csv")

	// 2) SQL round-trip.
	sqlScript := exportTo(t, manager, "rt", cfg, rt.schema, rt.src, "sql")
	importFrom(t, manager, "rt", cfg, rt.schema, rt.dstSQL, sqlScript, "sql")

	for _, table := range []string{rt.dstCSV, rt.dstSQL} {
		if got := scalar(t, manager, "rt", cfg, "SELECT count(*) FROM "+rt.quote(table)); got != "3" {
			t.Errorf("%s rows = %s, want 3", table, got)
		}
		if got := scalar(t, manager, "rt", cfg, "SELECT id FROM "+rt.quote(table)+" WHERE id = "+deepBigInt); got != deepBigInt {
			t.Errorf("%s bigint = %s, want %s", table, got, deepBigInt)
		}
		if got := scalar(t, manager, "rt", cfg, "SELECT note FROM "+rt.quote(table)+" WHERE id = "+deepBigInt); got != `comma, "quote"` {
			t.Errorf("%s text fidelity = %q", table, got)
		}
	}

	// All-or-nothing: replaying the same CSV hits the PK constraint and rolls
	// back — no partial/duplicate rows remain.
	if err := importErr(t, manager, "rt", cfg, rt.schema, rt.dstCSV, csv, "csv"); !errors.Is(err, database.ErrConstraintViolation) {
		t.Fatalf("replay error = %v, want ErrConstraintViolation", err)
	}
	if got := scalar(t, manager, "rt", cfg, "SELECT count(*) FROM "+rt.quote(rt.dstCSV)); got != "3" {
		t.Errorf("after replay rollback rows = %s, want 3", got)
	}
}

// exportTo streams the table into a CSV script or SQL INSERT script string.
func exportTo(t *testing.T, m *database.Manager, id string, cfg database.Config, schema, table, format string) string {
	t.Helper()
	var buf bytes.Buffer
	var sink database.ExportSink
	switch format {
	case "sql":
		sink = export.NewSQL(&buf, export.SQLOptions{Driver: string(cfg.Driver), Schema: schema, Table: table, Transaction: true})
	default:
		sink = export.NewCSV(&buf, true)
	}
	err := m.ExportTable(context.Background(), id, cfg, model.TableExportRequest{
		Format: format, Schema: schema, Table: table, InsertableOnly: true,
	}, sink)
	if err != nil {
		t.Fatalf("ExportTable(%s): %v", format, err)
	}
	if err := sink.End(); err != nil {
		t.Fatalf("ExportTable(%s) sink.End(): %v", format, err)
	}
	return buf.String()
}

// importFrom parses src (csv header or sql script) and imports into table.
// It returns the pipeline error so callers can assert on sentinel error kinds.
func importErr(t *testing.T, m *database.Manager, id string, cfg database.Config, schema, table, src, format string) error {
	t.Helper()
	req := database.ImportPlanRequest{Schema: schema, Table: table}
	stream := func(emit func(row []model.ImportCell) error) error {
		switch format {
		case "sql":
			_, cols, _, err := tableimport.ParseSQLInserts(strings.NewReader(src), tableimport.SQLOptions{Dialect: string(cfg.Driver)}, emit)
			if err != nil {
				return err
			}
			req.SourceColumns = cols
			return nil
		default:
			cols, _, err := tableimport.ParseCSV(strings.NewReader(src), tableimport.CSVOptions{HeaderRow: true}, emit)
			if err != nil {
				return err
			}
			req.SourceColumns = cols
			return nil
		}
	}
	_, err := m.ExecuteImportRows(context.Background(), id, cfg, req, stream)
	return err
}

func importFrom(t *testing.T, m *database.Manager, id string, cfg database.Config, schema, table, src, format string) {
	t.Helper()
	if err := importErr(t, m, id, cfg, schema, table, src, format); err != nil {
		t.Fatalf("import(%s -> %s): %v", format, table, err)
	}
}

func scalar(t *testing.T, m *database.Manager, id string, cfg database.Config, sqlText string) string {
	t.Helper()
	res, err := m.Execute(context.Background(), id, cfg, sqlText)
	if err != nil {
		t.Fatalf("query %q: %v", sqlText, err)
	}
	if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
		t.Fatalf("query %q returned no cell", sqlText)
	}
	return res.Rows[0][0].(string)
}

func boolLit(driver model.Driver, v bool) string {
	if driver == model.DriverMySQL {
		if v {
			return "1"
		}
		return "0"
	}
	if v {
		return "TRUE"
	}
	return "FALSE"
}
