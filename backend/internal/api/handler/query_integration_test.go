//go:build integration

package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/security"
)

// TestQueryExecutionIntegration exercises the query and history endpoints
// against a real PostgreSQL server. Opt-in via build tag and
// DATADECK_TEST_PG_HOST (see the database package integration tests).
func TestQueryExecutionIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_PG_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping query API integration test")
	}

	port := 5432
	if raw := os.Getenv("DATADECK_TEST_PG_PORT"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_PG_PORT %q: %v", raw, err)
		}
		port = value
	}
	user := os.Getenv("DATADECK_TEST_PG_USER")
	databaseName := envOr("DATADECK_TEST_PG_DATABASE", "postgres")
	sslMode := envOr("DATADECK_TEST_PG_SSLMODE", "disable")

	h, db := newQueryHandler(t)
	cipher, err := security.NewCipher(testKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	encrypted, err := cipher.Encrypt([]byte(os.Getenv("DATADECK_TEST_PG_PASSWORD")))
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	seedProfile(t, db, &model.ConnectionProfile{
		ID:                "pg",
		Name:              "PG",
		Driver:            model.DriverPostgres,
		Host:              strPtr(host),
		Port:              intPtr(port),
		DatabaseName:      databaseName,
		Username:          strPtr(user),
		EncryptedPassword: &encrypted,
		SSLMode:           sslMode,
	})

	success := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"pg","sql":"SELECT 1 AS one","timeout_seconds":10}`)
	if success.Code != http.StatusOK {
		t.Fatalf("execute status = %d, want 200 (body=%s)", success.Code, success.Body.String())
	}
	if !decodeEnvelope(t, success).Success {
		t.Errorf("execute success = false")
	}

	failure := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"pg","sql":"SELECT WHERRE"}`)
	if failure.Code != http.StatusBadRequest {
		t.Fatalf("syntax error status = %d, want 400 (body=%s)", failure.Code, failure.Body.String())
	}
	if env := decodeEnvelope(t, failure); env.Error == nil || env.Error.Code != "SQL_SYNTAX_ERROR" {
		t.Errorf("syntax error = %+v, want SQL_SYNTAX_ERROR", env.Error)
	}

	history := doRequest(h.History, http.MethodGet, "/api/v1/query/history?connection_id=pg", "")
	if history.Code != http.StatusOK {
		t.Fatalf("history status = %d, want 200", history.Code)
	}
	var records []HistoryRecord
	if err := json.Unmarshal(decodeEnvelope(t, history).Data, &records); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("history len = %d, want at least 2", len(records))
	}
	if records[0].Status != string(model.QueryStatusError) {
		t.Errorf("newest history status = %q, want ERROR", records[0].Status)
	}
}

// TestQueryTimeoutIntegration verifies the timeout path end to end against a
// real PostgreSQL server: a bounded 504, a consistent ERROR history entry, and
// a usable backend afterwards.
func TestQueryTimeoutIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_PG_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_PG_HOST not set; skipping query timeout integration test")
	}
	port := 5432
	if raw := os.Getenv("DATADECK_TEST_PG_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_PG_PORT %q: %v", raw, err)
		}
		port = parsed
	}

	h, db := newQueryHandler(t)
	cipher, err := security.NewCipher(testKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	encrypted, err := cipher.Encrypt([]byte(os.Getenv("DATADECK_TEST_PG_PASSWORD")))
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	seedProfile(t, db, &model.ConnectionProfile{
		ID:                "pgt",
		Name:              "PG timeout",
		Driver:            model.DriverPostgres,
		Host:              strPtr(host),
		Port:              intPtr(port),
		DatabaseName:      envOr("DATADECK_TEST_PG_DATABASE", "postgres"),
		Username:          strPtr(os.Getenv("DATADECK_TEST_PG_USER")),
		EncryptedPassword: &encrypted,
		SSLMode:           envOr("DATADECK_TEST_PG_SSLMODE", "disable"),
	})

	timeout := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"pgt","sql":"SELECT pg_sleep(5)","timeout_seconds":1}`)
	if timeout.Code != http.StatusGatewayTimeout {
		t.Fatalf("timeout status = %d, want 504 (body=%s)", timeout.Code, timeout.Body.String())
	}
	if env := decodeEnvelope(t, timeout); env.Error == nil || env.Error.Code != "QUERY_TIMEOUT" {
		t.Errorf("timeout error = %+v, want QUERY_TIMEOUT", env.Error)
	}

	history := doRequest(h.History, http.MethodGet, "/api/v1/query/history?connection_id=pgt", "")
	var records []HistoryRecord
	if err := json.Unmarshal(decodeEnvelope(t, history).Data, &records); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(records) == 0 || records[0].Status != string(model.QueryStatusError) {
		t.Fatalf("timeout history = %+v, want a newest ERROR entry", records)
	}
	if records[0].ErrorMessage == nil || *records[0].ErrorMessage != "query timeout" {
		t.Errorf("timeout history message = %v, want \"query timeout\"", records[0].ErrorMessage)
	}

	// The connection remains usable after a timed-out query.
	if rec := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"pgt","sql":"SELECT 1"}`); rec.Code != http.StatusOK {
		t.Fatalf("post-timeout status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
