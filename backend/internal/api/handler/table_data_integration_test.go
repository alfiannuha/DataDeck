//go:build integration

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// browsePage decodes the Table Data page payload from an envelope.
type browsePage struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Table    string `json:"table"`
	Columns  []struct {
		Name       string `json:"name"`
		PrimaryKey bool   `json:"primary_key"`
		Nullable   bool   `json:"nullable"`
	} `json:"columns"`
	Rows       [][]any `json:"rows"`
	Pagination struct {
		Page     int  `json:"page"`
		PageSize int  `json:"page_size"`
		HasMore  bool `json:"has_more"`
	} `json:"pagination"`
	Truncated bool `json:"truncated"`
}

func browse(t *testing.T, h *ConnectionHandler, id string, query url.Values) (int, browsePage, string) {
	t.Helper()
	rec := callTableData(t, h, id, query)
	env := decodeEnvelope(t, rec)
	if !env.Success {
		code := ""
		if env.Error != nil {
			code = env.Error.Code
		}
		return rec.Code, browsePage{}, code
	}
	var page browsePage
	if err := json.Unmarshal(env.Data, &page); err != nil {
		t.Fatalf("decode page: %v (%s)", err, rec.Body.String())
	}
	return rec.Code, page, ""
}

func pgTableConfig(t *testing.T) database.Config {
	t.Helper()
	host := envOr("DATADECK_TEST_PG_HOST", "")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping PostgreSQL Table Data integration test")
	}
	port := 5432
	if raw := envOr("DATADECK_TEST_PG_PORT", ""); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_PG_PORT: %v", err)
		}
		port = parsed
	}
	return database.Config{
		Driver:   model.DriverPostgres,
		Host:     host,
		Port:     port,
		Database: envOr("DATADECK_TEST_PG_DATABASE", "postgres"),
		Username: os.Getenv("DATADECK_TEST_PG_USER"),
		Password: os.Getenv("DATADECK_TEST_PG_PASSWORD"),
		SSLMode:  envOr("DATADECK_TEST_PG_SSLMODE", "disable"),
	}
}

