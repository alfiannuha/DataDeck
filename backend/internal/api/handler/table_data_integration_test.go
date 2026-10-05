//go:build integration

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// mutate calls a row mutation endpoint with the srv profile and returns the
// HTTP status plus the sanitized error code (empty on success).
func mutate(t *testing.T, h *ConnectionHandler, update bool, body string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := mutationRequest("srv", body)
	if update {
		h.UpdateRow(rec, req)
	} else {
		h.DeleteRow(rec, req)
	}
	env := decodeEnvelope(t, rec)
	code := ""
	if env.Error != nil {
		code = env.Error.Code
	}
	return rec.Code, code
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
		if _, err := pool.ExecContext(context.Background(),
			`CREATE TABLE public.logs (message text, created_at timestamp)`); err != nil {
			t.Fatalf("logs in %s: %v", database, err)
		}
		if _, err := pool.ExecContext(context.Background(),
			`CREATE TABLE public.uniq (email text UNIQUE NOT NULL, name text)`); err != nil {
			t.Fatalf("uniq in %s: %v", database, err)
		}
		if _, err := pool.ExecContext(context.Background(),
			`CREATE TABLE public.memberships (tenant_id text NOT NULL, user_id bigint NOT NULL, role text, PRIMARY KEY (tenant_id, user_id))`); err != nil {
			t.Fatalf("memberships in %s: %v", database, err)
		}
		if _, err := pool.ExecContext(context.Background(),
			`INSERT INTO public.memberships VALUES ('A', 9223372036854775807, 'admin')`); err != nil {
			t.Fatalf("memberships insert in %s: %v", database, err)
		}
		// Insert/defaults/generated/type fixtures.
		if _, err := pool.ExecContext(context.Background(), `CREATE TABLE public.insert_defaults (
			id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			name text NOT NULL,
			status text NOT NULL DEFAULT 'active',
			created_at timestamptz NOT NULL DEFAULT now(),
			nickname text NULL)`); err != nil {
			t.Fatalf("insert_defaults in %s: %v", database, err)
		}
		if _, err := pool.ExecContext(context.Background(), `CREATE TABLE public.gen (
			a int PRIMARY KEY,
			b int GENERATED ALWAYS AS (a * 2) STORED)`); err != nil {
			t.Fatalf("gen in %s: %v", database, err)
		}
		if _, err := pool.ExecContext(context.Background(), `CREATE TABLE public.types (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			amount numeric(30,10),
			flag boolean,
			created date)`); err != nil {
			t.Fatalf("types in %s: %v", database, err)
		}
		if _, err := pool.ExecContext(context.Background(),
			`CREATE VIEW public.v_users AS SELECT id, marker FROM public.users`); err != nil {
			t.Fatalf("view in %s: %v", database, err)
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

	t.Run("structured filters are validated and parameterized", func(t *testing.T) {
		rowsFor := func(database, filters string) [][]any {
			_, page, _ := browse(t, h, "srv", url.Values{
				"database": {database}, "schema": {"public"}, "table": {"users"}, "filters": {filters},
			})
			return page.Rows
		}
		// Text equality (all alpha rows share the marker).
		if rows := rowsFor(dbA, `[{"column":"marker","operator":"equals","value":"ALPHA"}]`); len(rows) != 3 {
			t.Errorf("equals rows = %d, want 3", len(rows))
		}
		// NULL semantics.
		if rows := rowsFor(dbA, `[{"column":"note","operator":"is_null"}]`); len(rows) != 1 {
			t.Errorf("is_null rows = %d, want 1", len(rows))
		}
		// Numeric comparison + AND.
		if rows := rowsFor(dbA, `[{"column":"marker","operator":"equals","value":"ALPHA"},{"column":"id","operator":"greater_or_equal","value":"2"}]`); len(rows) != 2 {
			t.Errorf("AND rows = %d, want 2", len(rows))
		}
		// IN.
		if rows := rowsFor(dbA, `[{"column":"marker","operator":"in","values":["ALPHA","BETA"]}]`); len(rows) != 3 {
			t.Errorf("in rows = %d, want 3", len(rows))
		}
		// contains on a text column.
		if rows := rowsFor(dbA, `[{"column":"marker","operator":"contains","value":"LPH"}]`); len(rows) != 3 {
			t.Errorf("contains rows = %d, want 3", len(rows))
		}
		// Unknown column / invalid operator are rejected.
		if status, _, code := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"users"},
			"filters": {`[{"column":"nope","operator":"equals","value":"x"}]`},
		}); code != "COLUMN_NOT_FOUND" {
			t.Errorf("unknown column -> status %d code %q, want COLUMN_NOT_FOUND", status, code)
		}
		if status, _, code := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"users"},
			"filters": {`[{"column":"marker","operator":"raw","value":"x"}]`},
		}); code != "INVALID_FILTER" {
			t.Errorf("invalid operator -> status %d code %q, want INVALID_FILTER", status, code)
		}
	})

	t.Run("filter applies before sort and pagination over the whole dataset", func(t *testing.T) {
		_, first, _ := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"scores"},
			"filters":     {`[{"column":"score","operator":"greater_or_equal","value":100}]`},
			"sort_column": {"score"}, "sort_direction": {"asc"}, "page_size": {"100"},
		})
		if len(first.Rows) != 100 || first.Rows[0][1] != float64(100) || !first.Pagination.HasMore {
			t.Fatalf("filtered page1 = %d rows start %v has_more %v", len(first.Rows), first.Rows[0][1], first.Pagination.HasMore)
		}
		_, second, _ := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"scores"},
			"filters":     {`[{"column":"score","operator":"greater_or_equal","value":100}]`},
			"sort_column": {"score"}, "sort_direction": {"asc"}, "page_size": {"100"}, "page": {"2"},
		})
		if len(second.Rows) != 50 || second.Rows[0][1] != float64(200) || second.Pagination.HasMore {
			t.Fatalf("filtered page2 = %d rows start %v has_more %v", len(second.Rows), second.Rows[0][1], second.Pagination.HasMore)
		}
	})

	t.Run("cross-database filter isolation", func(t *testing.T) {
		_, alpha, _ := browse(t, h, "srv", url.Values{
			"database": {dbA}, "schema": {"public"}, "table": {"namers"},
			"filters": {`[{"column":"name","operator":"equals","value":"OMEGA"}]`},
		})
		if len(alpha.Rows) != 1 || alpha.Rows[0][1] != "OMEGA" {
			t.Fatalf("alpha filter rows = %#v, want OMEGA only", alpha.Rows)
		}
		_, beta, _ := browse(t, h, "srv", url.Values{
			"database": {dbB}, "schema": {"public"}, "table": {"namers"},
			"filters": {`[{"column":"name","operator":"equals","value":"ALPHA"}]`},
		})
		if len(beta.Rows) != 1 || beta.Rows[0][1] != "ALPHA" {
			t.Fatalf("beta filter rows = %#v, want ALPHA only", beta.Rows)
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

	t.Run("row mutation is isolated to the explicit database", func(t *testing.T) {
		body := func(database, changes, expected string) string {
			return `{"database":"` + database + `","schema":"public","table":"users","identity":{"id":"1"},` +
				`"expected":{` + expected + `},"changes":{` + changes + `}}`
		}
		// Update alpha row 1; beta row 1 must be untouched.
		if status, code := mutate(t, h, true, body(dbA, `"note":"ALPHA-EDIT"`, `"marker":"ALPHA"`)); status != http.StatusOK {
			t.Fatalf("alpha update failed: status %d code %q", status, code)
		}
		_, alpha, _ := browse(t, h, "srv", url.Values{"database": {dbA}, "schema": {"public"}, "table": {"users"}, "filters": {`[{"column":"id","operator":"equals","value":1}]`}})
		if alpha.Rows[0][3] != "ALPHA-EDIT" {
			t.Fatalf("alpha note = %#v, want ALPHA-EDIT", alpha.Rows[0][3])
		}
		_, beta, _ := browse(t, h, "srv", url.Values{"database": {dbB}, "schema": {"public"}, "table": {"users"}, "filters": {`[{"column":"id","operator":"equals","value":1}]`}})
		if beta.Rows[0][3] != nil {
			t.Fatalf("beta note = %#v, want untouched NULL", beta.Rows[0][3])
		}

		// Opposite direction.
		if status, code := mutate(t, h, true, body(dbB, `"note":"BETA-EDIT"`, `"marker":"BETA"`)); status != http.StatusOK {
			t.Fatalf("beta update failed: status %d code %q", status, code)
		}
		_, alpha2, _ := browse(t, h, "srv", url.Values{"database": {dbA}, "schema": {"public"}, "table": {"users"}, "filters": {`[{"column":"id","operator":"equals","value":1}]`}})
		if alpha2.Rows[0][3] != "ALPHA-EDIT" {
			t.Fatalf("alpha note changed by beta mutation: %#v", alpha2.Rows[0][3])
		}
	})

	t.Run("row conflict, not found and composite identity", func(t *testing.T) {
		if _, code := mutate(t, h, true, `{"database":"`+dbA+`","schema":"public","table":"users","identity":{"id":"1"},"expected":{"marker":"WRONG"},"changes":{"note":"x"}}`); code != "ROW_CONFLICT" {
			t.Fatalf("expected ROW_CONFLICT, got %q", code)
		}
		if _, code := mutate(t, h, true, `{"database":"`+dbA+`","schema":"public","table":"users","identity":{"id":"999"},"changes":{"note":"x"}}`); code != "ROW_NOT_FOUND" {
			t.Fatalf("expected ROW_NOT_FOUND, got %q", code)
		}
		// Composite PK (tenant_id, user_id) with an exact BIGINT.
		if status, code := mutate(t, h, true, `{"database":"`+dbA+`","schema":"public","table":"memberships","identity":{"tenant_id":"A","user_id":"9223372036854775807"},"changes":{"role":"lead"}}`); status != http.StatusOK {
			t.Fatalf("composite update failed: status %d code %q", status, code)
		}
		if _, code := mutate(t, h, true, `{"database":"`+dbA+`","schema":"public","table":"memberships","identity":{"tenant_id":"A"},"changes":{"role":"x"}}`); code != "ROW_IDENTITY_INVALID" {
			t.Fatalf("partial composite identity = %q, want ROW_IDENTITY_INVALID", code)
		}
	})

	t.Run("read-only and no-identity tables reject mutation", func(t *testing.T) {
		for _, table := range []string{"logs", "uniq"} {
			if _, code := mutate(t, h, true, `{"database":"`+dbA+`","schema":"public","table":"`+table+`","identity":{"message":"a"},"changes":{"message":"b"}}`); code != "ROW_IDENTITY_REQUIRED" {
				t.Fatalf("%s mutation code = %q, want ROW_IDENTITY_REQUIRED", table, code)
			}
		}
	})

	t.Run("insert honors defaults, generated columns, exact types and isolation", func(t *testing.T) {
		insert := func(body string) (int, string, []any) {
			rec := httptest.NewRecorder()
			req := mutationRequest("srv", body)
			req.Method = http.MethodPost
			h.InsertRow(rec, req)
			env := decodeEnvelope(t, rec)
			code := ""
			if env.Error != nil {
				code = env.Error.Code
			}
			var result struct {
				Row []any `json:"row"`
			}
			if env.Success {
				_ = json.Unmarshal(env.Data, &result)
			}
			return rec.Code, code, result.Row
		}

		// Value + NULL + DEFAULT (id/status/created omitted → database defaults).
		status, code, row := insert(`{"database":"` + dbA + `","schema":"public","table":"insert_defaults","values":{"name":{"mode":"value","value":"Alice"},"nickname":{"mode":"null"}}}`)
		if status != http.StatusOK {
			t.Fatalf("defaults insert status=%d code=%q", status, code)
		}
		if len(row) != 5 || row[1] != "Alice" || row[2] != "active" || row[3] == nil || row[4] != nil {
			t.Fatalf("defaults row = %#v, want db defaults applied", row)
		}

		// Generated column: cannot set b explicitly; computed value returned.
		if _, code, _ := insert(`{"database":"` + dbA + `","schema":"public","table":"gen","values":{"a":{"mode":"value","value":"5"},"b":{"mode":"value","value":"9"}}}`); code != "COLUMN_READ_ONLY" {
			t.Fatalf("generated insert code = %q, want COLUMN_READ_ONLY", code)
		}
		if _, _, row := insert(`{"database":"` + dbA + `","schema":"public","table":"gen","values":{"a":{"mode":"value","value":"5"}}}`); len(row) != 2 || row[1] != float64(10) {
			t.Fatalf("generated row = %#v, want b=10", row)
		}

		// Exact BIGINT + explicit UUID + high-precision NUMERIC + boolean + date.
		bigStatus, bigCode, bigRow := insert(`{"database":"` + dbA + `","schema":"public","table":"users","values":{"id":{"mode":"value","value":"20"},"marker":{"mode":"value","value":"INSERTED_ALPHA"},"big":{"mode":"value","value":"9223372036854775807"},"note":{"mode":"default"}}}`)
		if bigStatus != http.StatusOK {
			t.Fatalf("bigint insert status=%d code=%q", bigStatus, bigCode)
		}
		if len(bigRow) < 3 || bigRow[2] != "9223372036854775807" {
			t.Fatalf("bigint = %#v, want exact string", bigRow)
		}
		typeStatus, typeCode, typeRow := insert(`{"database":"` + dbA + `","schema":"public","table":"types","values":{"id":{"mode":"value","value":"11111111-1111-1111-1111-111111111111"},"amount":{"mode":"value","value":"12345678901234567890.1234567890"},"flag":{"mode":"value","value":true},"created":{"mode":"value","value":"2026-01-02"}}}`)
		if typeStatus != http.StatusOK {
			t.Fatalf("types insert status=%d code=%q", typeStatus, typeCode)
		}
		if typeRow[1] != "12345678901234567890.1234567890" || typeRow[2] != true || typeRow[3] != "2026-01-02" {
			t.Fatalf("types row = %#v", typeRow)
		}

		// Wrong-database isolation: the new alpha marker must not appear in beta.
		_, betaPage, _ := browse(t, h, "srv", url.Values{"database": {dbB}, "schema": {"public"}, "table": {"users"}, "filters": {`[{"column":"marker","operator":"equals","value":"INSERTED_ALPHA"}]`}})
		if len(betaPage.Rows) != 0 {
			t.Fatalf("beta contains alpha insert: %#v", betaPage.Rows)
		}

		// Constraints: duplicate PK and NULL into NOT NULL are sanitized.
		if _, code, _ := insert(`{"database":"` + dbA + `","schema":"public","table":"users","values":{"id":{"mode":"value","value":"20"},"marker":{"mode":"value","value":"x"}}}`); code != "CONSTRAINT_VIOLATION" {
			t.Fatalf("duplicate pk code = %q, want CONSTRAINT_VIOLATION", code)
		}
		if _, code, _ := insert(`{"database":"` + dbA + `","schema":"public","table":"insert_defaults","values":{"name":{"mode":"null"}}}`); code != "INVALID_COLUMN_VALUE" {
			t.Fatalf("null not-null code = %q, want INVALID_COLUMN_VALUE", code)
		}

		// No-PK base table insert allowed; view rejected.
		if status, code, _ := insert(`{"database":"` + dbA + `","schema":"public","table":"logs","values":{"message":{"mode":"value","value":"hello"}}}`); status != http.StatusOK {
			t.Fatalf("no-PK insert status=%d code=%q", status, code)
		}
		if _, code, _ := insert(`{"database":"` + dbA + `","schema":"public","table":"v_users","values":{"marker":{"mode":"value","value":"x"}}}`); code != "ROW_NOT_MUTABLE" {
			t.Fatalf("view insert code = %q, want ROW_NOT_MUTABLE", code)
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

	_, filtered, _ := browse(t, h, "my", url.Values{
		"table":   {"datadeck_td_users"},
		"filters": {`[{"column":"marker","operator":"equals","value":"MY"},{"column":"id","operator":"greater_than","value":1}]`},
	})
	if len(filtered.Rows) != 2 {
		t.Fatalf("mysql filtered rows = %d, want 2", len(filtered.Rows))
	}

	// Mutation: ClientFoundRows makes an UPDATE that does not change the value
	// still report 1 matched row (not ROW_NOT_FOUND).
	myMutate := func(update bool, body string) (int, string) {
		rec := httptest.NewRecorder()
		req := mutationRequest("my", body)
		if update {
			h.UpdateRow(rec, req)
		} else {
			h.DeleteRow(rec, req)
		}
		env := decodeEnvelope(t, rec)
		code := ""
		if env.Error != nil {
			code = env.Error.Code
		}
		return rec.Code, code
	}
	if status, code := myMutate(true, `{"table":"datadeck_td_users","identity":{"id":"1"},"expected":{"marker":"MY"},"changes":{"marker":"MY"}}`); status != http.StatusOK {
		t.Fatalf("mysql unchanged update status=%d code=%q (want 200)", status, code)
	}
	if _, code := myMutate(true, `{"table":"datadeck_td_users","identity":{"id":"1"},"expected":{"marker":"WRONG"},"changes":{"marker":"Z"}}`); code != "ROW_CONFLICT" {
		t.Fatalf("mysql conflict code = %q, want ROW_CONFLICT", code)
	}
	if _, code := myMutate(true, `{"table":"datadeck_td_users","identity":{"id":"999"},"changes":{"marker":"Z"}}`); code != "ROW_NOT_FOUND" {
		t.Fatalf("mysql not found code = %q, want ROW_NOT_FOUND", code)
	}

	// Insert: AUTO_INCREMENT default, DB default, exact BIGINT, unique constraint.
	if _, err := pool.ExecContext(context.Background(), `CREATE TABLE datadeck_td_insert (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(32) NOT NULL,
		status VARCHAR(16) NOT NULL DEFAULT 'active',
		note VARCHAR(32) NULL UNIQUE)`); err != nil {
		t.Fatalf("create insert table: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.ExecContext(context.Background(), "DROP TABLE IF EXISTS datadeck_td_insert") })
	myInsert := func(body string) (int, string) {
		rec := httptest.NewRecorder()
		req := mutationRequest("my", body)
		req.Method = http.MethodPost
		h.InsertRow(rec, req)
		env := decodeEnvelope(t, rec)
		code := ""
		if env.Error != nil {
			code = env.Error.Code
		}
		return rec.Code, code
	}
	if status, code := myInsert(`{"table":"datadeck_td_insert","values":{"name":{"mode":"value","value":"Auto"},"note":{"mode":"null"}}}`); status != http.StatusOK {
		t.Fatalf("mysql default insert status=%d code=%q", status, code)
	}
	_, inserted, _ := browse(t, h, "my", url.Values{"table": {"datadeck_td_insert"}, "filters": {`[{"column":"name","operator":"equals","value":"Auto"}]`}})
	if len(inserted.Rows) != 1 || inserted.Rows[0][2] != "active" || inserted.Rows[0][3] != nil {
		t.Fatalf("mysql inserted row = %#v, want db defaults applied", inserted.Rows)
	}
	if status, code := myInsert(`{"table":"datadeck_td_users","values":{"id":{"mode":"value","value":"9223372036854775807"},"marker":{"mode":"value","value":"BIG"},"note":{"mode":"null"}}}`); status != http.StatusOK {
		t.Fatalf("mysql bigint insert status=%d code=%q", status, code)
	}
	_, bigInserted, _ := browse(t, h, "my", url.Values{"table": {"datadeck_td_users"}, "filters": {`[{"column":"id","operator":"equals","value":"9223372036854775807"}]`}})
	if len(bigInserted.Rows) != 1 || bigInserted.Rows[0][0] != "9223372036854775807" {
		t.Fatalf("mysql bigint readback = %#v", bigInserted.Rows)
	}
	if _, code := myInsert(`{"table":"datadeck_td_users","values":{"id":{"mode":"value","value":"9223372036854775807"},"marker":{"mode":"value","value":"dup"}}}`); code != "CONSTRAINT_VIOLATION" {
		t.Fatalf("mysql duplicate pk code = %q, want CONSTRAINT_VIOLATION", code)
	}
	if status, code := myInsert(`{"table":"datadeck_td_insert","values":{"name":{"mode":"value","value":"Seeded"},"note":{"mode":"value","value":"uniq-note"}}}`); status != http.StatusOK {
		t.Fatalf("mysql unique seed status=%d code=%q", status, code)
	}
	if _, code := myInsert(`{"table":"datadeck_td_insert","values":{"name":{"mode":"value","value":"Dup"},"note":{"mode":"value","value":"uniq-note"}}}`); code != "CONSTRAINT_VIOLATION" {
		t.Fatalf("mysql unique code = %q, want CONSTRAINT_VIOLATION", code)
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

	// Filtering executes in the database (indexed equality + non-indexed scan).
	for _, spec := range []struct{ filters, label string }{
		{`[{"column":"id","operator":"equals","value":50000}]`, "indexed equality"},
		{`[{"column":"payload","operator":"equals","value":"payload-999"}]`, "non-indexed equality"},
	} {
		start := time.Now()
		status, filtered, _ := browse(t, h, "perf", url.Values{
			"database": {perfDB}, "schema": {"public"}, "table": {"big"}, "filters": {spec.filters},
		})
		if status != http.StatusOK || len(filtered.Rows) != 1 {
			t.Fatalf("%s: status=%d rows=%d", spec.label, status, len(filtered.Rows))
		}
		t.Logf("100k-row filter %s: %s", spec.label, time.Since(start))
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

	_, filtered, _ := browse(t, h, "sq", url.Values{
		"table":   {"items"},
		"filters": {`[{"column":"label","operator":"is_null"}]`},
	})
	if len(filtered.Rows) != 1 {
		t.Fatalf("sqlite is_null rows = %d, want 1", len(filtered.Rows))
	}
	_, contains, _ := browse(t, h, "sq", url.Values{
		"table":   {"items"},
		"filters": {`[{"column":"big","operator":"greater_or_equal","value":"9007199254740993"}]`},
	})
	if len(contains.Rows) != 1 {
		t.Fatalf("sqlite bigint filter rows = %d, want 1", len(contains.Rows))
	}

	// Decimal-compatible mutation + optimistic conflict.
	sqMutate := func(update bool, body string) (int, string) {
		rec := httptest.NewRecorder()
		req := mutationRequest("sq", body)
		if update {
			h.UpdateRow(rec, req)
		} else {
			h.DeleteRow(rec, req)
		}
		env := decodeEnvelope(t, rec)
		code := ""
		if env.Error != nil {
			code = env.Error.Code
		}
		return rec.Code, code
	}
	if status, code := sqMutate(true, `{"table":"items","identity":{"id":"2"},"expected":{"label":null},"changes":{"label":"b"}}`); status != http.StatusOK {
		t.Fatalf("sqlite null-expected update status=%d code=%q", status, code)
	}
	if _, code := sqMutate(true, `{"table":"items","identity":{"id":"2"},"expected":{"label":"NOPE"},"changes":{"label":"c"}}`); code != "ROW_CONFLICT" {
		t.Fatalf("sqlite conflict code = %q", code)
	}

	sqInsert := func(body string) (int, string) {
		rec := httptest.NewRecorder()
		req := mutationRequest("sq", body)
		req.Method = http.MethodPost
		h.InsertRow(rec, req)
		env := decodeEnvelope(t, rec)
		code := ""
		if env.Error != nil {
			code = env.Error.Code
		}
		return rec.Code, code
	}
	if status, code := sqInsert(`{"table":"items","values":{"label":{"mode":"value","value":"d"},"big":{"mode":"value","value":"3"}}}`); status != http.StatusOK {
		t.Fatalf("sqlite insert status=%d code=%q", status, code)
	}
	// INTEGER PRIMARY KEY is engine-assigned: an explicit VALUE is rejected.
	if _, code := sqInsert(`{"table":"items","values":{"id":{"mode":"value","value":"99"},"label":{"mode":"value","value":"x"}}}`); code != "COLUMN_READ_ONLY" {
		t.Fatalf("sqlite rowid identity explicit value code = %q, want COLUMN_READ_ONLY", code)
	}
	if _, err := raw.Exec(`CREATE TABLE keyed (k TEXT PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create keyed: %v", err)
	}
	if status, code := sqInsert(`{"table":"keyed","values":{"k":{"mode":"value","value":"a"},"v":{"mode":"value","value":"1"}}}`); status != http.StatusOK {
		t.Fatalf("sqlite text-pk insert status=%d code=%q", status, code)
	}
	if _, code := sqInsert(`{"table":"keyed","values":{"k":{"mode":"value","value":"a"},"v":{"mode":"value","value":"2"}}}`); code != "CONSTRAINT_VIOLATION" {
		t.Fatalf("sqlite duplicate code = %q, want CONSTRAINT_VIOLATION", code)
	}
	if _, err := raw.Exec(`CREATE TABLE nopk (message TEXT)`); err != nil {
		t.Fatalf("create nopk: %v", err)
	}
	if status, code := sqInsert(`{"table":"nopk","values":{"message":{"mode":"value","value":"hi"}}}`); status != http.StatusOK {
		t.Fatalf("sqlite no-PK insert status=%d code=%q", status, code)
	}
}
