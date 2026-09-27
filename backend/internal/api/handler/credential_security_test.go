package handler

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/security"
	"github.com/datadeck/datadeck/backend/internal/storage"
)

// credentialProbe is a distinctive value used to detect leaks in storage,
// API responses and logs.
const credentialProbe = "P@ssw0rd-credential-probe-9f3"

// newCredentialHarness wires handlers over a real temporary store, a real
// manager and a logging buffer so credential handling can be asserted end to
// end (storage + API + logs).
func newCredentialHarness(t *testing.T) (
	*ConnectionHandler, *QueryHandler, *sql.DB, *security.Cipher, *bytes.Buffer,
) {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "datadeck.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cipher, err := security.NewCipher(testKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	manager := database.NewManager(database.Options{
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Minute,
		PingTimeout:     300 * time.Millisecond,
	}, database.Postgres{}, database.MySQL{}, database.SQLite{})

	repo := repository.NewConnectionRepository(store.DB())
	history := repository.NewQueryHistoryRepository(store.DB())
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	return NewConnectionHandler(repo, manager, cipher, logger),
		NewQueryHandler(repo, history, manager, cipher, logger),
		store.DB(), cipher, logs
}

// mustEncrypt encrypts a value with the probe credential, failing the test on
// error.
func mustEncrypt(t *testing.T, cipher *security.Cipher) string {
	t.Helper()
	encrypted, err := cipher.Encrypt([]byte(credentialProbe))
	if err != nil {
		t.Fatalf("encrypt probe: %v", err)
	}
	return encrypted
}

func TestStoredCredentialIsEncryptedAndKeyBound(t *testing.T) {
	_, _, db, cipher, _ := newCredentialHarness(t)
	encrypted := mustEncrypt(t, cipher)
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "c1", Name: "PG", Driver: model.DriverPostgres,
		Host: strPtr("127.0.0.1"), Port: intPtr(5432),
		DatabaseName: "app", Username: strPtr("appuser"),
		EncryptedPassword: &encrypted,
	})

	var stored sql.NullString
	if err := db.QueryRow(
		`SELECT encrypted_password FROM connection_profiles WHERE id = 'c1'`,
	).Scan(&stored); err != nil {
		t.Fatalf("read stored credential: %v", err)
	}
	if !stored.Valid {
		t.Fatal("stored credential is NULL, want ciphertext")
	}
	if stored.String == credentialProbe {
		t.Fatal("plaintext password was persisted")
	}
	if strings.Contains(stored.String, credentialProbe) {
		t.Fatal("stored value contains the plaintext password")
	}

	// The ciphertext is decryptable only with the correct key.
	plaintext, err := cipher.Decrypt(stored.String)
	if err != nil {
		t.Fatalf("decrypt with correct key: %v", err)
	}
	if string(plaintext) != credentialProbe {
		t.Fatalf("decrypt = %q, want probe", plaintext)
	}
	other, err := security.NewCipher("fedcba9876543210fedcba9876543210")
	if err != nil {
		t.Fatalf("other cipher: %v", err)
	}
	if _, err := other.Decrypt(stored.String); !errors.Is(err, security.ErrInvalidCiphertext) {
		t.Fatalf("decrypt with wrong key error = %v, want ErrInvalidCiphertext", err)
	}
}

func TestCredentialNeverSerializedInAPIResponses(t *testing.T) {
	connH, _, db, cipher, _ := newCredentialHarness(t)
	encrypted := mustEncrypt(t, cipher)
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "c1", Name: "PG", Driver: model.DriverPostgres,
		Host: strPtr("127.0.0.1"), Port: intPtr(5432),
		DatabaseName: "app", Username: strPtr("appuser"),
		EncryptedPassword: &encrypted,
	})

	list := doRequest(connH.List, http.MethodGet, "/api/v1/connections", "")
	if !strings.Contains(list.Body.String(), "appuser") {
		t.Fatalf("list response missing expected profile: %s", list.Body.String())
	}
	assertNoCredentialLeak(t, "list", list.Body.String(), encrypted)

	create := doRequest(connH.Create, http.MethodPost, "/api/v1/connections", validBody)
	if create.Code != http.StatusOK {
		t.Fatalf("create status = %d: %s", create.Code, create.Body.String())
	}
	if strings.Contains(create.Body.String(), "s3cret") {
		t.Fatalf("create response leaked the password: %s", create.Body.String())
	}
	assertNoCredentialLeak(t, "create", create.Body.String(), "")
}

