package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func deletePreflightRequest(id, database string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/"+id+"/databases/"+database+"/delete-preflight", nil)
	return withPathParams(req, "id", id, "database", database)
}

func deleteExecuteRequest(id, database, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/"+id+"/databases/"+database+"/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return withPathParams(req, "id", id, "database", database)
}

func TestDatabaseDeletePreflightSQLiteUnsupported(t *testing.T) {
	h, store, _ := newTestHandler(t)
	seedProfile(t, store, &model.ConnectionProfile{
		ID: "lit", Name: "sqlite", Driver: model.DriverSQLite, DatabaseName: "/tmp/lit.db",
	})
	rec := httptest.NewRecorder()
	h.DatabaseDeletePreflight(rec, deletePreflightRequest("lit", "lit.db"))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "DATABASE_DELETE_UNSUPPORTED" {
		t.Errorf("error = %+v", env.Error)
	}
}

func TestDatabaseDeletePreflightProtectedPostgres(t *testing.T) {
	h, store, _ := newTestHandler(t)
	host, port, user := "127.0.0.1", 1, "u"
	seedProfile(t, store, &model.ConnectionProfile{
		ID: "pg", Name: "pg", Driver: model.DriverPostgres, Host: &host, Port: &port, Username: &user, DatabaseName: "",
	})
	rec := httptest.NewRecorder()
	h.DatabaseDeletePreflight(rec, deletePreflightRequest("pg", "template1"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "DATABASE_DELETE_PROTECTED" {
		t.Errorf("error = %+v", env.Error)
	}
}

func TestDatabaseDeletePreflightProtectedMySQL(t *testing.T) {
	h, store, _ := newTestHandler(t)
	host, port, user := "127.0.0.1", 3306, "u"
	seedProfile(t, store, &model.ConnectionProfile{
		ID: "my", Name: "my", Driver: model.DriverMySQL, Host: &host, Port: &port, Username: &user, DatabaseName: "mysql",
	})
	// The profile is bound to "mysql"; requesting it must be protected.
	rec := httptest.NewRecorder()
	h.DatabaseDeletePreflight(rec, deletePreflightRequest("my", "mysql"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "DATABASE_DELETE_PROTECTED" {
		t.Errorf("error = %+v", env.Error)
	}
}

func TestDatabaseDeleteExecuteRejectsUnknownToken(t *testing.T) {
	h, store, _ := newTestHandler(t)
	host, port, user := "127.0.0.1", 1, "u"
	seedProfile(t, store, &model.ConnectionProfile{
		ID: "pg", Name: "pg", Driver: model.DriverPostgres, Host: &host, Port: &port, Username: &user,
	})
	rec := httptest.NewRecorder()
	h.DatabaseDelete(rec, deleteExecuteRequest("pg", "alpha",
		`{"confirmation_token":"nope","typed_database_name":"alpha","acknowledged":true}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "DATABASE_DELETE_CONFIRMATION_INVALID" {
		t.Errorf("error = %+v", env.Error)
	}
}

func TestDatabaseDeleteExecuteRejectsBadConfirmation(t *testing.T) {
	h, store, _ := newTestHandler(t)
	host, port, user := "127.0.0.1", 1, "u"
	seedProfile(t, store, &model.ConnectionProfile{
		ID: "pg", Name: "pg", Driver: model.DriverPostgres, Host: &host, Port: &port, Username: &user,
	})
	inject := func(database string) {
		h.deleteMu.Lock()
		h.deleteTokens["tok-"+database] = &deleteToken{connectionID: "pg", database: database, engine: "postgres", createdAt: time.Now()}
		h.deleteMu.Unlock()
	}

	cases := map[string]string{
		"wrong typed name": `{"confirmation_token":"tok-alpha","typed_database_name":"ALPHA","acknowledged":true}`,
		"missing ack":      `{"confirmation_token":"tok-alpha","typed_database_name":"alpha","acknowledged":false}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			inject("alpha")
			rec := httptest.NewRecorder()
			h.DatabaseDelete(rec, deleteExecuteRequest("pg", "alpha", body))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
			if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "DATABASE_DELETE_CONFIRMATION_INVALID" {
				t.Errorf("error = %+v", env.Error)
			}
		})
	}
}

// A token for alpha must not authorize beta.
func TestDatabaseDeleteExecuteRejectsTokenSubstitution(t *testing.T) {
	h, store, _ := newTestHandler(t)
	host, port, user := "127.0.0.1", 1, "u"
	seedProfile(t, store, &model.ConnectionProfile{
		ID: "pg", Name: "pg", Driver: model.DriverPostgres, Host: &host, Port: &port, Username: &user,
	})
	h.deleteMu.Lock()
	h.deleteTokens["tok-alpha"] = &deleteToken{connectionID: "pg", database: "alpha", engine: "postgres", createdAt: time.Now()}
	h.deleteMu.Unlock()

	rec := httptest.NewRecorder()
	h.DatabaseDelete(rec, deleteExecuteRequest("pg", "beta",
		`{"confirmation_token":"tok-alpha","typed_database_name":"beta","acknowledged":true}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "DATABASE_DELETE_CONFIRMATION_INVALID" {
		t.Errorf("error = %+v", env.Error)
	}
}

// A valid token for a protected database is still rejected before any DROP.
func TestDatabaseDeleteExecuteRejectsProtected(t *testing.T) {
	h, store, _ := newTestHandler(t)
	host, port, user := "127.0.0.1", 1, "u"
	seedProfile(t, store, &model.ConnectionProfile{
		ID: "pg", Name: "pg", Driver: model.DriverPostgres, Host: &host, Port: &port, Username: &user,
	})
	h.deleteMu.Lock()
	h.deleteTokens["tok-pg"] = &deleteToken{connectionID: "pg", database: "postgres", engine: "postgres", createdAt: time.Now()}
	h.deleteMu.Unlock()

	rec := httptest.NewRecorder()
	h.DatabaseDelete(rec, deleteExecuteRequest("pg", "postgres",
		`{"confirmation_token":"tok-pg","typed_database_name":"postgres","acknowledged":true}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "DATABASE_DELETE_PROTECTED" {
		t.Errorf("error = %+v", env.Error)
	}
}

// A token is single-use: the second execute is rejected.
func TestDatabaseDeleteExecuteTokenSingleUse(t *testing.T) {
	h, store, _ := newTestHandler(t)
	host, port, user := "127.0.0.1", 1, "u"
	seedProfile(t, store, &model.ConnectionProfile{
		ID: "pg", Name: "pg", Driver: model.DriverPostgres, Host: &host, Port: &port, Username: &user,
	})
	h.deleteMu.Lock()
	h.deleteTokens["tok-once"] = &deleteToken{connectionID: "pg", database: "alpha", engine: "postgres", createdAt: time.Now()}
	h.deleteMu.Unlock()

	body := `{"confirmation_token":"tok-once","typed_database_name":"alpha","acknowledged":true}`
	// First execute consumes the token (and then fails to reach the server).
	first := httptest.NewRecorder()
	h.DatabaseDelete(first, deleteExecuteRequest("pg", "alpha", body))
	// Second execute must be rejected as invalid.
	second := httptest.NewRecorder()
	h.DatabaseDelete(second, deleteExecuteRequest("pg", "alpha", body))
	if second.Code != http.StatusBadRequest {
		t.Fatalf("second status = %d, want 400 (body=%s)", second.Code, second.Body.String())
	}
	if env := decodeEnvelope(t, second); env.Error == nil || env.Error.Code != "DATABASE_DELETE_CONFIRMATION_INVALID" {
		t.Errorf("error = %+v", env.Error)
	}
}
