package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func mapValues(key, value string) url.Values {
	return url.Values{key: {value}}
}

func mutationRequest(id, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/connections/"+id+"/table-data/rows", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func seedSQLiteMutation(t *testing.T, db *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mut.db")
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "sq", Name: "SQLite", Driver: model.DriverSQLite, DatabaseName: path,
	})
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if _, err := raw.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, amount INTEGER);
		CREATE TABLE logs (message TEXT, created_at TEXT);
		CREATE TABLE big (id TEXT PRIMARY KEY, note TEXT);
		INSERT INTO users VALUES (1, 'Alfie', 10), (2, 'Bea', 20);
		INSERT INTO logs VALUES ('a', 'now');
		INSERT INTO big VALUES ('9223372036854775807', 'x')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func doMutation(t *testing.T, h *ConnectionHandler, update bool, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := mutationRequest("sq", body)
	if update {
		h.UpdateRow(rec, req)
	} else {
		h.DeleteRow(rec, req)
	}
	return rec
}

func TestUpdateRowSQLite(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedSQLiteMutation(t, db)

	rec := doMutation(t, h, true, `{"database":"","schema":"","table":"users","identity":{"id":"1"},"expected":{"name":"Alfie"},"changes":{"name":"Alfred"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body=%s)", rec.Code, rec.Body.String())
	}
	var result struct {
		AffectedRows int   `json:"affected_rows"`
		Row          []any `json:"row"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.AffectedRows != 1 || len(result.Row) != 3 || result.Row[1] != "Alfred" {
		t.Fatalf("result = %+v", result)
	}
}

func TestUpdateRowConflictAndNotFound(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedSQLiteMutation(t, db)

	conflict := doMutation(t, h, true, `{"table":"users","identity":{"id":"1"},"expected":{"name":"Nope"},"changes":{"name":"X"}}`)
	if env := decodeEnvelope(t, conflict); env.Error == nil || env.Error.Code != "ROW_CONFLICT" {
		t.Fatalf("conflict = %+v (status %d)", env.Error, conflict.Code)
	}

	missing := doMutation(t, h, true, `{"table":"users","identity":{"id":"999"},"changes":{"name":"X"}}`)
	if env := decodeEnvelope(t, missing); env.Error == nil || env.Error.Code != "ROW_NOT_FOUND" {
		t.Fatalf("missing = %+v (status %d)", env.Error, missing.Code)
	}
}

func TestDeleteRowSQLite(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedSQLiteMutation(t, db)

	first := doMutation(t, h, false, `{"table":"users","identity":{"id":"2"}}`)
	if first.Code != http.StatusOK {
		t.Fatalf("delete status = %d (body=%s)", first.Code, first.Body.String())
	}
	second := doMutation(t, h, false, `{"table":"users","identity":{"id":"2"}}`)
	if env := decodeEnvelope(t, second); env.Error == nil || env.Error.Code != "ROW_NOT_FOUND" {
		t.Fatalf("second delete = %+v", env.Error)
	}
}

func TestMutationRejections(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedSQLiteMutation(t, db)

	cases := []struct {
		name string
		body string
		code string
	}{
		{"no primary key", `{"table":"logs","identity":{"message":"a"},"changes":{"message":"b"}}`, "ROW_IDENTITY_REQUIRED"},
		{"pk change", `{"table":"users","identity":{"id":"1"},"changes":{"id":"2"}}`, "COLUMN_READ_ONLY"},
		{"extra identity", `{"table":"users","identity":{"id":"1","name":"Alfie"},"changes":{"name":"x"}}`, "ROW_IDENTITY_INVALID"},
		{"missing identity", `{"table":"users","identity":{},"changes":{"name":"x"}}`, "ROW_IDENTITY_REQUIRED"},
		{"unknown change column", `{"table":"users","identity":{"id":"1"},"changes":{"nope":"x"}}`, "ROW_IDENTITY_INVALID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doMutation(t, h, true, tc.body)
			env := decodeEnvelope(t, rec)
			if env.Error == nil || env.Error.Code != tc.code {
				t.Fatalf("status=%d error=%+v, want %s", rec.Code, env.Error, tc.code)
			}
		})
	}
}

func TestMutationBigintIdentityExact(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedSQLiteMutation(t, db)

	rec := doMutation(t, h, true, `{"table":"big","identity":{"id":"9223372036854775807"},"changes":{"note":"y"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestTableDataExposesRowCapabilities(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedSQLiteMutation(t, db)

	rec := callTableData(t, h, "sq", mapValues("table", "users"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var page struct {
		RowCapabilities struct {
			Insert, Update, Delete, Duplicate bool
		} `json:"row_capabilities"`
		RowIdentity struct {
			Kind    string   `json:"kind"`
			Columns []string `json:"columns"`
		} `json:"row_identity"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !page.RowCapabilities.Update || !page.RowCapabilities.Delete {
		t.Errorf("users caps = %+v, want update/delete true", page.RowCapabilities)
	}
	if page.RowIdentity.Kind != "primary_key" || len(page.RowIdentity.Columns) != 1 {
		t.Errorf("identity = %+v", page.RowIdentity)
	}

	noPK := callTableData(t, h, "sq", mapValues("table", "logs"))
	var noPKPage struct {
		RowCapabilities struct {
			Update, Delete bool
		} `json:"row_capabilities"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, noPK).Data, &noPKPage); err != nil {
		t.Fatalf("decode noPK: %v", err)
	}
	if noPKPage.RowCapabilities.Update || noPKPage.RowCapabilities.Delete {
		t.Errorf("no-PK caps = %+v, want update/delete false", noPKPage.RowCapabilities)
	}
}
