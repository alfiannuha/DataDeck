package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func reconnectRequest(id, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/"+id+"/reconnect", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return withPathParams(req, "id", id)
}

func TestReconnectSQLite(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "c1",
		Name:         "SQLite target",
		Driver:       model.DriverSQLite,
		DatabaseName: path,
	})

	rec := httptest.NewRecorder()
	h.Reconnect(rec, reconnectRequest("c1", `{}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp ReconnectResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "ok" || resp.Database != path {
		t.Errorf("reconnect response = %+v, want status ok and database %q", resp, path)
	}
	if _, ok := h.manager.Get("c1", path); !ok {
		t.Error("reconnect did not establish a pool for the target")
	}
}

func TestReconnectUnknownProfile(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := httptest.NewRecorder()
	h.Reconnect(rec, reconnectRequest("missing", `{}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Errorf("error = %+v, want NOT_FOUND", env.Error)
	}
}

// A server-level PostgreSQL profile has no default database; reconnect without
// an explicit database must be rejected, never silently resolved elsewhere.
func TestReconnectServerLevelPostgresRequiresDatabase(t *testing.T) {
	h, db, _ := newTestHandler(t)
	host := "127.0.0.1"
	port := 5432
	user := "u"
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "srv", Name: "Server", Driver: model.DriverPostgres,
		Host: &host, Port: &port, Username: &user, DatabaseName: "",
	})

	rec := httptest.NewRecorder()
	h.Reconnect(rec, reconnectRequest("srv", `{}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "DATABASE_REQUIRED" {
		t.Errorf("error = %+v, want DATABASE_REQUIRED", env.Error)
	}
}

// MySQL is bound to one database; a mismatched target is rejected, never
// redirected to another database.
func TestReconnectMySQLDatabaseMismatch(t *testing.T) {
	h, db, _ := newTestHandler(t)
	host := "127.0.0.1"
	port := 3306
	user := "u"
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "my", Name: "MySQL", Driver: model.DriverMySQL,
		Host: &host, Port: &port, Username: &user, DatabaseName: "app",
	})

	rec := httptest.NewRecorder()
	h.Reconnect(rec, reconnectRequest("my", `{"database":"other"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("error = %+v, want VALIDATION_ERROR", env.Error)
	}
}

// An unreachable target is a sanitized connection failure (never a validation
// error, never a credential leak).
func TestReconnectConnectionFailureIsSanitized(t *testing.T) {
	h, db, cipher := newTestHandler(t)
	host := "127.0.0.1"
	port := 1
	user := "u"
	secret, err := cipher.Encrypt([]byte("sup3r-secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "pg", Name: "PG", Driver: model.DriverPostgres,
		Host: &host, Port: &port, Username: &user, DatabaseName: "app",
		EncryptedPassword: &secret,
	})

	rec := httptest.NewRecorder()
	h.Reconnect(rec, reconnectRequest("pg", `{}`))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "CONNECTION_ERROR" {
		t.Errorf("error = %+v, want CONNECTION_ERROR", env.Error)
	}
	if strings.Contains(rec.Body.String(), "sup3r-secret") {
		t.Error("reconnect error leaked the password")
	}
}

func TestReconnectInvalidBody(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := httptest.NewRecorder()
	h.Reconnect(rec, reconnectRequest("c1", `{`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("error = %+v, want VALIDATION_ERROR", env.Error)
	}
}
