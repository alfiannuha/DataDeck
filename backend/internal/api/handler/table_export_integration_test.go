//go:build integration

package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xuri/excelize/v2"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// exportRequest builds a POST /table-data/export request with the route params
// populated, mirroring mutationRequest.
func exportRequest(id, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/"+id+"/table-data/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// callExport runs TableExport and returns the recorded response.
func callExport(t *testing.T, h *ConnectionHandler, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.TableExport(rec, exportRequest(id, body))
	return rec
}

// exportBody is a small builder so tests keep bodies readable.
func exportBody(format, database, schema, table string) string {
	raw, err := json.Marshal(map[string]any{
		"format": format, "database": database, "schema": schema, "table": table,
	})
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// parseCSV decodes a CSV export body into records, skipping empty trailing lines.
func parseCSV(t *testing.T, body string) [][]string {
	t.Helper()
	records, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v\nbody:\n%s", err, body)
	}
	return records
}

// TestTableExportAPIIntegrationPostgres exercises the CSV/XLSX/SQL sinks and the
// explicit-target binding rule against a real PostgreSQL server.
func TestTableExportAPIIntegrationPostgres(t *testing.T) {
	pg := pgTableConfig(t)
	h, db, cipher := newTestHandler(t)

	adminMgr := database.NewManager(database.DefaultOptions(), database.Postgres{})
	t.Cleanup(func() { _ = adminMgr.CloseAll() })
	admin, err := adminMgr.Open(context.Background(), "exp-admin", pg)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}

	const expDB = "datadeck_exp"
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+expDB+" WITH (FORCE)")
	})
	execAdmin(t, admin, "DROP DATABASE IF EXISTS "+expDB+" WITH (FORCE)")
	execAdmin(t, admin, "CREATE DATABASE "+expDB)

	seedCfg := pg
	seedCfg.Database = expDB
	pool, err := adminMgr.Open(context.Background(), "exp-seed", seedCfg)
	if err != nil {
		t.Fatalf("open seed pool: %v", err)
	}
	defer func() { _ = adminMgr.CloseConnection("exp-seed") }()

	if _, err := pool.ExecContext(context.Background(), `CREATE TABLE public.exp_t (
		id bigint PRIMARY KEY,
		name text,
		big bigint,
		amount numeric(10,2),
		active boolean DEFAULT true,
		inserted timestamp,
		tags jsonb,
		payload bytea,
		note text)`); err != nil {
		t.Fatalf("create exp_t: %v", err)
	}
	if _, err := pool.ExecContext(context.Background(), `INSERT INTO public.exp_t (id, name, big, amount, active, inserted, tags, payload, note) VALUES
		(1, '=1+1', 9007199254740993, 12.34, true, '2024-01-02 03:04:05', '{"k":"v"}', decode('616263','hex'), NULL),
		(2, 'plain', 0, -42.99, false, NULL, NULL, NULL, 'b, "tail"')`); err != nil {
		t.Fatalf("insert exp_t: %v", err)
	}
	if _, err := pool.ExecContext(context.Background(),
		`CREATE VIEW public.exp_v AS SELECT id, name FROM public.exp_t`); err != nil {
		t.Fatalf("create view: %v", err)
	}

	seedProfile(t, db, &model.ConnectionProfile{
		ID: "exp", Name: "Export", Driver: model.DriverPostgres,
		Host: strPtr(pg.Host), Port: intPtr(pg.Port), DatabaseName: "",
		Username: strPtr(pg.Username), EncryptedPassword: encryptedOrNil(t, cipher, pg.Password), SSLMode: "disable",
	})

	t.Run("csv full export", func(t *testing.T) {
		rec := callExport(t, h, "exp", exportBody("csv", expDB, "public", "exp_t"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, `attachment; filename="datadeck_exp_public_exp_t_2`) || !strings.HasSuffix(cd, `.csv"`) {
			t.Errorf("content-disposition = %q", cd)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
			t.Errorf("content-type = %q", ct)
		}
		records := parseCSV(t, rec.Body.String())
		if len(records) != 3 {
			t.Fatalf("csv rows = %d, want header + 2", len(records))
		}
		wantHeaders := "id,name,big,amount,active,inserted,tags,payload,note"
		if got := strings.Join(records[0], ","); got != wantHeaders {
			t.Errorf("headers = %q, want %q", got, wantHeaders)
		}
		row1 := mapRow(t, records[1], records[0])
		if row1["id"] != "1" || row1["name"] != "'=1+1" {
			t.Errorf("row1 id/name = %q/%q", row1["id"], row1["name"])
		}
		if row1["big"] != "9007199254740993" {
			t.Errorf("big = %q, want exact 9007199254740993", row1["big"])
		}
		if row1["amount"] != "12.34" || row1["active"] != "true" {
			t.Errorf("amount/active = %q/%q", row1["amount"], row1["active"])
		}
		if row1["inserted"] != "2024-01-02T03:04:05Z" {
			t.Errorf("inserted = %q", row1["inserted"])
		}
		if row1["tags"] != `{"k":"v"}` {
			t.Errorf("tags = %q", row1["tags"])
		}
		if row1["payload"] != "YWJj" {
			t.Errorf("payload = %q, want base64 YWJj", row1["payload"])
		}
		if row1["note"] != "" {
			t.Errorf("note = %q, want empty for NULL", row1["note"])
		}
		row2 := mapRow(t, records[2], records[0])
		if row2["note"] != `b, "tail"` || row2["amount"] != "-42.99" {
			t.Errorf("row2 note/amount = %q/%q", row2["note"], row2["amount"])
		}
	})

	t.Run("csv respects filters", func(t *testing.T) {
		body := exportBody("csv", expDB, "public", "exp_t")
		body = strings.TrimSuffix(body, "}")
		body += `,"filters":[{"column":"id","operator":"equals","value":"2"}]}`
		records := parseCSV(t, callExport(t, h, "exp", body).Body.String())
		if len(records) != 2 {
			t.Fatalf("filtered rows = %d, want header + 1", len(records))
		}
		if records[1][3] != "-42.99" {
			t.Errorf("filtered row = %q, want the id=2 row", records[1])
		}
	})

	t.Run("xlsx round trip", func(t *testing.T) {
		rec := callExport(t, h, "exp", exportBody("xlsx", expDB, "public", "exp_t"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		book, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatalf("open workbook: %v", err)
		}
		sheet := book.GetSheetName(0)
		rows, err := book.GetRows(sheet)
		if err != nil {
			t.Fatalf("get rows: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("xlsx rows = %d, want 3", len(rows))
		}
		if got := strings.Join(rows[0], ","); got != "id,name,big,amount,active,inserted,tags,payload,note" {
			t.Fatalf("xlsx header = %q", got)
		}
		// XLSX writes typed string cells, so a formula-looking value stays a
		// literal string (Excel never evaluates t="str" cells); the CSV guard
		// covers the format that opens as live formulas.
		if rows[1][1] != "=1+1" || rows[1][2] != "9007199254740993" {
			t.Errorf("xlsx row1 = %q", rows[1])
		}
		if rows[1][6] != `{"k":"v"}` {
			t.Errorf("xlsx tags = %q", rows[1][6])
		}
	})

	t.Run("sql script", func(t *testing.T) {
		rec := callExport(t, h, "exp", exportBody("sql", expDB, "public", "exp_t"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, want := range []string{
			"-- data for \"public\".\"exp_t\"\n",
			"BEGIN;\n",
			`INSERT INTO "public"."exp_t" ("id", "name", "big", "amount", "active", "inserted", "tags", "payload", "note") VALUES ('1', '=1+1', '9007199254740993', '12.34', TRUE, '2024-01-02T03:04:05Z', '{"k":"v"}', decode('YWJj','base64'), NULL);`,
			`INSERT INTO "public"."exp_t" ("id", "name", "big", "amount", "active", "inserted", "tags", "payload", "note") VALUES ('2', 'plain', '0', '-42.99', FALSE, NULL, NULL, NULL, 'b, "tail"');`,
			"COMMIT;\n",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("sql script missing %q\n%s", want, body)
			}
		}
	})

	t.Run("sql without transaction", func(t *testing.T) {
		body := exportBody("sql", expDB, "public", "exp_t")
		body = strings.TrimSuffix(body, "}")
		body += `,"transaction":false}`
		raw := callExport(t, h, "exp", body).Body.String()
		if strings.Contains(raw, "BEGIN;") || strings.Contains(raw, "COMMIT;") {
			t.Errorf("transaction=false script still wraps in BEGIN/COMMIT:\n%s", raw)
		}
	})

	t.Run("sql export of a view is rejected", func(t *testing.T) {
		rec := callExport(t, h, "exp", exportBody("sql", expDB, "public", "exp_v"))
		env := decodeEnvelope(t, rec)
		if rec.Code != http.StatusBadRequest || env.Error.Code != "EXPORT_FORMAT_UNSUPPORTED" {
			t.Fatalf("view sql export = %d %s", rec.Code, env.Error.Code)
		}
	})

	t.Run("missing table is 404", func(t *testing.T) {
		rec := callExport(t, h, "exp", exportBody("csv", expDB, "public", "nope"))
		env := decodeEnvelope(t, rec)
		if rec.Code != http.StatusNotFound || env.Error.Code != "TABLE_NOT_FOUND" {
			t.Fatalf("missing table = %d %s", rec.Code, env.Error.Code)
		}
	})

	t.Run("bad format is 400", func(t *testing.T) {
		rec := callExport(t, h, "exp", exportBody("pdf", expDB, "public", "exp_t"))
		env := decodeEnvelope(t, rec)
		if rec.Code != http.StatusBadRequest || env.Error.Code != "EXPORT_FORMAT_UNSUPPORTED" {
			t.Fatalf("bad format = %d %s", rec.Code, env.Error.Code)
		}
	})

	t.Run("missing database is 400", func(t *testing.T) {
		rec := callExport(t, h, "exp", exportBody("csv", "", "public", "exp_t"))
		env := decodeEnvelope(t, rec)
		if rec.Code != http.StatusBadRequest || env.Error.Code != "DATABASE_REQUIRED" {
			t.Fatalf("missing database = %d %s", rec.Code, env.Error.Code)
		}
	})
}

func mapRow(t *testing.T, record, header []string) map[string]string {
	t.Helper()
	row := make(map[string]string, len(header))
	for i, cellHeader := range header {
		row[cellHeader] = record[i]
	}
	return row
}

// TestTableExportIntegrationSQLite covers the shared SQLite stream path and the
// engine-without-schema binding (database field is the profile path).
func TestTableExportIntegrationSQLite(t *testing.T) {
	h, db, _ := newTestHandler(t)

	path := filepath.Join(t.TempDir(), "export.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if _, err := raw.Exec(`CREATE TABLE exp_t (id INTEGER PRIMARY KEY, name TEXT, big INTEGER, note TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO exp_t VALUES (1, '=1+1', 9007199254740993, NULL), (2, 'plain', -42, 'sq note')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "exp-sq", Name: "SQLite export", Driver: model.DriverSQLite, DatabaseName: path,
	})

	t.Run("csv", func(t *testing.T) {
		rec := callExport(t, h, "exp-sq", exportBody("csv", path, "", "exp_t"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		records := parseCSV(t, rec.Body.String())
		if len(records) != 3 {
			t.Fatalf("rows = %d, want header + 2", len(records))
		}
		if records[0][0] != "id" || records[1][1] != "'=1+1" || records[1][3] != "" {
			t.Errorf("csv = %q", records)
		}
		if records[1][2] != "9007199254740993" {
			t.Errorf("big = %q, want exact string", records[1][2])
		}
	})

	t.Run("sql", func(t *testing.T) {
		body := callExport(t, h, "exp-sq", exportBody("sql", path, "", "exp_t")).Body.String()
		// The rowid-alias INTEGER PRIMARY KEY is Identity, so the insertable
		// SQL export excludes it (like InsertRow does).
		if !strings.Contains(body, `INSERT INTO "exp_t" ("name", "big", "note") VALUES ('=1+1', '9007199254740993', NULL);`) {
			t.Errorf("sql missing row 1:\n%s", body)
		}
		if !strings.Contains(body, "('plain', '-42', 'sq note')") || !strings.Contains(body, "BEGIN;\n") {
			t.Errorf("sql missing row 2/transaction:\n%s", body)
		}
	})
}

// TestTableExportIntegrationMySQL covers the MySQL-specific literal escaping
// (backslash doubling) against a real server.
func TestTableExportIntegrationMySQL(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_MYSQL_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_MYSQL_HOST not set; skipping MySQL table export integration test")
	}
	port := 3306
	if raw := os.Getenv("DATADECK_TEST_MYSQL_PORT"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_MYSQL_PORT %q: %v", raw, err)
		}
		port = value
	}
	user := envOr("DATADECK_TEST_MYSQL_USER", "root")
	databaseName := "datadeck_exp"

	h, db, cipher := newTestHandler(t)

	adminMgr := database.NewManager(database.DefaultOptions(), database.MySQL{})
	t.Cleanup(func() { _ = adminMgr.CloseAll() })
	adminCfg := database.Config{
		Driver: model.DriverMySQL, Host: host, Port: port, Database: "mysql",
		Username: user, Password: os.Getenv("DATADECK_TEST_MYSQL_PASSWORD"), SSLMode: "disable",
	}
	admin, err := adminMgr.Open(context.Background(), "exp-my-admin", adminCfg)
	if err != nil {
		t.Fatalf("open mysql admin: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+databaseName) })
	execAdmin(t, admin, "DROP DATABASE IF EXISTS "+databaseName)
	execAdmin(t, admin, "CREATE DATABASE "+databaseName+" CHARACTER SET utf8mb4")

	seedCfg := adminCfg
	seedCfg.Database = databaseName
	seedPool, err := adminMgr.Open(context.Background(), "exp-my-seed", seedCfg)
	if err != nil {
		t.Fatalf("open seed pool: %v", err)
	}
	defer func() { _ = adminMgr.CloseConnection("exp-my-seed") }()
	if _, err := seedPool.ExecContext(context.Background(), `CREATE TABLE exp_t (
		id BIGINT PRIMARY KEY, name VARCHAR(64), big BIGINT, amount DECIMAL(10,2), note TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	// a\b is a stored literal backslash (MySQL doubles it in the script).
	if _, err := seedPool.ExecContext(context.Background(),
		`INSERT INTO exp_t VALUES (1, 'a\\b', 9007199254740993, 12.34, NULL), (2, 'plain', 0, -1.50, 'tail')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	seedProfile(t, db, &model.ConnectionProfile{
		ID: "exp-my", Name: "MySQL export", Driver: model.DriverMySQL,
		Host: strPtr(host), Port: intPtr(port), DatabaseName: databaseName,
		Username: strPtr(user), EncryptedPassword: encryptedOrNil(t, cipher, os.Getenv("DATADECK_TEST_MYSQL_PASSWORD")), SSLMode: "disable",
	})

	t.Run("csv", func(t *testing.T) {
		rec := callExport(t, h, "exp-my", exportBody("csv", databaseName, "", "exp_t"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		records := parseCSV(t, rec.Body.String())
		if len(records) != 3 || records[0][0] != "id" {
			t.Fatalf("csv = %q", records)
		}
		if records[1][2] != "9007199254740993" || records[1][1] != `a\b` {
			t.Errorf("row1 = %q", records[1])
		}
	})

	t.Run("sql backslash escaping", func(t *testing.T) {
		body := callExport(t, h, "exp-my", exportBody("sql", databaseName, "", "exp_t")).Body.String()
		if !strings.Contains(body, "START TRANSACTION;\n") {
			t.Errorf("missing START TRANSACTION:\n%s", body)
		}
		wantRow := "INSERT INTO `exp_t` (`id`, `name`, `big`, `amount`, `note`) VALUES ('1', 'a\\\\b', '9007199254740993', '12.34', NULL);"
		if !strings.Contains(body, wantRow) {
			t.Errorf("sql row 1 escaping wrong:\n%s\nwant substring:\n%s", body, wantRow)
		}
	})
}

// TestTableExportPerformancePostgres exports a 100k-row table and verifies the
// streaming sinks complete within a bounded time (no unbounded buffering).
func TestTableExportPerformancePostgres(t *testing.T) {
	pg := pgTableConfig(t)
	h, db, cipher := newTestHandler(t)

	adminMgr := database.NewManager(database.DefaultOptions(), database.Postgres{})
	t.Cleanup(func() { _ = adminMgr.CloseAll() })
	admin, err := adminMgr.Open(context.Background(), "exp-perf-admin", pg)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	const perfDB = "datadeck_exp_perf"
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+perfDB+" WITH (FORCE)")
	})
	execAdmin(t, admin, "DROP DATABASE IF EXISTS "+perfDB+" WITH (FORCE)")
	execAdmin(t, admin, "CREATE DATABASE "+perfDB)

	cfg := pg
	cfg.Database = perfDB
	pool, err := adminMgr.Open(context.Background(), "exp-perf-seed", cfg)
	if err != nil {
		t.Fatalf("open seed pool: %v", err)
	}
	t.Cleanup(func() { _ = adminMgr.CloseConnection("exp-perf-seed") })
	if _, err := pool.ExecContext(context.Background(),
		`CREATE TABLE public.big AS SELECT g AS id, 'payload-' || g AS payload FROM generate_series(1, 100000) g`); err != nil {
		t.Fatalf("seed perf table: %v", err)
	}

	seedProfile(t, db, &model.ConnectionProfile{
		ID: "exp-perf", Name: "Export perf", Driver: model.DriverPostgres,
		Host: strPtr(pg.Host), Port: intPtr(pg.Port), DatabaseName: "",
		Username: strPtr(pg.Username), EncryptedPassword: encryptedOrNil(t, cipher, pg.Password), SSLMode: "disable",
	})

	start := time.Now()
	rec := callExport(t, h, "exp-perf", exportBody("csv", perfDB, "public", "big"))
	elapsed := time.Since(start)
	if rec.Code != http.StatusOK {
		t.Fatalf("csv status = %d, body = %s", rec.Code, rec.Body.String())
	}
	records := parseCSV(t, rec.Body.String())
	if len(records) != 100001 {
		t.Fatalf("csv rows = %d, want 100001 (header + 100000)", len(records))
	}
	if elapsed > 20*time.Second {
		t.Errorf("100k csv export took %s, want bounded (< 20s)", elapsed)
	}
	t.Logf("100k csv export: %s, %d rows, %d bytes", elapsed, len(records), rec.Body.Len())

	start = time.Now()
	rec = callExport(t, h, "exp-perf", exportBody("sql", perfDB, "public", "big"))
	elapsed = time.Since(start)
	if rec.Code != http.StatusOK {
		t.Fatalf("sql status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if lines := strings.Count(rec.Body.String(), "\n"); lines < 100000 {
		t.Errorf("sql export produced %d lines, want >= 100000", lines)
	}
	if elapsed > 30*time.Second {
		t.Errorf("100k sql export took %s, want bounded (< 30s)", elapsed)
	}
	t.Logf("100k sql export: %s, %d bytes", elapsed, rec.Body.Len())
}
