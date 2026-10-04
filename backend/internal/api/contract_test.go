package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/config"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/security"
	"github.com/datadeck/datadeck/backend/internal/storage"
)

const contractKey = "0123456789abcdef0123456789abcdef"

// swaggerPath is relative to the package directory (internal/api).
const swaggerPath = "../../docs/swagger.json"

func loadSwagger(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(swaggerPath)
	if err != nil {
		t.Fatalf("read %s: %v (regenerate with: cd backend && go run github.com/swaggo/swag/v2/cmd/swag@latest init -g cmd/server/main.go -o docs --parseInternal --outputTypes json,yaml --v3.1)", swaggerPath, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("swagger.json is not valid JSON: %v", err)
	}
	return doc
}

func TestSwaggerSpecMatchesImplementation(t *testing.T) {
	raw, err := os.ReadFile(swaggerPath)
	if err != nil {
		t.Fatalf("read swagger: %v", err)
	}

	// Security audit: no credential representations may appear anywhere.
	for _, forbidden := range []string{
		"encrypted_password",
		"ssh_encrypted_password",
		"ssh_password",
		"encryption_key",
		"ENCRYPTION_KEY",
		"private_key",
		"ConnectionProfile",
	} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("swagger.json exposes forbidden identifier %q", forbidden)
		}
	}

	doc := loadSwagger(t)

	if version, _ := doc["openapi"].(string); version != "3.1.0" {
		t.Errorf("openapi version = %q, want 3.1.0", version)
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("paths missing or wrong type")
	}
	gotPaths := make([]string, 0, len(paths))
	for path := range paths {
		gotPaths = append(gotPaths, path)
	}
	sort.Strings(gotPaths)

	wantPaths := []string{
		"/connections",
		"/connections/test",
		"/connections/{id}",
		"/connections/{id}/databases",
		"/connections/{id}/schemas",
		"/connections/{id}/table-data",
		"/health",
		"/queries/saved",
		"/queries/saved/{id}",
		"/query/execute",
		"/query/history",
	}
	if strings.Join(gotPaths, ",") != strings.Join(wantPaths, ",") {
		t.Errorf("swagger paths = %v, want %v", gotPaths, wantPaths)
	}

	schemas, _ := doc["components"].(map[string]any)["schemas"].(map[string]any)
	if schemas == nil {
		t.Fatal("components.schemas missing")
	}

	response, _ := schemas["handler.ConnectionResponse"].(map[string]any)
	props, _ := response["properties"].(map[string]any)
	for _, forbidden := range []string{"password", "encrypted_password", "ssh_encrypted_password"} {
		if _, ok := props[forbidden]; ok {
			t.Errorf("ConnectionResponse schema must not contain %q", forbidden)
		}
	}

	request, _ := schemas["handler.ConnectionRequest"].(map[string]any)
	requestProps, _ := request["properties"].(map[string]any)
	if _, ok := requestProps["password"]; !ok {
		t.Error("ConnectionRequest schema should document the write-only password field")
	}

	// Required fields must match the actual validation behavior.
	// PRF-01: database_name is required per-driver at runtime (mysql/sqlite)
	// but optional in the schema because PostgreSQL profiles may omit it.
	requiredFields := map[string][]string{
		"handler.ConnectionRequest":     {"driver", "name"},
		"handler.ConnectionTestRequest": {"driver"},
		"handler.QueryRequest":          {"connection_id", "sql"},
		"handler.SavedQueryRequest":     {"title", "sql_text"},
	}
	for schemaName, want := range requiredFields {
		schema, _ := schemas[schemaName].(map[string]any)
		if schema == nil {
			t.Errorf("swagger missing schema %q", schemaName)
			continue
		}
		got := requiredList(schema)
		wantSorted := append([]string(nil), want...)
		sort.Strings(wantSorted)
		if strings.Join(got, ",") != strings.Join(wantSorted, ",") {
			t.Errorf("%s required = %v, want %v", schemaName, got, wantSorted)
		}
	}

	// Genuinely optional fields must not be marked required.
	for _, schemaName := range []string{"handler.ConnectionRequest", "handler.ConnectionTestRequest", "handler.QueryRequest"} {
		schema, _ := schemas[schemaName].(map[string]any)
		for _, field := range requiredList(schema) {
			switch field {
			case "password", "port", "ssl_mode", "timeout_seconds":
				t.Errorf("%s incorrectly marks optional field %q required", schemaName, field)
			}
		}
	}
}

func requiredList(schema map[string]any) []string {
	raw, _ := schema["required"].([]any)
	fields := make([]string, 0, len(raw))
	for _, value := range raw {
		if name, ok := value.(string); ok {
			fields = append(fields, name)
		}
	}
	sort.Strings(fields)
	return fields
}

func newContractRouter(t *testing.T) http.Handler {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "datadeck.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cipher, err := security.NewCipher(contractKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	cfg := config.Config{
		Host:               config.DefaultHost,
		StoragePath:        store.Path(),
		LogLevel:           slog.LevelInfo,
		Environment:        config.EnvTest,
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	}
	server := NewServer(
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		cipher,
		repository.NewConnectionRepository(store.DB()),
		repository.NewQueryHistoryRepository(store.DB()),
		repository.NewSavedQueryRepository(store.DB()),
		database.NewManager(database.DefaultOptions(), database.Postgres{}, database.MySQL{}, database.SQLite{}),
	)
	return server.Router()
}

func TestEnvelopeConsistencyAcrossEndpoints(t *testing.T) {
	router := newContractRouter(t)

	cases := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"health", http.MethodGet, "/api/v1/health", ""},
		{"list connections", http.MethodGet, "/api/v1/connections", ""},
		{"create invalid", http.MethodPost, "/api/v1/connections", `{}`},
		{"test invalid", http.MethodPost, "/api/v1/connections/test", `{}`},
		{"delete missing", http.MethodDelete, "/api/v1/connections/missing", ""},
		{"schemas missing", http.MethodGet, "/api/v1/connections/missing/schemas", ""},
		{"execute invalid", http.MethodPost, "/api/v1/query/execute", `{}`},
		{"history", http.MethodGet, "/api/v1/query/history", ""},
		{"saved queries", http.MethodGet, "/api/v1/queries/saved", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			if tc.method == http.MethodPost || tc.method == http.MethodPut {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(rec, req)

			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var keys map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
				t.Fatalf("response is not a JSON object: %v (body=%s)", err, rec.Body.String())
			}
			want := []string{"data", "error", "meta", "success"}
			got := make([]string, 0, len(keys))
			for key := range keys {
				got = append(got, key)
			}
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("envelope keys = %v, want %v (body=%s)", got, want, rec.Body.String())
			}
		})
	}
}

func TestCreateConnectionResponseNeverExposesSecrets(t *testing.T) {
	router := newContractRouter(t)

	body := `{"name":"PG","driver":"postgres","host":"127.0.0.1","port":5432,"database_name":"app","username":"u","password":"s3cret","ssl_mode":"disable"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	responseBody := rec.Body.String()
	for _, forbidden := range []string{"s3cret", "encrypted_password", "password"} {
		if strings.Contains(responseBody, forbidden) {
			t.Errorf("response exposed %q: %s", forbidden, responseBody)
		}
	}
}
