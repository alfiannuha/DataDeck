package handler

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
)

func schemasRequest(id string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections/"+id+"/schemas", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func strPtr(v string) *string { return &v }
func intPtr(v int) *int       { return &v }

func seedProfile(t *testing.T, db *sql.DB, profile *model.ConnectionProfile) {
	t.Helper()
	if err := repository.NewConnectionRepository(db).Create(context.Background(), profile); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
}

func TestSchemasUnknownProfile(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := httptest.NewRecorder()
	h.Schemas(rec, schemasRequest("does-not-exist"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Errorf("error = %+v, want NOT_FOUND", env.Error)
	}
}

func TestSchemasSQLiteEmptyDatabase(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "c1",
		Name:         "SQLite target",
		Driver:       model.DriverSQLite,
		DatabaseName: filepath.Join(t.TempDir(), "user.db"),
	})

	rec := httptest.NewRecorder()
	h.Schemas(rec, schemasRequest("c1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if !decodeEnvelope(t, rec).Success {
		t.Error("sqlite schemas success = false")
	}
}

func TestSchemasUnreachableDatabase(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "c2",
		Name:         "Unreachable PG",
		Driver:       model.DriverPostgres,
		Host:         strPtr("127.0.0.1"),
		Port:         intPtr(1),
		DatabaseName: "app",
		Username:     strPtr("appuser"),
	})

	rec := httptest.NewRecorder()
	h.Schemas(rec, schemasRequest("c2"))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "INTROSPECTION_ERROR" {
		t.Errorf("error = %+v, want INTROSPECTION_ERROR", env.Error)
	}
	if env.Success {
		t.Error("success = true, want false")
	}
}

func TestSchemasServerProfileWithoutDatabaseRequiresDatabase(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID:       "srv",
		Name:     "Server",
		Driver:   model.DriverPostgres,
		Host:     strPtr("127.0.0.1"),
		Port:     intPtr(5432),
		Username: strPtr("appuser"),
	})

	rec := httptest.NewRecorder()
	h.Schemas(rec, schemasRequest("srv"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "DATABASE_REQUIRED" {
		t.Errorf("error = %+v, want DATABASE_REQUIRED", env.Error)
	}
}
