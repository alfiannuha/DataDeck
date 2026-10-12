//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// TestMySQLDeleteDatabaseIntegration verifies the T18 two-phase Delete Database
// against a real, explicitly disposable MySQL database.
func TestMySQLDeleteDatabaseIntegration(t *testing.T) {
	host := os.Getenv("DATADECK_TEST_MYSQL_HOST")
	if host == "" {
		t.Skip("DATADECK_TEST_MYSQL_HOST not set; skipping MySQL delete integration test")
	}
	port := 3306
	if raw := os.Getenv("DATADECK_TEST_MYSQL_PORT"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			port = v
		}
	}
	user := envOr("DATADECK_TEST_MYSQL_USER", "root")
	password := os.Getenv("DATADECK_TEST_MYSQL_PASSWORD")

	target := fmt.Sprintf("datadeck_t20_del_%d", rand.Int63())
	h, store, cipher := newTestHandler(t)
	ctx := context.Background()

	encrypted, err := cipher.Encrypt([]byte(password))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// A command profile bound to the protected-but-usable `mysql` database.
	seedProfile(t, store, &model.ConnectionProfile{
		ID: "cmd", Name: "cmd", Driver: model.DriverMySQL,
		Host: &host, Port: &port, Username: &user, DatabaseName: "mysql",
		EncryptedPassword: &encrypted,
	})
	cmdCfg := database.Config{Driver: model.DriverMySQL, Host: host, Port: port, Username: user, Password: password, Database: "mysql", SSLMode: "disable"}

	if _, err := h.manager.Execute(ctx, "cmd", cmdCfg, "CREATE DATABASE `"+target+"`"); err != nil {
		t.Fatalf("create disposable database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = h.manager.Execute(ctx, "cmd", cmdCfg, "DROP DATABASE IF EXISTS `"+target+"`")
	})

	seedProfile(t, store, &model.ConnectionProfile{
		ID: "target", Name: "target", Driver: model.DriverMySQL,
		Host: &host, Port: &port, Username: &user, DatabaseName: target,
		EncryptedPassword: &encrypted,
	})

	// Preflight.
	preRec := httptest.NewRecorder()
	h.DatabaseDeletePreflight(preRec, deletePreflightRequest("target", target))
	if preRec.Code != http.StatusOK {
		t.Fatalf("preflight status = %d, want 200 (body=%s)", preRec.Code, preRec.Body.String())
	}
	var preflight DatabaseDeletePreflight
	if err := json.Unmarshal(decodeEnvelope(t, preRec).Data, &preflight); err != nil {
		t.Fatalf("decode preflight: %v", err)
	}
	if preflight.Database != target || preflight.Token == "" {
		t.Fatalf("preflight = %+v", preflight)
	}

	// Execute with typed name + acknowledgment.
	rec := httptest.NewRecorder()
	h.DatabaseDelete(rec, deleteExecuteRequest("target", target, fmt.Sprintf(
		`{"confirmation_token":%q,"typed_database_name":%q,"acknowledged":true}`, preflight.Token, target)))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var result DatabaseDeleteResult
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Status != "deleted" {
		t.Fatalf("result = %+v", result)
	}

	// Independent verification: the database is gone.
	exists, err := h.manager.DatabaseExists(ctx, "cmd", cmdCfg, target)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if exists {
		t.Fatalf("database %s still exists after delete", target)
	}
}