func TestTableDataAPIIntegrationPostgres(t *testing.T) {
	pg := pgTableConfig(t)
	h, db, cipher := newTestHandler(t)

	adminMgr := database.NewManager(database.DefaultOptions(), database.Postgres{})
	t.Cleanup(func() { _ = adminMgr.CloseAll() })
	admin, err := adminMgr.Open(context.Background(), "admin", pg)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}

	const (
		dbA = "datadeck_td_alpha"
		dbB = "datadeck_td_beta"
	)
	t.Cleanup(func() {
		for _, name := range []string{dbA, dbB} {
			_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		}
	})

	for _, name := range []string{dbA, dbB} {
		execAdmin(t, admin, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		execAdmin(t, admin, "CREATE DATABASE "+name)
	}
	seedTable := func(database, marker string) {
		cfg := pg
		cfg.Database = database
		pool, err := adminMgr.Open(context.Background(), "seed-"+database, cfg)
		if err != nil {
			t.Fatalf("open %s: %v", database, err)
		}
		defer func() { _ = adminMgr.CloseConnection("seed-" + database) }()
		if _, err := pool.ExecContext(context.Background(), `CREATE TABLE public.users (
			id bigint PRIMARY KEY, marker text, big bigint, note text)`); err != nil {
			t.Fatalf("create users in %s: %v", database, err)
		}
		if _, err := pool.ExecContext(context.Background(),
			`INSERT INTO public.users VALUES (1, $1, 9007199254740993, NULL), (2, $1, 0, 'x'), (3, $1, 0, 'y')`,
			marker); err != nil {
			t.Fatalf("insert %s: %v", database, err)
		}
		// Cross-database sorting fixture: same table name, different values.
		names := map[string][]string{
			"ALPHA": {"ZETA", "OMEGA", "THETA"},
			"BETA":  {"ALPHA", "BETA", "GAMMA"},
		}[marker]
		if _, err := pool.ExecContext(context.Background(),
			`CREATE TABLE public.namers (id int PRIMARY KEY, name text)`); err != nil {
			t.Fatalf("create namers in %s: %v", database, err)
		}
		for i, name := range names {
			if _, err := pool.ExecContext(context.Background(),
				`INSERT INTO public.namers VALUES ($1, $2)`, i+1, name); err != nil {
				t.Fatalf("insert namer %s: %v", database, err)
			}
		}
		// >pageSize rows in deliberately mixed order to prove global sorting.
		if _, err := pool.ExecContext(context.Background(),
			`CREATE TABLE public.scores AS SELECT g AS id, (g * 37) % 250 AS score FROM generate_series(1, 250) g`); err != nil {
			t.Fatalf("create scores in %s: %v", database, err)
		}
		if _, err := pool.ExecContext(context.Background(),
			`ALTER TABLE public.scores ADD PRIMARY KEY (id)`); err != nil {
			t.Fatalf("scores pk in %s: %v", database, err)
		}
	}
	seedTable(dbA, "ALPHA")
	seedTable(dbB, "BETA")

	seedProfile(t, db, &model.ConnectionProfile{
		ID: "srv", Name: "Server", Driver: model.DriverPostgres,
		Host: strPtr(pg.Host), Port: intPtr(pg.Port), DatabaseName: "",
		Username: strPtr(pg.Username), EncryptedPassword: encryptedOrNil(t, cipher, pg.Password), SSLMode: "disable",
	})

	t.Run("explicit database binding returns the right marker", func(t *testing.T) {
		status, page, _ := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"users"},
		})
		if status != http.StatusOK {
			t.Fatalf("status = %d", status)
		}
		if got := page.Rows[0][1]; got != "ALPHA" {
			t.Fatalf("alpha marker = %#v, want ALPHA (BLOCKER otherwise)", got)
		}
		for _, row := range page.Rows {
			if row[1] == "BETA" {
				t.Fatalf("alpha request leaked BETA: %#v", row)
			}
		}

		status, page, _ = browse(t, h, "srv", url.Values{
			"database": {dbB}, "schema": {"public"}, "table": {"users"},
		})
		if status != http.StatusOK || page.Rows[0][1] != "BETA" {
			t.Fatalf("beta marker = %#v (status %d), want BETA", page.Rows, status)
		}
	})

	t.Run("bigint exact and null preserved", func(t *testing.T) {
		_, page, _ := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"users"},
		})
		if page.Rows[0][2] != "9007199254740993" {
			t.Errorf("bigint = %#v, want exact string", page.Rows[0][2])
		}
		if page.Rows[0][3] != nil {
			t.Errorf("NULL note = %#v, want nil", page.Rows[0][3])
		}
		if len(page.Columns) != 4 || page.Columns[0].Name != "id" || !page.Columns[0].PrimaryKey {
			t.Errorf("columns = %+v, want id primary key first", page.Columns)
		}
	})

	t.Run("pagination has_more", func(t *testing.T) {
		_, first, _ := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"users"}, "page_size": {"2"},
		})
		if len(first.Rows) != 2 || !first.Pagination.HasMore {
			t.Fatalf("page1 = %d rows has_more=%v, want 2/true", len(first.Rows), first.Pagination.HasMore)
		}
		_, second, _ := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"users"}, "page_size": {"2"}, "page": {"2"},
		})
		if len(second.Rows) != 1 || second.Pagination.HasMore {
			t.Fatalf("page2 = %d rows has_more=%v, want 1/false", len(second.Rows), second.Pagination.HasMore)
		}
	})

	t.Run("unknown and malicious identifiers", func(t *testing.T) {
		for _, q := range []url.Values{
			{"database": {dbA}, "schema": {"private"}, "table": {"users"}},
			{"database": {dbA}, "schema": {"public"}, "table": {"users; DROP TABLE users"}},
			{"database": {dbA}, "schema": {`public"`}, "table": {"users"}},
			{"database": {dbA + "?sslmode=disable"}, "schema": {"public"}, "table": {"users"}},
		} {
			status, _, code := browse(t, h, "srv", q)
			if code != "TABLE_NOT_FOUND" && code != "DATABASE_NOT_FOUND" {
				t.Errorf("query %v -> status %d code %q, want TABLE_NOT_FOUND/DATABASE_NOT_FOUND", q, status, code)
			}
		}
	})

	t.Run("sort is applied server-side across the whole dataset", func(t *testing.T) {
		_, first, _ := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"scores"},
			"sort_column": {"score"}, "sort_direction": {"asc"}, "page_size": {"100"},
		})
		if len(first.Rows) != 100 {
			t.Fatalf("page1 rows = %d, want 100", len(first.Rows))
		}
		if first.Rows[0][1] != float64(0) || first.Rows[99][1] != float64(99) {
			t.Fatalf("page1 scores start/end = %v/%v, want 0/99 (global asc)", first.Rows[0][1], first.Rows[99][1])
		}
		if !first.Pagination.HasMore {
			t.Error("page1 has_more = false, want true")
		}

		_, second, _ := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"scores"},
			"sort_column": {"score"}, "sort_direction": {"asc"}, "page_size": {"100"}, "page": {"2"},
		})
		if len(second.Rows) != 100 || second.Rows[0][1] != float64(100) {
			t.Fatalf("page2 first score = %v, want 100 (continues global order)", second.Rows[0][1])
		}
	})

	t.Run("sort DESC and unknown sort column", func(t *testing.T) {
		_, page, _ := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"users"},
			"sort_column": {"id"}, "sort_direction": {"desc"},
		})
		if page.Rows[0][0] != "3" {
			t.Fatalf("first id = %v, want 3 (desc)", page.Rows[0][0])
		}
		status, _, code := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"users"},
			"sort_column": {`id"; DROP TABLE users`}, "sort_direction": {"asc"},
		})
		if code != "VALIDATION_ERROR" {
			t.Fatalf("malicious sort -> status %d code %q, want VALIDATION_ERROR", status, code)
		}
	})

	t.Run("cross-database sort isolation", func(t *testing.T) {
		names := func(database string) []any {
			_, page, _ := browse(t, h, "srv", url.Values{
				"database": {database}, "schema": {"public"}, "table": {"namers"},
				"sort_column": {"name"}, "sort_direction": {"asc"},
			})
			out := make([]any, 0, len(page.Rows))
			for _, row := range page.Rows {
				out = append(out, row[1])
			}
			return out
		}
		alphaNames := names(dbA)
		betaNames := names(dbB)
		wantAlpha := []any{"OMEGA", "THETA", "ZETA"}
		wantBeta := []any{"ALPHA", "BETA", "GAMMA"}
		if len(alphaNames) != 3 || alphaNames[0] != wantAlpha[0] || alphaNames[2] != wantAlpha[2] {
			t.Fatalf("alpha sorted names = %v, want %v", alphaNames, wantAlpha)
		}
		if len(betaNames) != 3 || betaNames[0] != wantBeta[0] || betaNames[2] != wantBeta[2] {
			t.Fatalf("beta sorted names = %v, want %v", betaNames, wantBeta)
		}
	})
}

