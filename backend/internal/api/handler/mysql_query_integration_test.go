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

// TestQueryExecutionMySQLIntegration verifies the shared query + history
// pipeline against a real MySQL server. Skips (NOT RUN) without
// DATADECK_TEST_MYSQL_HOST.
func TestQueryExecutionMySQLIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_MYSQL_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_MYSQL_HOST not set; skipping MySQL query API integration test")
	}

	port := 3306
	if raw := os.Getenv("DATADECK_TEST_MYSQL_PORT"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid DATADECK_TEST_MYSQL_PORT %q: %v", raw, err)
		}
		port = value
	}
	user := envOr("DATADECK_TEST_MYSQL_USER", "root")
	databaseName := envOr("DATADECK_TEST_MYSQL_DATABASE", "datadeck_test")

	h, db := newQueryHandler(t)
	cipher, err := security.NewCipher(testKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	encrypted, err := cipher.Encrypt([]byte(os.Getenv("DATADECK_TEST_MYSQL_PASSWORD")))
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	seedProfile(t, db, &model.ConnectionProfile{
		ID:                "my",
		Name:              "MySQL",
		Driver:            model.DriverMySQL,
		Host:              strPtr(host),
		Port:              intPtr(port),
		DatabaseName:      databaseName,
		Username:          strPtr(user),
		EncryptedPassword: &encrypted,
		SSLMode:           "disable",
	})

	for _, sql := range []string{
		`DROP TABLE IF EXISTS hist_t`,
		`CREATE TABLE hist_t (id BIGINT PRIMARY KEY, name VARCHAR(32))`,
		`INSERT INTO hist_t VALUES (1, 'a'), (2, 'b')`,
	} {
		body := `{"connection_id":"my","sql":` + jsonString(sql) + `}`
		rec := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("execute %q status = %d (body=%s)", sql, rec.Code, rec.Body.String())
		}
	}

	selectRec := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"my","sql":"SELECT id, name FROM hist_t ORDER BY id"}`)
	if selectRec.Code != http.StatusOK {
		t.Fatalf("select status = %d (body=%s)", selectRec.Code, selectRec.Body.String())
	}
	if !decodeEnvelope(t, selectRec).Success {
		t.Error("select success = false")
	}

	failure := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"my","sql":"SELCT 1"}`)
	if failure.Code != http.StatusBadRequest {
		t.Fatalf("syntax error status = %d, want 400 (body=%s)", failure.Code, failure.Body.String())
	}
	if env := decodeEnvelope(t, failure); env.Error == nil || env.Error.Code != "SQL_SYNTAX_ERROR" {
		t.Errorf("syntax error = %+v, want SQL_SYNTAX_ERROR", env.Error)
	}

	historyRec := doRequest(h.History, http.MethodGet, "/api/v1/query/history?connection_id=my", "")
	if historyRec.Code != http.StatusOK {
		t.Fatalf("history status = %d", historyRec.Code)
	}
	var records []HistoryRecord
	if err := json.Unmarshal(decodeEnvelope(t, historyRec).Data, &records); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(records) < 4 {
		t.Fatalf("history len = %d, want at least 4", len(records))
	}
	if records[0].Status != string(model.QueryStatusError) {
		t.Errorf("newest history status = %q, want ERROR", records[0].Status)
	}
	successes := 0
	for _, record := range records {
		if record.Status == string(model.QueryStatusSuccess) {
			successes++
		}
	}
	if successes == 0 {
		t.Error("expected at least one SUCCESS history record")
	}
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