func TestDecryptFailureIsExplicitAndSafe(t *testing.T) {
	connH, queryH, db, _, logs := newCredentialHarness(t)
	broken := "!!! not a valid ciphertext !!!"
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "c1", Name: "PG", Driver: model.DriverPostgres,
		Host: strPtr("127.0.0.1"), Port: intPtr(5432),
		DatabaseName: "app", Username: strPtr("appuser"),
		EncryptedPassword: &broken,
	})

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"execute": doRequest(queryH.Execute, http.MethodPost, "/api/v1/query/execute",
			`{"connection_id":"c1","sql":"SELECT 1"}`),
		"schemas": schemasRecorder(connH, "c1"),
	} {
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s status = %d, want 500 (body=%s)", name, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "INTERNAL_ERROR") {
			t.Errorf("%s body = %s, want INTERNAL_ERROR", name, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), broken) {
			t.Errorf("%s body leaked the stored credential value", name)
		}
	}
	if logs.String() == "" {
		t.Fatal("expected a failure to be logged")
	}
	if strings.Contains(logs.String(), broken) {
		t.Error("logs leaked the stored credential value")
	}
	if strings.Contains(logs.String(), testKey) {
		t.Error("logs leaked the encryption key")
	}
}

func TestLogsDoNotLeakCredentialsOnSuccessOrFailure(t *testing.T) {
	connH, queryH, db, cipher, logs := newCredentialHarness(t)
	encrypted := mustEncrypt(t, cipher)

	// Success path: a reachable SQLite target whose profile also carries an
	// (unused) encrypted credential.
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "ok", Name: "SQLite", Driver: model.DriverSQLite,
		DatabaseName:      filepath.Join(t.TempDir(), "user.db"),
		EncryptedPassword: &encrypted,
	})
	success := doRequest(queryH.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"ok","sql":"SELECT 1"}`)
	if success.Code != http.StatusOK {
		t.Fatalf("sqlite execute status = %d, want 200: %s", success.Code, success.Body.String())
	}

	// Failure path: an unreachable PostgreSQL host with the same credential.
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "bad", Name: "Unreachable PG", Driver: model.DriverPostgres,
		Host: strPtr("127.0.0.1"), Port: intPtr(1),
		DatabaseName: "app", Username: strPtr("appuser"),
		EncryptedPassword: &encrypted,
	})
	_ = doRequest(queryH.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"bad","sql":"SELECT 1"}`)
	_ = doRequest(connH.Test, http.MethodPost, "/api/v1/connections/test",
		`{"driver":"postgres","host":"127.0.0.1","port":1,"database_name":"app",`+
			`"username":"appuser","password":"`+credentialProbe+`","ssl_mode":"disable"}`)

	out := logs.String()
	if out == "" {
		t.Fatal("expected log output")
	}
	for _, secret := range []string{credentialProbe, encrypted, testKey, "s3cret"} {
		if strings.Contains(out, secret) {
			t.Errorf("logs leaked sensitive value %.12q", secret)
		}
	}
}

func TestFailedConnectionTestDoesNotEchoCredentials(t *testing.T) {
	connH, _, _, _, logs := newCredentialHarness(t)

	body := `{"driver":"postgres","host":"127.0.0.1","port":1,"database_name":"app",` +
		`"username":"appuser","password":"` + credentialProbe + `","ssl_mode":"disable"}`
	rec := doRequest(connH.Test, http.MethodPost, "/api/v1/connections/test", body)
	if rec.Code == http.StatusOK {
		t.Fatalf("expected failure for unreachable host, got 200: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), credentialProbe) {
		t.Errorf("test-connection response leaked the password: %s", rec.Body.String())
	}
	if strings.Contains(logs.String(), credentialProbe) {
		t.Errorf("test-connection logs leaked the password")
	}
}

func schemasRecorder(h *ConnectionHandler, id string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.Schemas(rec, schemasRequest(id))
	return rec
}

// assertNoCredentialLeak fails when a payload contains a credential value or a
// credential-bearing field name.
func assertNoCredentialLeak(t *testing.T, label, payload, ciphertext string) {
	t.Helper()
	if ciphertext != "" && strings.Contains(payload, ciphertext) {
		t.Errorf("%s response leaked the ciphertext", label)
	}
	for _, needle := range []string{"password", "encrypted_password", "passwd"} {
		if strings.Contains(strings.ToLower(payload), needle) {
			t.Errorf("%s response exposed credential field %q: %s", label, needle, payload)
		}
	}
}
