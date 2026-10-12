package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// multipartUpload builds a multipart/form-data body with one file part and the
// given form fields, and returns the part's Content-Type for the request.
func multipartUpload(t *testing.T, fields map[string]string, filename, content string) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return writer.FormDataContentType(), &buf
}

// uploadImport drives POST /connections/{id}/table-import.
func uploadImport(t *testing.T, h *ConnectionHandler, id string, contentType string, body *bytes.Buffer, wantCode int) *model.TableImportSession {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/"+id+"/table-import", body)
	req.Header.Set("Content-Type", contentType)
	h.TableImportUpload(rec, withPathParams(req, "id", id))
	if rec.Code != wantCode {
		t.Fatalf("upload status = %d, want %d (body=%s)", rec.Code, wantCode, rec.Body.String())
	}
	if wantCode != http.StatusOK {
		return nil
	}
	env := decodeEnvelope(t, rec)
	var session model.TableImportSession
	if err := json.Unmarshal(env.Data, &session); err != nil {
		t.Fatalf("decode session: %v (data=%s)", err, env.Data)
	}
	if session.ImportID == "" || session.RowCount == 0 || session.SHA256 != "" {
		t.Fatalf("incomplete session: %+v", session)
	}
	return &session
}

// importStage invokes validate or execute for a session.
func importStage(t *testing.T, h *ConnectionHandler, stage, id, importID, body string, wantCode int) envelope {
	t.Helper()
	target := "/api/v1/connections/" + id + "/table-import/" + importID + "/" + stage
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathParams(req, "id", id, "import_id", importID)
	switch stage {
	case "validate":
		h.TableImportValidate(rec, req)
	case "execute":
		h.TableImportExecute(rec, req)
	}
	if rec.Code != wantCode {
		t.Fatalf("%s status = %d, want %d (body=%s)", stage, rec.Code, wantCode, rec.Body.String())
	}
	return decodeEnvelope(t, rec)
}

