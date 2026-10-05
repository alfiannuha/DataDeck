package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func tableDataRequest(id string, query url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections/"+id+"/table-data?"+query.Encode(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func callTableData(t *testing.T, h *ConnectionHandler, id string, query url.Values) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.TableData(rec, tableDataRequest(id, query))
	return rec
}

func seedSQLiteUsers(t *testing.T, db *sql.DB, path string) {
	t.Helper()
	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "sq",
		Name:         "SQLite users",
		Driver:       model.DriverSQLite,
		DatabaseName: path,
	})
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if _, err := raw.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		name TEXT,
		big INTEGER,
		note TEXT
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO users (id, name, big, note) VALUES
		(1, 'Alfian', 9007199254740993, NULL),
		(2, NULL, 10, 'x')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func TestTableDataSQLiteBrowsing(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	seedSQLiteUsers(t, db, path)

	rec := callTableData(t, h, "sq", url.Values{"table": {"users"}, "page_size": {"100"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if !env.Success {
		t.Fatalf("success = false: %s", rec.Body.String())
	}
	var page struct {
		Table   string `json:"table"`
		Columns []struct {
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
	}
	if err := json.Unmarshal(env.Data, &page); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if page.Table != "users" || len(page.Rows) != 2 {
		t.Fatalf("page = %+v", page)
	}
	if len(page.Columns) != 4 || page.Columns[0].Name != "id" || !page.Columns[0].PrimaryKey {
		t.Errorf("columns = %+v, want id first with primary_key", page.Columns)
	}
	// BIGINT beyond JS safe range stays an exact string; NULL stays null.
	if got := page.Rows[0][2]; got != "9007199254740993" {
		t.Errorf("big value = %#v, want exact string", got)
	}
	if page.Rows[1][1] != nil {
		t.Errorf("NULL name = %#v, want nil", page.Rows[1][1])
	}
	if page.Pagination.PageSize != 100 || page.Pagination.HasMore {
		t.Errorf("pagination = %+v", page.Pagination)
	}
	if env.Meta["page_size"] != float64(100) {
		t.Errorf("meta page_size = %#v", env.Meta["page_size"])
	}
}

func TestTableDataClampsOversizedPage(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	seedSQLiteUsers(t, db, path)

	rec := callTableData(t, h, "sq", url.Values{"table": {"users"}, "page_size": {"99999"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeEnvelope(t, rec).Meta["page_size"]; got != float64(maxPageSize) {
		t.Errorf("page_size = %#v, want clamped %d", got, maxPageSize)
	}
}

func TestTableDataValidation(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	seedSQLiteUsers(t, db, path)

	cases := []struct {
		name  string
		query url.Values
		code  string
	}{
		{"missing table", url.Values{}, "VALIDATION_ERROR"},
		{"invalid page", url.Values{"table": {"users"}, "page": {"0"}}, "VALIDATION_ERROR"},
		{"invalid page_size", url.Values{"table": {"users"}, "page_size": {"-3"}}, "VALIDATION_ERROR"},
		{"unknown table", url.Values{"table": {"nope"}}, "TABLE_NOT_FOUND"},
		{"malicious table identifier", url.Values{"table": {"users; DROP TABLE users"}}, "TABLE_NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := callTableData(t, h, "sq", tc.query)
			env := decodeEnvelope(t, rec)
			if env.Error == nil || env.Error.Code != tc.code {
				t.Fatalf("status=%d error=%+v, want %s", rec.Code, env.Error, tc.code)
			}
			if env.Success {
				t.Error("success = true, want false")
			}
		})
	}
}

func TestTableDataPostgresRequiresSchema(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "pg",
		Name:         "PG",
		Driver:       model.DriverPostgres,
		Host:         strPtr("127.0.0.1"),
		Port:         intPtr(5432),
		DatabaseName: "alpha",
		Username:     strPtr("u"),
	})

	rec := callTableData(t, h, "pg", url.Values{"table": {"users"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("error = %+v, want VALIDATION_ERROR", env.Error)
	}
}

func TestTableDataServerLevelRequiresDatabase(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "srv",
		Name:         "Server",
		Driver:       model.DriverPostgres,
		Host:         strPtr("127.0.0.1"),
		Port:         intPtr(5432),
		DatabaseName: "",
		Username:     strPtr("u"),
	})

	rec := callTableData(t, h, "srv", url.Values{"table": {"users"}, "schema": {"public"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "DATABASE_REQUIRED" {
		t.Errorf("error = %+v, want DATABASE_REQUIRED", env.Error)
	}
}

func TestTableDataUnknownConnection(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := callTableData(t, h, "ghost", url.Values{"table": {"users"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestTableDataSortValidation(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	seedSQLiteUsers(t, db, path)

	cases := []struct {
		name  string
		query url.Values
		code  string
	}{
		{"invalid direction", url.Values{"table": {"users"}, "sort_column": {"name"}, "sort_direction": {"random()"}}, "VALIDATION_ERROR"},
		{"direction without column", url.Values{"table": {"users"}, "sort_direction": {"asc"}}, "VALIDATION_ERROR"},
		{"column without direction", url.Values{"table": {"users"}, "sort_column": {"name"}}, "VALIDATION_ERROR"},
		{"unknown column", url.Values{"table": {"users"}, "sort_column": {"nope"}, "sort_direction": {"asc"}}, "VALIDATION_ERROR"},
		{"malicious column", url.Values{"table": {"users"}, "sort_column": {`name"; DROP TABLE users`}, "sort_direction": {"asc"}}, "VALIDATION_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := callTableData(t, h, "sq", tc.query)
			env := decodeEnvelope(t, rec)
			if env.Error == nil || env.Error.Code != tc.code {
				t.Fatalf("status=%d error=%+v, want %s", rec.Code, env.Error, tc.code)
			}
		})
	}
}

func TestTableDataSortOrdersRowsServerSide(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	seedSQLiteUsers(t, db, path)

	rec := callTableData(t, h, "sq", url.Values{
		"table": {"users"}, "sort_column": {"id"}, "sort_direction": {"desc"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body=%s)", rec.Code, rec.Body.String())
	}
	var page struct {
		Rows [][]any `json:"rows"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Rows) != 2 || page.Rows[0][0] != float64(2) || page.Rows[1][0] != float64(1) {
		t.Fatalf("rows = %+v, want id order [2,1]", page.Rows)
	}
}

func TestTableDataFilters(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	seedSQLiteUsers(t, db, path)

	count := func(t *testing.T, query url.Values) [][]any {
		t.Helper()
		rec := callTableData(t, h, "sq", query)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (body=%s)", rec.Code, rec.Body.String())
		}
		var page struct {
			Rows [][]any `json:"rows"`
		}
		if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &page); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return page.Rows
	}

	if rows := count(t, url.Values{"table": {"users"}, "filters": {`[{"column":"name","operator":"equals","value":"Alfian"}]`}}); len(rows) != 1 {
		t.Errorf("equals rows = %d, want 1", len(rows))
	}
	if rows := count(t, url.Values{"table": {"users"}, "filters": {`[{"column":"name","operator":"is_null"}]`}}); len(rows) != 1 {
		t.Errorf("is_null rows = %d, want 1", len(rows))
	}
	if rows := count(t, url.Values{"table": {"users"}, "filters": {`[{"column":"id","operator":"greater_or_equal","value":2}]`}}); len(rows) != 1 {
		t.Errorf("numeric rows = %d, want 1", len(rows))
	}
	if rows := count(t, url.Values{
		"table":       {"users"},
		"filters":     {`[{"column":"id","operator":"greater_or_equal","value":1}]`},
		"sort_column": {"id"}, "sort_direction": {"desc"},
	}); len(rows) != 2 || rows[0][0] != float64(2) {
		t.Errorf("filter+sort rows = %#v, want id desc [2,1]", rows)
	}
}

func TestTableDataFilterErrors(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	seedSQLiteUsers(t, db, path)

	cases := []struct {
		name   string
		filter string
		code   string
	}{
		{"malformed json", `not-json`, "VALIDATION_ERROR"},
		{"unknown column", `[{"column":"nope","operator":"equals","value":"x"}]`, "COLUMN_NOT_FOUND"},
		{"malicious column", `[{"column":"name; DROP TABLE users","operator":"equals","value":"x"}]`, "COLUMN_NOT_FOUND"},
		{"invalid operator", `[{"column":"name","operator":"raw","value":"x"}]`, "INVALID_FILTER"},
		{"incompatible type", `[{"column":"big","operator":"contains","value":"9"}]`, "INVALID_FILTER"},
		{"is_null with value", `[{"column":"name","operator":"is_null","value":"x"}]`, "INVALID_FILTER"},
		{"empty in", `[{"column":"name","operator":"in","values":[]}]`, "INVALID_FILTER"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := callTableData(t, h, "sq", url.Values{"table": {"users"}, "filters": {tc.filter}})
			env := decodeEnvelope(t, rec)
			if env.Error == nil || env.Error.Code != tc.code {
				t.Fatalf("status=%d error=%+v, want %s", rec.Code, env.Error, tc.code)
			}
		})
	}
}
