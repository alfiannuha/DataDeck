//go:build integration

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/security"
)

// TestDatabaseAwareAPIIntegration exercises PRF-01 database-aware API behaviour
// against a real PostgreSQL server.
func TestDatabaseAwareAPIIntegration(t *testing.T) {
	host := envOr("DATADECK_TEST_PG_HOST", "")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping database-aware API integration test")
	}

	h, db, cipher := newTestHandler(t)
	// Share the same store and manager options as the connection handler so the
	// seeded profiles are visible to the query handler.
	queryH := NewQueryHandler(
		repository.NewConnectionRepository(db),
		repository.NewQueryHistoryRepository(db),
		database.NewManager(database.DefaultOptions(), database.Postgres{}, database.MySQL{}, database.SQLite{}),
		cipher,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	const (
		dbA  = "datadeck_api_alpha"
		dbB  = "datadeck_api_beta"
		role = "dd_api_limited"
	)

	pgConfig := database.Config{
		Driver:   model.DriverPostgres,
		Host:     host,
		Port:     5432,
		Database: envOr("DATADECK_TEST_PG_DATABASE", "postgres"),
		Username: envOr("DATADECK_TEST_PG_USER", ""),
		Password: envOr("DATADECK_TEST_PG_PASSWORD", ""),
		SSLMode:  envOr("DATADECK_TEST_PG_SSLMODE", "disable"),
	}
	if raw := envOr("DATADECK_TEST_PG_PORT", ""); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_PG_PORT %q: %v", raw, err)
		}
		pgConfig.Port = port
	}

	// A dedicated manager with production ping timeouts for fixture/admin work;
	// the handler test manager uses an aggressively short ping timeout.
	adminMgr := database.NewManager(database.DefaultOptions(), database.Postgres{})
	t.Cleanup(func() { _ = adminMgr.CloseAll() })

	admin, err := adminMgr.Open(context.Background(), "admin", pgConfig)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(func() {
		for _, name := range []string{dbA, dbB} {
			_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		}
		_, _ = admin.ExecContext(context.Background(), "DROP OWNED BY "+role+" CASCADE")
		_, _ = admin.ExecContext(context.Background(), "DROP ROLE IF EXISTS "+role)
	})
	for _, name := range []string{dbA, dbB} {
		execAdmin(t, admin, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		execAdmin(t, admin, "CREATE DATABASE "+name)
	}
	tryAdmin(admin, "DROP OWNED BY "+role+" CASCADE")
	tryAdmin(admin, "DROP ROLE IF EXISTS "+role)
	execAdmin(t, admin, "CREATE ROLE "+role+" LOGIN PASSWORD 'api-secret'")
	execAdmin(t, admin, "REVOKE CONNECT ON DATABASE "+dbB+" FROM PUBLIC")
	execAdmin(t, admin, "GRANT CONNECT ON DATABASE "+pgConfig.Database+", "+dbA+" TO "+role)

	makeTable := func(database, table string) {
		cfg := pgConfig
		cfg.Database = database
		pool, err := adminMgr.Open(context.Background(), "seed-"+database, cfg)
		if err != nil {
			t.Fatalf("open %s: %v", database, err)
		}
		if _, err := pool.ExecContext(context.Background(), "CREATE TABLE "+table+" (id int)"); err != nil {
			t.Fatalf("create %s.%s: %v", database, table, err)
		}
		_ = adminMgr.CloseConnection("seed-" + database)
	}
	makeTable(dbA, "only_in_a")
	makeTable(dbB, "only_in_b")

	// Server-level profile (no default database) and a legacy profile bound to A.
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "srv", Name: "Server", Driver: model.DriverPostgres,
		Host: strPtr(host), Port: intPtr(pgConfig.Port), DatabaseName: "",
		Username: strPtr(pgConfig.Username), EncryptedPassword: encryptedOrNil(t, cipher, pgConfig.Password), SSLMode: "disable",
	})
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "legacy", Name: "Legacy", Driver: model.DriverPostgres,
		Host: strPtr(host), Port: intPtr(pgConfig.Port), DatabaseName: dbA,
		Username: strPtr(pgConfig.Username), EncryptedPassword: encryptedOrNil(t, cipher, pgConfig.Password), SSLMode: "disable",
	})
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "my", Name: "MySQL", Driver: model.DriverMySQL,
		Host: strPtr(host), Port: intPtr(pgConfig.Port), DatabaseName: "datadeck_test",
		Username: strPtr("root"), SSLMode: "disable",
	})

	t.Run("list databases", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.Databases(rec, databasesRequest("srv"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		env := decodeEnvelope(t, rec)
		if env.Error != nil {
			t.Fatalf("error = %+v", env.Error)
		}
		var names []string
		var items []model.DatabaseInfo
		if err := json.Unmarshal(env.Data, &items); err != nil {
			t.Fatalf("decode databases: %v", err)
		}
		for _, item := range items {
			names = append(names, item.Name)
		}
		joined := strings.Join(names, ",")
		if !strings.Contains(joined, dbA) || !strings.Contains(joined, dbB) {
			t.Errorf("databases = %v, want %s and %s", names, dbA, dbB)
		}
	})

	t.Run("discovery unsupported for mysql", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.Databases(rec, databasesRequest("my"))
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("status = %d, want 501 (body=%s)", rec.Code, rec.Body.String())
		}
		if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "NOT_IMPLEMENTED" {
			t.Errorf("error = %+v, want NOT_IMPLEMENTED", env.Error)
		}
	})

	t.Run("unknown connection", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.Databases(rec, databasesRequest("missing"))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("schema per database", func(t *testing.T) {
		reqA := schemasRequest("srv")
		reqA.URL.RawQuery = "database=" + dbA
		reqB := schemasRequest("srv")
		reqB.URL.RawQuery = "database=" + dbB
		recA := httptest.NewRecorder()
		h.Schemas(recA, reqA)
		recB := httptest.NewRecorder()
		h.Schemas(recB, reqB)
		if recA.Code != http.StatusOK || recB.Code != http.StatusOK {
			t.Fatalf("schema status A=%d B=%d (A=%s B=%s)", recA.Code, recB.Code, recA.Body.String(), recB.Body.String())
		}
		bodyA, bodyB := recA.Body.String(), recB.Body.String()
		if !strings.Contains(bodyA, "only_in_a") || strings.Contains(bodyA, "only_in_b") {
			t.Errorf("schema A mismatch: %s", bodyA)
		}
		if !strings.Contains(bodyB, "only_in_b") || strings.Contains(bodyB, "only_in_a") {
			t.Errorf("schema B mismatch: %s", bodyB)
		}
	})

	t.Run("schema requires a database on a server-level profile", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.Schemas(rec, schemasRequest("srv"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
		}
		if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
			t.Errorf("error = %+v, want VALIDATION_ERROR", env.Error)
		}
	})

	exec := func(connection, database, sqlText string) (int, string, string) {
		body := `{"connection_id":"` + connection + `","sql":"` + strings.ReplaceAll(sqlText, `"`, `\"`) + `"`
		if database != "" {
			body += `,"database":"` + database + `"`
		}
		body += `}`
		rec := doRequest(queryH.Execute, http.MethodPost, "/api/v1/query/execute", body)
		var env struct {
			Data  model.QueryResult `json:"data"`
			Error *apiError         `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		code := ""
		if env.Error != nil {
			code = env.Error.Code
		}
		value := ""
		if len(env.Data.Rows) > 0 && len(env.Data.Rows[0]) > 0 {
			if s, ok := env.Data.Rows[0][0].(string); ok {
				value = s
			}
		}
		return rec.Code, code, value
	}

	t.Run("query per database with isolation", func(t *testing.T) {
		statusA, codeA, valueA := exec("srv", dbA, "SELECT current_database()")
		if statusA != http.StatusOK || valueA != dbA {
			t.Fatalf("query A: status=%d code=%s value=%q want %q", statusA, codeA, valueA, dbA)
		}
		statusB, codeB, valueB := exec("srv", dbB, "SELECT current_database()")
		if statusB != http.StatusOK || valueB != dbB {
			t.Fatalf("query B: status=%d code=%s value=%q want %q", statusB, codeB, valueB, dbB)
		}
		if valueA == valueB {
			t.Error("databases were not isolated")
		}
	})

	t.Run("query requires a database on a server-level profile", func(t *testing.T) {
		status, code, _ := exec("srv", "", "SELECT 1")
		if status != http.StatusBadRequest || code != "VALIDATION_ERROR" {
			t.Fatalf("status=%d code=%s, want 400 VALIDATION_ERROR", status, code)
		}
	})

	t.Run("legacy profile uses its database by default", func(t *testing.T) {
		status, code, value := exec("legacy", "", "SELECT current_database()")
		if status != http.StatusOK || value != dbA {
			t.Fatalf("legacy query: status=%d code=%s value=%q want %q", status, code, value, dbA)
		}
	})

	t.Run("unknown database is sanitized", func(t *testing.T) {
		status, code, _ := exec("srv", "datadeck_does_not_exist", "SELECT 1")
		if status != http.StatusBadRequest || code != "DATABASE_NOT_FOUND" {
			t.Fatalf("status=%d code=%s, want 400 DATABASE_NOT_FOUND", status, code)
		}
	})

	t.Run("CONNECT denied is sanitized", func(t *testing.T) {
		seedProfile(t, db, &model.ConnectionProfile{
			ID: "limited", Name: "Limited", Driver: model.DriverPostgres,
			Host: strPtr(host), Port: intPtr(pgConfig.Port), DatabaseName: "",
			Username: strPtr(role), EncryptedPassword: encryptedOrNil(t, cipher, "api-secret"), SSLMode: "disable",
		})
		status, code, _ := exec("limited", dbB, "SELECT 1")
		if status != http.StatusBadRequest || code != "DATABASE_CONNECT_DENIED" {
			t.Fatalf("status=%d code=%s, want 400 DATABASE_CONNECT_DENIED", status, code)
		}
	})

	t.Run("mysql rejects a mismatched database", func(t *testing.T) {
		status, code, _ := exec("my", "other_db", "SELECT 1")
		if status != http.StatusBadRequest || code != "VALIDATION_ERROR" {
			t.Fatalf("status=%d code=%s, want 400 VALIDATION_ERROR", status, code)
		}
	})
}

func databasesRequest(id string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections/"+id+"/databases", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func execAdmin(t *testing.T, admin *sql.DB, sqlText string) {
	t.Helper()
	if _, err := admin.ExecContext(context.Background(), sqlText); err != nil {
		t.Fatalf("admin exec %q: %v", sqlText, err)
	}
}

// tryAdmin runs best-effort cleanup SQL (e.g. dropping a role that may not
// exist) without failing the test.
func tryAdmin(admin *sql.DB, sqlText string) {
	_, _ = admin.ExecContext(context.Background(), sqlText)
}

func encryptedOrNil(t *testing.T, cipher *security.Cipher, plaintext string) *string {
	t.Helper()
	if plaintext == "" {
		return nil
	}
	value, err := cipher.Encrypt([]byte(plaintext))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return &value
}