func TestTableDataAPIIntegrationMySQL(t *testing.T) {
	host := envOr("DATADECK_TEST_MYSQL_HOST", "")
	if host == "" {
		t.Skip("DATADECK_TEST_MYSQL_HOST not set; skipping MySQL Table Data integration test")
	}
	port := 3306
	if raw := envOr("DATADECK_TEST_MYSQL_PORT", ""); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_MYSQL_PORT: %v", err)
		}
		port = parsed
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

	h, db, cipher := newTestHandler(t)
	mgr := database.NewManager(database.DefaultOptions(), database.MySQL{})
	t.Cleanup(func() { _ = mgr.CloseAll() })
	pool, err := mgr.Open(context.Background(), "seed", cfg)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.ExecContext(context.Background(), "DROP TABLE IF EXISTS datadeck_td_users") })
	if _, err := pool.ExecContext(context.Background(), `CREATE TABLE datadeck_td_users (
		id BIGINT PRIMARY KEY, marker VARCHAR(32), note VARCHAR(32))`); err != nil {
		t.Fatalf("create mysql table: %v", err)
	}
	if _, err := pool.ExecContext(context.Background(), `INSERT INTO datadeck_td_users VALUES
		(1, 'MY', NULL), (2, 'MY', 'x'), (3, 'MY', 'y')`); err != nil {
		t.Fatalf("insert mysql: %v", err)
	}

	seedProfile(t, db, &model.ConnectionProfile{
		ID: "my", Name: "MySQL", Driver: model.DriverMySQL,
		Host: strPtr(cfg.Host), Port: intPtr(cfg.Port), DatabaseName: cfg.Database,
		Username: strPtr(cfg.Username), EncryptedPassword: encryptedOrNil(t, cipher, cfg.Password), SSLMode: "disable",
	})

	status, page, _ := browse(t, h, "my", url.Values{"table": {"datadeck_td_users"}})
	if status != http.StatusOK || len(page.Rows) != 3 {
		t.Fatalf("status=%d rows=%d, want 200/3", status, len(page.Rows))
	}
	if page.Rows[0][2] != nil {
		t.Errorf("mysql NULL = %#v, want nil", page.Rows[0][2])
	}
	if !page.Columns[0].PrimaryKey {
		t.Errorf("mysql id primary_key = false")
	}

	_, paged, _ := browse(t, h, "my", url.Values{"table": {"datadeck_td_users"}, "page_size": {"2"}})
	if len(paged.Rows) != 2 || !paged.Pagination.HasMore {
		t.Fatalf("mysql page = %d rows has_more=%v", len(paged.Rows), paged.Pagination.HasMore)
	}

	_, sorted, _ := browse(t, h, "my", url.Values{
		"table": {"datadeck_td_users"}, "sort_column": {"id"}, "sort_direction": {"desc"},
	})
	if len(sorted.Rows) != 3 || sorted.Rows[0][0] != "3" {
		t.Fatalf("mysql sort desc first id = %v, want 3", sorted.Rows[0][0])
	}
}

