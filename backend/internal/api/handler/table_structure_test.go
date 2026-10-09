package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

func tableStructureRequest(id string, query string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections/"+id+"/table-structure"+query, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestTableStructureValidation(t *testing.T) {
	h, db, _ := newTestHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "pg",
		Name:         "PG",
		Driver:       model.DriverPostgres,
		Host:         strPtr("127.0.0.1"),
		Port:         intPtr(5432),
		DatabaseName: "app",
		Username:     strPtr("appuser"),
	})

	cases := []struct {
		name  string
		id    string
		query string
		want  int
	}{
		{"id required", "", "?table=users", http.StatusBadRequest},
		{"table required", "pg", "", http.StatusBadRequest},
		{"schema required for postgres", "pg", "?table=users", http.StatusBadRequest},
		{"unknown profile", "nope", "?table=users", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.TableStructure(rec, tableStructureRequest(tc.id, tc.query))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
			env := decodeEnvelope(t, rec)
			if env.Success || env.Error == nil {
				t.Errorf("envelope = %+v, want error envelope", env)
			}
		})
	}
}

func TestTableStructureServerProfileRequiresDatabase(t *testing.T) {
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
	h.TableStructure(rec, tableStructureRequest("srv", "?table=users&schema=public"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "DATABASE_REQUIRED" {
		t.Errorf("error = %+v, want DATABASE_REQUIRED", env.Error)
	}
}

func TestTableStructureSQLite(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	if _, err := seedSQLiteFixture(path); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "lite",
		Name:         "SQLite target",
		Driver:       model.DriverSQLite,
		DatabaseName: path,
	})

	rec := httptest.NewRecorder()
	h.TableStructure(rec, tableStructureRequest("lite", "?table=users"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if !env.Success {
		t.Fatalf("success = false (body=%s)", rec.Body.String())
	}

	var structure struct {
		Database string `json:"database"`
		Table    string `json:"table"`
		Columns  []struct {
			Name           string `json:"name"`
			PrimaryKey     bool   `json:"primary_key"`
			Insertable     bool   `json:"insertable"`
			Updatable      bool   `json:"updatable"`
			NormalizedType string `json:"normalized_type"`
		} `json:"columns"`
	}
	if err := json.Unmarshal(env.Data, &structure); err != nil {
		t.Fatalf("decode structure: %v", err)
	}
	if structure.Table != "users" || structure.Database != path {
		t.Errorf("structure = table %q database %q", structure.Table, structure.Database)
	}
	if len(structure.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(structure.Columns))
	}
	if !structure.Columns[0].PrimaryKey || structure.Columns[0].Insertable || structure.Columns[0].Updatable {
		t.Errorf("identity pk column flags = %+v", structure.Columns[0])
	}
	if structure.Columns[1].Insertable != true || structure.Columns[1].Updatable != true {
		t.Errorf("plain column flags = %+v", structure.Columns[1])
	}
}

func TestTableStructureNotFound(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	if _, err := seedSQLiteFixture(path); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "lite",
		Name:         "SQLite target",
		Driver:       model.DriverSQLite,
		DatabaseName: path,
	})

	rec := httptest.NewRecorder()
	h.TableStructure(rec, tableStructureRequest("lite", "?table=missing"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "TABLE_NOT_FOUND" {
		t.Errorf("error = %+v, want TABLE_NOT_FOUND", env.Error)
	}
}

func TestTableStructureResponseHidesSecrets(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	if _, err := seedSQLiteFixture(path); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "lite",
		Name:         "SQLite target",
		Driver:       model.DriverSQLite,
		DatabaseName: path,
	})

	rec := httptest.NewRecorder()
	h.TableStructure(rec, tableStructureRequest("lite", "?table=users"))
	for _, secret := range []string{"encrypted_password", `"password"`, "dsn", "sqlite://"} {
		if strings.Contains(strings.ToLower(rec.Body.String()), strings.ToLower(secret)) {
			t.Errorf("response leaks %q", secret)
		}
	}
}

// seedSQLiteFixture creates a fresh SQLite file at path with a users table and
// returns the path.
func seedSQLiteFixture(path string) (string, error) {
	manager := database.NewManager(database.Options{PingTimeout: time.Second}, database.SQLite{})
	db, err := manager.Open(context.Background(), "fixture", database.Config{Driver: model.DriverSQLite, Database: path})
	if err != nil {
		return "", err
	}
	defer func() { _ = manager.CloseAll() }()
	if _, err := db.ExecContext(context.Background(),
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL UNIQUE)`); err != nil {
		return "", err
	}
	return path, nil
}