func TestTableImportCSVLifecycle(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "import.db")
	seedProfile(t, db, &model.ConnectionProfile{ID: "imp", Name: "SQLite", Driver: model.DriverSQLite, DatabaseName: path})
	cfg := database.Config{Driver: model.DriverSQLite, Database: path}
	ctx := context.Background()

	if _, err := h.manager.Execute(ctx, "imp", cfg, `CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		active BOOLEAN,
		joined TEXT
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	csv := "id,name,active,joined\r\n" +
		"1,Ada,true,2024-01-02\r\n" +
		"2,Bob,false,\r\n" +
		"3,Cara,,2025-05-06\r\n"
	contentType, body := multipartUpload(t, map[string]string{
		"database": path, "table": "users", "format": "csv", "csv_header": "true",
	}, "users.csv", csv)
	session := uploadImport(t, h, "imp", contentType, body, http.StatusOK)
	if session.RowCount != 3 {
		t.Fatalf("row_count = %d, want 3", session.RowCount)
	}
	if len(session.Columns) != 4 || session.Columns[0] != "id" {
		t.Fatalf("columns = %v, want [id name active joined]", session.Columns)
	}

	target := `{"database":` + mustJSON(path) + `,"table":"users"}`
	// Echo-check: a different target is rejected without touching data.
	importStage(t, h, "validate", "imp", session.ImportID,
		`{"database":`+mustJSON(path)+`,"table":"other"}`, http.StatusBadRequest)

	// Validate, then execute.
	validateEnv := importStage(t, h, "validate", "imp", session.ImportID, target, http.StatusOK)
	var validation model.TableImportValidation
	if err := json.Unmarshal(validateEnv.Data, &validation); err != nil {
		t.Fatalf("decode validation: %v", err)
	}
	if !validation.Valid || validation.RowCount != 3 || len(validation.Mapping) != 3 {
		t.Fatalf("validation = %+v, want valid with 3 rows and 3 mapped columns (id is a SQLite rowid alias, not insertable)", validation)
	}

	execEnv := importStage(t, h, "execute", "imp", session.ImportID, target, http.StatusOK)
	var result model.TableImportExecutionResult
	if err := json.Unmarshal(execEnv.Data, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Inserted != 3 || result.RowCount != 3 {
		t.Fatalf("result = %+v, want 3 inserted", result)
	}

	count := queryScalar(t, h, "imp", cfg, "SELECT count(*) FROM users")
	if count != "3" {
		t.Fatalf("users count = %s, want 3", count)
	}

	// Re-execute hits the unique(name) constraint → 409, and the rollback leaves
	// the table at exactly 3 rows (no partial re-import).
	importStage(t, h, "execute", "imp", session.ImportID, target, http.StatusConflict)
	if count := queryScalar(t, h, "imp", cfg, "SELECT count(*) FROM users"); count != "3" {
		t.Fatalf("users after failed second execute = %s, want 3 (rollback)", count)
	}
}

func TestTableImportSQLScript(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "imp.db")
	seedProfile(t, db, &model.ConnectionProfile{ID: "imp", Name: "SQLite", Driver: model.DriverSQLite, DatabaseName: path})
	cfg := database.Config{Driver: model.DriverSQLite, Database: path}

	if _, err := h.manager.Execute(context.Background(), "imp", cfg, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	script := "BEGIN;\nINSERT INTO \"t\" (id, v) VALUES (1, 'a'), (2, 'b');\nCOMMIT;\n"
	contentType, body := multipartUpload(t, map[string]string{
		"database": path, "table": "t", "format": "sql",
	}, "t.sql", script)
	session := uploadImport(t, h, "imp", contentType, body, http.StatusOK)
	if session.RowCount != 2 {
		t.Fatalf("row_count = %d, want 2", session.RowCount)
	}

	target := `{"database":` + mustJSON(path) + `,"table":"t"}`
	execEnv := importStage(t, h, "execute", "imp", session.ImportID, target, http.StatusOK)
	var result model.TableImportExecutionResult
	if err := json.Unmarshal(execEnv.Data, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Inserted != 2 {
		t.Fatalf("result = %+v, want 2 inserted", result)
	}
	if count := queryScalar(t, h, "imp", cfg, "SELECT count(*) FROM t"); count != "2" {
		t.Fatalf("rows = %s, want 2", count)
	}
}

func TestTableImportRejects(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "imp.db")
	seedProfile(t, db, &model.ConnectionProfile{ID: "imp", Name: "SQLite", Driver: model.DriverSQLite, DatabaseName: path})

	uploadStatus := func(fields map[string]string, filename, content string) (int, envelope) {
		ct, body := multipartUpload(t, fields, filename, content)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/imp/table-import", body)
		req.Header.Set("Content-Type", ct)
		h.TableImportUpload(rec, withPathParams(req, "id", "imp"))
		if rec.Code != http.StatusOK {
			return rec.Code, decodeEnvelope(t, rec)
		}
		return rec.Code, envelope{}
	}

	// Unknown format.
	code, env := uploadStatus(map[string]string{"database": path, "table": "t", "format": "xlsx"}, "x.xlsx", "a,b\r\n1,2\r\n")
	if code != http.StatusBadRequest || env.Error == nil || env.Error.Code != "IMPORT_FORMAT_UNSUPPORTED" {
		t.Fatalf("unsupported format -> %d (%+v)", code, env.Error)
	}

	// Uneven CSV width → validation error before any data is written.
	code, env = uploadStatus(map[string]string{"database": path, "table": "t", "format": "csv"}, "bad.csv", "a,b\r\n1\r\n")
	if code != http.StatusBadRequest || env.Error == nil || env.Error.Code != "IMPORT_VALIDATION_ERROR" {
		t.Fatalf("uneven csv -> %d (%+v)", code, env.Error)
	}

	// Non-INSERT SQL is rejected.
	code, env = uploadStatus(map[string]string{"database": path, "table": "t", "format": "sql"}, "ddl.sql", "DROP TABLE t;")
	if code != http.StatusBadRequest || env.Error == nil {
		t.Fatalf("ddl -> %d (%+v)", code, env.Error)
	}

	// Unknown session → NOT_FOUND.
	env = importStage(t, h, "validate", "imp", "missing-session",
		`{"database":`+mustJSON(path)+`,"table":"t"}`, http.StatusNotFound)
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Fatalf("unknown session error = %+v, want NOT_FOUND", env.Error)
	}

	// SQL script targeting another relation is rejected at upload.
	ct, body := multipartUpload(t, map[string]string{
		"database": path, "table": "t", "format": "sql",
	}, "z.sql", `INSERT INTO other (id) VALUES (1);`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/imp/table-import", body)
	req.Header.Set("Content-Type", ct)
	h.TableImportUpload(rec, withPathParams(req, "id", "imp"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("relation mismatch status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
}

// Oversized multipart uploads are rejected with 413 before anything is parsed
// (security: the upload size cap). The cap is lowered so the test need not
// allocate 64 MiB.
func TestTableImportRejectsOversize(t *testing.T) {
	h, db, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "imp.db")
	seedProfile(t, db, &model.ConnectionProfile{ID: "imp", Name: "SQLite", Driver: model.DriverSQLite, DatabaseName: path})

	original := maxImportUploadBytes
	maxImportUploadBytes = 1024
	t.Cleanup(func() { maxImportUploadBytes = original })

	ct, body := multipartUpload(t, map[string]string{
		"database": path, "table": "t", "format": "csv",
	}, "big.csv", strings.Repeat("a", 4096))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/imp/table-import", body)
	req.Header.Set("Content-Type", ct)
	h.TableImportUpload(rec, withPathParams(req, "id", "imp"))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize upload status = %d, want 413 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "IMPORT_TOO_LARGE" {
		t.Fatalf("oversize error = %+v, want IMPORT_TOO_LARGE", env.Error)
	}
}

// withPathParams injects chi route parameters (direct handler calls bypass the
// router, which is where these are normally populated).
func withPathParams(req *http.Request, pairs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(pairs); i += 2 {
		rctx.URLParams.Add(pairs[i], pairs[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func mustJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// queryScalar returns the first cell of a single-row query as a string.
func queryScalar(t *testing.T, h *ConnectionHandler, id string, cfg database.Config, sql string) string {
	t.Helper()
	result, err := h.manager.Execute(context.Background(), id, cfg, sql)
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	if len(result.Rows) == 0 {
		t.Fatalf("query %q returned no rows", sql)
	}
	return cellString(result.Rows[0][0])
}

func cellString(v any) string {
	switch v := v.(type) {
	case nil:
		return "<nil>"
	case []byte:
		return string(v)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