// TestTableDataPerformancePostgres verifies a 100k-row table returns a bounded,
// fast first page (no COUNT(*), no full-table load).
func TestTableDataPerformancePostgres(t *testing.T) {
	pg := pgTableConfig(t)
	h, db, cipher := newTestHandler(t)

	adminMgr := database.NewManager(database.DefaultOptions(), database.Postgres{})
	t.Cleanup(func() { _ = adminMgr.CloseAll() })
	admin, err := adminMgr.Open(context.Background(), "admin", pg)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	const perfDB = "datadeck_td_perf"
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+perfDB+" WITH (FORCE)")
	})
	execAdmin(t, admin, "DROP DATABASE IF EXISTS "+perfDB+" WITH (FORCE)")
	execAdmin(t, admin, "CREATE DATABASE "+perfDB)

	cfg := pg
	cfg.Database = perfDB
	pool, err := adminMgr.Open(context.Background(), "seed-perf", cfg)
	if err != nil {
		t.Fatalf("open perf db: %v", err)
	}
	t.Cleanup(func() { _ = adminMgr.CloseConnection("seed-perf") })
	if _, err := pool.ExecContext(context.Background(),
		`CREATE TABLE public.big AS SELECT g AS id, 'payload-' || g AS payload FROM generate_series(1, 100000) g`); err != nil {
		t.Fatalf("seed perf table: %v", err)
	}
	if _, err := pool.ExecContext(context.Background(), `ALTER TABLE public.big ADD PRIMARY KEY (id)`); err != nil {
		t.Fatalf("add pk: %v", err)
	}

	seedProfile(t, db, &model.ConnectionProfile{
		ID: "perf", Name: "Perf", Driver: model.DriverPostgres,
		Host: strPtr(pg.Host), Port: intPtr(pg.Port), DatabaseName: "",
		Username: strPtr(pg.Username), EncryptedPassword: encryptedOrNil(t, cipher, pg.Password), SSLMode: "disable",
	})

	start := time.Now()
	status, page, _ := browse(t, h, "perf", url.Values{
		"database": {perfDB}, "schema": {"public"}, "table": {"big"},
	})
	elapsed := time.Since(start)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(page.Rows) != 100 || !page.Pagination.HasMore {
		t.Fatalf("rows=%d has_more=%v, want 100/true", len(page.Rows), page.Pagination.HasMore)
	}
	if elapsed > 5*time.Second {
		t.Errorf("first page took %s, want bounded (< 5s)", elapsed)
	}
	t.Logf("100k-row first page: %s, rows=%d", elapsed, len(page.Rows))

	// Sorted variants stay bounded (database performs the ordering).
	for _, spec := range []struct{ column, direction, label string }{
		{"id", "desc", "indexed PK desc"},
		{"payload", "asc", "non-indexed text asc"},
	} {
		start := time.Now()
		status, sorted, _ := browse(t, h, "perf", url.Values{
			"database": {perfDB}, "schema": {"public"}, "table": {"big"},
			"sort_column": {spec.column}, "sort_direction": {spec.direction},
		})
		if status != http.StatusOK || len(sorted.Rows) != 100 {
			t.Fatalf("%s: status=%d rows=%d", spec.label, status, len(sorted.Rows))
		}
		t.Logf("100k-row sort %s: %s", spec.label, time.Since(start))
	}
}

func TestTableDataIntegrationSQLite(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "td.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if _, err := raw.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, label TEXT, big INTEGER);
		INSERT INTO items VALUES (1, 'a', 9007199254740993), (2, NULL, 1), (3, 'c', 2)`); err != nil {
		t.Fatalf("seed sqlite: %v", err)
	}
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "sq", Name: "SQLite", Driver: model.DriverSQLite, DatabaseName: path,
	})

	status, page, _ := browse(t, h, "sq", url.Values{"table": {"items"}})
	if status != http.StatusOK || len(page.Rows) != 3 {
		t.Fatalf("status=%d rows=%d", status, len(page.Rows))
	}
	if page.Rows[0][2] != "9007199254740993" {
		t.Errorf("sqlite biginteger = %#v, want exact string", page.Rows[0][2])
	}
	if page.Rows[1][1] != nil {
		t.Errorf("sqlite NULL = %#v, want nil", page.Rows[1][1])
	}

	_, paged, _ := browse(t, h, "sq", url.Values{"table": {"items"}, "page_size": {"2"}, "page": {"2"}})
	if len(paged.Rows) != 1 || paged.Pagination.HasMore {
		t.Fatalf("sqlite page2 = %d rows has_more=%v", len(paged.Rows), paged.Pagination.HasMore)
	}

	_, sorted, _ := browse(t, h, "sq", url.Values{
		"table": {"items"}, "sort_column": {"id"}, "sort_direction": {"desc"},
	})
	if len(sorted.Rows) != 3 || sorted.Rows[0][0] != float64(3) {
		t.Fatalf("sqlite sort desc first id = %v, want 3", sorted.Rows[0][0])
	}
}
