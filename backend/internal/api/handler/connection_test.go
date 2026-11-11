package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/security"
	"github.com/datadeck/datadeck/backend/internal/storage"
)

const testKey = "0123456789abcdef0123456789abcdef"

const validBody = `{"name":"Local PG","driver":"postgres","host":"127.0.0.1","port":5432,` +
	`"database_name":"app","username":"appuser","password":"s3cret","ssl_mode":"disable"}`

func newTestHandler(t *testing.T) (*ConnectionHandler, *sql.DB, *security.Cipher) {
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
	h := NewConnectionHandler(repo, manager, cipher, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return h, store.DB(), cipher
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *apiError       `json:"error"`
	Meta    map[string]any  `json:"meta"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v (body=%s)", err, rec.Body.String())
	}
	if env.Meta == nil {
		t.Errorf("envelope meta is nil, want {} (body=%s)", rec.Body.String())
	}
	return env
}

func doRequest(h func(http.ResponseWriter, *http.Request), method, target, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}
	h(rec, req)
	return rec
}

func TestListEmptyConnections(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := doRequest(h.List, http.MethodGet, "/api/v1/connections", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	env := decodeEnvelope(t, rec)
	if !env.Success {
		t.Error("success = false, want true")
	}
	if string(env.Data) != "[]" {
		t.Errorf("data = %s, want []", env.Data)
	}
	if env.Error != nil {
		t.Errorf("error = %+v, want nil", env.Error)
	}
}

func TestCreateProfile(t *testing.T) {
	h, db, cipher := newTestHandler(t)

	rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", validBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if !env.Success {
		t.Fatalf("success = false: %s", rec.Body.String())
	}

	var profile ConnectionResponse
	if err := json.Unmarshal(env.Data, &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.ID == "" || profile.Name != "Local PG" || profile.Driver != "postgres" {
		t.Errorf("unexpected profile: %+v", profile)
	}
	if profile.Host == nil || *profile.Host != "127.0.0.1" || profile.Port == nil || *profile.Port != 5432 {
		t.Errorf("unexpected host/port: %+v", profile)
	}
	if profile.SSLMode != "disable" {
		t.Errorf("ssl_mode = %q", profile.SSLMode)
	}

	// Response must never expose credentials.
	if strings.Contains(rec.Body.String(), "s3cret") {
		t.Error("response leaked the plaintext password")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		t.Fatalf("decode raw profile: %v", err)
	}
	for _, key := range []string{"password", "encrypted_password", "ssh_encrypted_password"} {
		if _, ok := raw[key]; ok {
			t.Errorf("response contains forbidden field %q", key)
		}
	}

	// Persisted value must be ciphertext, and must decrypt back.
	repo := repository.NewConnectionRepository(db)
	stored, err := repo.Get(context.Background(), profile.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.EncryptedPassword == nil {
		t.Fatal("EncryptedPassword is nil, want ciphertext")
	}
	if *stored.EncryptedPassword == "s3cret" {
		t.Fatal("password was persisted in plaintext")
	}
	plaintext, err := cipher.Decrypt(*stored.EncryptedPassword)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if string(plaintext) != "s3cret" {
		t.Errorf("decrypted password = %q, want s3cret", plaintext)
	}
}

func TestCreateValidationErrors(t *testing.T) {
	h, _, _ := newTestHandler(t)

	cases := map[string]string{
		"missing name":           `{"driver":"postgres","host":"h","database_name":"d","username":"u"}`,
		"missing driver":         `{"name":"n","host":"h","database_name":"d","username":"u"}`,
		"unsupported oracle":     `{"name":"n","driver":"oracle","host":"h","database_name":"d","username":"u"}`,
		"missing host":           `{"name":"n","driver":"postgres","database_name":"d","username":"u"}`,
		"mysql missing database": `{"name":"n","driver":"mysql","host":"h","username":"u"}`,
		"missing username":       `{"name":"n","driver":"postgres","host":"h","database_name":"d"}`,
		"invalid port":           `{"name":"n","driver":"postgres","host":"h","port":70000,"database_name":"d","username":"u"}`,
		"invalid ssl mode":       `{"name":"n","driver":"postgres","host":"h","database_name":"d","username":"u","ssl_mode":"bogus"}`,
		"malformed json":         `{`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
			env := decodeEnvelope(t, rec)
			if env.Success {
				t.Error("success = true, want false")
			}
			if env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
				t.Errorf("error = %+v, want VALIDATION_ERROR", env.Error)
			}
			if string(env.Data) != "null" {
				t.Errorf("data = %s, want null", env.Data)
			}
		})
	}
}

func TestCreateMySQLConnectionDefaultsPort(t *testing.T) {
	h, db, _ := newTestHandler(t)

	rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections",
		`{"name":"MySQL","driver":"mysql","host":"127.0.0.1","database_name":"app","username":"u","password":"secret"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var profile ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.Driver != "mysql" {
		t.Errorf("driver = %q, want mysql", profile.Driver)
	}
	if profile.Port == nil || *profile.Port != 3306 {
		t.Errorf("port = %v, want default 3306", profile.Port)
	}

	stored, err := repository.NewConnectionRepository(db).Get(context.Background(), profile.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.EncryptedPassword == nil || *stored.EncryptedPassword == "secret" {
		t.Error("password was not stored as ciphertext")
	}
}

func TestCreateUnsupportedDriverMessage(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections",
		`{"name":"n","driver":"oracle","host":"h","database_name":"d","username":"u"}`)
	env := decodeEnvelope(t, rec)
	if env.Error == nil || !strings.Contains(env.Error.Message, "not supported") {
		t.Errorf("error message = %+v, want it to state the driver is unsupported", env.Error)
	}
}

func TestTestConnectionFailure(t *testing.T) {
	h, db, _ := newTestHandler(t)

	body := `{"driver":"postgres","host":"127.0.0.1","port":1,"database_name":"app","username":"u","password":"p","ssl_mode":"disable"}`
	rec := doRequest(h.Test, http.MethodPost, "/api/v1/connections/test", body)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Success {
		t.Error("success = true, want false")
	}
	if env.Error == nil || env.Error.Code != "CONNECTION_ERROR" {
		t.Errorf("error = %+v, want CONNECTION_ERROR", env.Error)
	}
	if strings.Contains(rec.Body.String(), `"p"`) {
		t.Error("response may have leaked the password")
	}

	// Nothing may be persisted by a test connection.
	profiles, err := repository.NewConnectionRepository(db).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("test connection persisted %d profiles, want 0", len(profiles))
	}
}

func TestTestConnectionValidation(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := doRequest(h.Test, http.MethodPost, "/api/v1/connections/test",
		`{"driver":"oracle","host":"h","database_name":"d","username":"u"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("error = %+v, want VALIDATION_ERROR", env.Error)
	}
}

func deleteRequest(id string) *http.Request {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections/"+id, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestDeleteProfile(t *testing.T) {
	h, db, _ := newTestHandler(t)
	repo := repository.NewConnectionRepository(db)

	createRec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", validBody)
	var profile ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, createRec).Data, &profile); err != nil {
		t.Fatalf("decode created profile: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Delete(rec, deleteRequest(profile.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if !decodeEnvelope(t, rec).Success {
		t.Errorf("delete envelope success = false")
	}
	if _, err := repo.Get(context.Background(), profile.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("profile still present after delete: %v", err)
	}
}

func TestDeleteUnknownProfile(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := httptest.NewRecorder()
	h.Delete(rec, deleteRequest("does-not-exist"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Errorf("error = %+v, want NOT_FOUND", env.Error)
	}
	if string(env.Data) != "null" {
		t.Errorf("data = %s, want null", env.Data)
	}
}

func TestCreateSQLiteConnectionWithoutCredentials(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	body := fmt.Sprintf(`{"name":"Local file","driver":"sqlite","database_name":%q}`, path)

	rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var profile ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.Driver != "sqlite" {
		t.Errorf("driver = %q, want sqlite", profile.Driver)
	}
	if profile.Host != nil || profile.Port != nil || profile.Username != nil {
		t.Errorf("sqlite must not require host/port/username: %+v", profile)
	}
	stored, err := repository.NewConnectionRepository(db).Get(context.Background(), profile.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.EncryptedPassword != nil {
		t.Error("sqlite profile should not store a password")
	}
}

func TestTestSQLiteConnection(t *testing.T) {
	h, _, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")

	rec := doRequest(h.Test, http.MethodPost, "/api/v1/connections/test",
		fmt.Sprintf(`{"driver":"sqlite","database_name":%q}`, path))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if !decodeEnvelope(t, rec).Success {
		t.Error("sqlite test connection success = false")
	}
}

// PRF-01: PostgreSQL profiles may omit the database (server-level profile).
func TestCreatePostgresProfileWithoutDatabase(t *testing.T) {
	h, db, _ := newTestHandler(t)

	rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections",
		`{"name":"Server only","driver":"postgres","host":"127.0.0.1","port":5432,"username":"u","password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var created ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &created); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if created.DatabaseName != "" {
		t.Errorf("database_name = %q, want empty", created.DatabaseName)
	}

	stored, err := repository.NewConnectionRepository(db).Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.DatabaseName != "" {
		t.Errorf("stored database_name = %q, want empty", stored.DatabaseName)
	}
	if stored.EncryptedPassword == nil || *stored.EncryptedPassword == "pw" {
		t.Error("password was not stored as ciphertext")
	}
}

// PRF-01: an existing PostgreSQL profile that carries a database keeps working
// unchanged (the value acts as the default database).
func TestCreatePostgresProfileWithDatabasePreserved(t *testing.T) {
	h, db, cipher := newTestHandler(t)

	rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections",
		`{"name":"Legacy","driver":"postgres","host":"127.0.0.1","port":5432,"database_name":"CCM","username":"u","password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var created ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &created); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if created.DatabaseName != "CCM" {
		t.Errorf("database_name = %q, want CCM", created.DatabaseName)
	}

	stored, err := repository.NewConnectionRepository(db).Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.DatabaseName != "CCM" {
		t.Errorf("stored database_name = %q, want preserved CCM", stored.DatabaseName)
	}
	plaintext, err := cipher.Decrypt(*stored.EncryptedPassword)
	if err != nil || string(plaintext) != "pw" {
		t.Fatalf("credential did not round-trip: %q, %v", plaintext, err)
	}
}

// PRF-01: /test accepts a PostgreSQL profile without a database and probes the
// bootstrap candidates; when none is reachable it reports an explicit bootstrap
// failure (never a validation error).
func TestTestPostgresConnectionWithoutDatabase(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := doRequest(h.Test, http.MethodPost, "/api/v1/connections/test",
		`{"driver":"postgres","host":"127.0.0.1","port":1,"username":"u","password":"pw"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "BOOTSTRAP_DATABASE_UNAVAILABLE" {
		t.Errorf("error = %+v, want BOOTSTRAP_DATABASE_UNAVAILABLE", env.Error)
	}
}

// PRF-03/A: the color is persisted on create and echoed (lowercased) in the
// response; a profile created without a color stays empty for backward compat.
func TestCreateConnectionPersistsColor(t *testing.T) {
	h, db, _ := newTestHandler(t)

	rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections",
		`{"name":"Colored","driver":"postgres","host":"127.0.0.1","port":5432,`+
			`"database_name":"app","username":"u","password":"p","color":"#3B82F6"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var profile ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.Color != "#3b82f6" {
		t.Errorf("color = %q, want normalized #3b82f6", profile.Color)
	}

	stored, err := repository.NewConnectionRepository(db).Get(context.Background(), profile.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Color != "#3b82f6" {
		t.Errorf("stored color = %q, want #3b82f6", stored.Color)
	}

	plain := doRequest(h.Create, http.MethodPost, "/api/v1/connections",
		`{"name":"Plain","driver":"postgres","host":"127.0.0.1","port":5432,"database_name":"app","username":"u"}`)
	var plainProfile ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, plain).Data, &plainProfile); err != nil {
		t.Fatalf("decode plain profile: %v", err)
	}
	if plainProfile.Color != "" {
		t.Errorf("color without a value = %q, want empty", plainProfile.Color)
	}
}

func TestCreateConnectionRejectsInvalidColor(t *testing.T) {
	h, _, _ := newTestHandler(t)

	for _, color := range []string{"blue", "#12345", "#GGGGGG", "3b82f6"} {
		t.Run(color, func(t *testing.T) {
			rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections",
				`{"name":"n","driver":"postgres","host":"h","port":5432,"database_name":"d","username":"u","color":"`+color+`"}`)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
			if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
				t.Errorf("error = %+v, want VALIDATION_ERROR", env.Error)
			}
		})
	}
}

func updateRequest(id, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/connections/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// PRF-03/B: editing a profile keeps the stored password when none is supplied,
// updates the other fields, and preserves created_at.
func TestUpdateConnectionPreservesPasswordAndUpdatesFields(t *testing.T) {
	h, db, _ := newTestHandler(t)
	repo := repository.NewConnectionRepository(db)

	createRec := doRequest(h.Create, http.MethodPost, "/api/v1/connections",
		`{"name":"Before","driver":"postgres","host":"127.0.0.1","port":5432,`+
			`"database_name":"app","username":"u","password":"original","color":"#3b82f6"}`)
	var created ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, createRec).Data, &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	before, err := repo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() before error = %v", err)
	}

	// Password omitted -> keep the ciphertext; rename + recolor.
	rec := httptest.NewRecorder()
	h.Update(rec, updateRequest(created.ID,
		`{"name":"After","driver":"postgres","host":"127.0.0.1","port":5432,`+
			`"database_name":"app","username":"u","color":"#ef4444"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var updated ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.Name != "After" || updated.Color != "#ef4444" {
		t.Errorf("updated = %q/%q, want After/#ef4444", updated.Name, updated.Color)
	}

	after, err := repo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() after error = %v", err)
	}
	if before.EncryptedPassword == nil || after.EncryptedPassword == nil ||
		*before.EncryptedPassword != *after.EncryptedPassword {
		t.Error("password ciphertext changed when no new password was supplied")
	}
	if !after.CreatedAt.Equal(before.CreatedAt) {
		t.Error("created_at was not preserved across update")
	}
}

// A supplied password is re-encrypted (not stored verbatim) and replaces the old
// ciphertext.
func TestUpdateConnectionReplacesPasswordWhenProvided(t *testing.T) {
	h, db, cipher := newTestHandler(t)
	repo := repository.NewConnectionRepository(db)

	createRec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", validBody)
	var created ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, createRec).Data, &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	before, err := repo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() before error = %v", err)
	}

	rec := httptest.NewRecorder()
	h.Update(rec, updateRequest(created.ID,
		`{"name":"Local PG","driver":"postgres","host":"127.0.0.1","port":5432,`+
			`"database_name":"app","username":"appuser","password":"rotated"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	after, err := repo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() after error = %v", err)
	}
	if after.EncryptedPassword == nil || *after.EncryptedPassword == "rotated" {
		t.Error("new password was not stored as ciphertext")
	}
	if before.EncryptedPassword != nil && *before.EncryptedPassword == *after.EncryptedPassword {
		t.Error("password ciphertext did not change")
	}
	plaintext, err := cipher.Decrypt(*after.EncryptedPassword)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(plaintext) != "rotated" {
		t.Errorf("decrypted password = %q, want rotated", plaintext)
	}
}

func TestUpdateConnectionValidationAndNotFound(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := httptest.NewRecorder()
	h.Update(rec, updateRequest("missing", validBody))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
	}

	createRec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", validBody)
	var created ConnectionResponse
	if err := json.Unmarshal(decodeEnvelope(t, createRec).Data, &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	bad := updateRequest(created.ID,
		`{"name":"n","driver":"postgres","host":"h","port":5432,"database_name":"d","username":"u","color":"nope"}`)
	rec = httptest.NewRecorder()
	h.Update(rec, bad)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid color status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
}
