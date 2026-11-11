package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func backupRequest(id, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/"+id+"/backup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// PRF-04/Phase 3: transport/validation paths that must be rejected before any
// target-database work is attempted.
func TestTableBackupValidation(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "pg", Name: "PG", Driver: model.DriverPostgres,
		Host: strPtr("127.0.0.1"), Port: intPtr(5432), DatabaseName: "app", Username: strPtr("u"),
	})

	cases := []struct {
		name string
		id   string
		body string
		want int
		code string
	}{
		{"missing table", "pg", `{"schema":"public"}`, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"postgres missing schema", "pg", `{"table":"orders"}`, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"unknown profile", "missing", `{"schema":"public","table":"orders"}`, http.StatusNotFound, "NOT_FOUND"},
		{"malformed json", "pg", `{`, http.StatusBadRequest, "VALIDATION_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.TableBackup(rec, backupRequest(tc.id, tc.body))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
			env := decodeEnvelope(t, rec)
			if env.Error == nil || env.Error.Code != tc.code {
				t.Errorf("error = %+v, want %s", env.Error, tc.code)
			}
		})
	}
}

func TestConnectionBackupValidation(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := httptest.NewRecorder()
	h.ConnectionBackup(rec, backupRequest("missing", `{}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown profile status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ConnectionBackup(rec, backupRequest("missing", `{`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed body status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
}
