package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/dbscript"
	"github.com/datadeck/datadeck/backend/internal/model"
)

func sqliteConfig(path string) database.Config {
	return database.Config{Driver: model.DriverSQLite, Database: path}
}

// exportSQLite exports a sqlite database to a SQL script via the handler source.
func exportSQLite(t *testing.T, h *ConnectionHandler, id, path string) []byte {
	t.Helper()
	src := &databaseScriptSource{handler: h, id: id, cfg: sqliteConfig(path), driver: "sqlite"}
	manifest := dbscript.Manifest{
		Engine:      "sqlite",
		Database:    path,
		Mode:        dbscript.ModeSchemaData,
		GeneratedAt: time.Now(),
	}
	var buf bytes.Buffer
	if err := dbscript.Export(context.Background(), &buf, manifest, src); err != nil {
		t.Fatalf("Export: %v", err)
	}
	return buf.Bytes()
}

func TestSQLiteDatabaseExportRestoreRoundTrip(t *testing.T) {
	h, store, _ := newTestHandler(t)
	ctx := context.Background()
	srcPath := filepath.Join(t.TempDir(), "src.db")
	seedProfile(t, store, &model.ConnectionProfile{ID: "src", Name: "src", Driver: model.DriverSQLite, DatabaseName: srcPath})
	cfg := sqliteConfig(srcPath)

	for _, ddl := range []string{
		`CREATE TABLE parent (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE)`,
		`CREATE TABLE child (
			id INTEGER PRIMARY KEY,
			parent_id INTEGER NOT NULL,
			amount INTEGER,
			note TEXT,
			CONSTRAINT fk_p FOREIGN KEY (parent_id) REFERENCES parent (id),
			CHECK (amount >= 0)
		)`,
		`CREATE INDEX ix_child_parent ON child (parent_id)`,
		`INSERT INTO parent (id, name) VALUES (1, 'Ünïcode'), (2, 'two')`,
		`INSERT INTO child (id, parent_id, amount, note) VALUES (1, 1, 42, NULL), (2, 2, 0, 'x')`,
	} {
		if _, err := h.manager.Execute(ctx, "src", cfg, ddl); err != nil {
			t.Fatalf("seed %q: %v", ddl, err)
		}
	}

	artifact := exportSQLite(t, h, "src", srcPath)
	if !bytes.Contains(artifact, []byte("-- datadeck:export")) {
		t.Fatalf("artifact missing manifest:\n%s", artifact)
	}
	if !bytes.Contains(artifact, []byte("CREATE TABLE")) {
		t.Fatalf("artifact missing DDL:\n%s", artifact)
	}
	if !bytes.Contains(artifact, []byte("'Ünïcode'")) {
		t.Fatalf("artifact lost unicode data:\n%s", artifact)
	}

	// Restore the parsed statements into a fresh empty database.
	reader := bufio.NewReader(bytes.NewReader(artifact))
	if _, err := dbscript.ReadHeader(reader); err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	statements, err := dbscript.ParseStatements(reader)
	if err != nil {
		t.Fatalf("ParseStatements: %v", err)
	}
	sql := make([]string, 0, len(statements))
	for _, statement := range statements {
		if statement.Kind != dbscript.StmtTransaction {
			sql = append(sql, statement.SQL)
		}
	}

	dstPath := filepath.Join(t.TempDir(), "dst.db")
	seedProfile(t, store, &model.ConnectionProfile{ID: "dst", Name: "dst", Driver: model.DriverSQLite, DatabaseName: dstPath})
	if err := h.manager.RestoreScript(ctx, "dst", sqliteConfig(dstPath), sql); err != nil {
		t.Fatalf("RestoreScript: %v", err)
	}

	result, err := h.manager.Execute(ctx, "dst", sqliteConfig(dstPath), "SELECT name FROM parent ORDER BY id")
	if err != nil {
		t.Fatalf("verify parents: %v", err)
	}
	rows := result.Rows
	if len(rows) != 2 || rows[0][0] != "Ünïcode" || rows[1][0] != "two" {
		t.Fatalf("parent rows = %#v", rows)
	}

	// Referential integrity: the foreign key was restored.
	fkResult, err := h.manager.Execute(ctx, "dst", sqliteConfig(dstPath), "PRAGMA foreign_key_list(child)")
	if err != nil {
		t.Fatalf("pragma foreign_key_list: %v", err)
	}
	if len(fkResult.Rows) == 0 {
		t.Error("foreign key was not restored")
	}
}

func databaseExportRequest(id, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/"+id+"/database-export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return withPathParams(req, "id", id)
}

func TestDatabaseExportHandler(t *testing.T) {
	h, store, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "src.db")
	seedProfile(t, store, &model.ConnectionProfile{ID: "src", Name: "src", Driver: model.DriverSQLite, DatabaseName: path})
	if _, err := h.manager.Execute(context.Background(), "src", sqliteConfig(path), "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.DatabaseExport(rec, databaseExportRequest("src", `{"database":`+mustJSON(path)+`,"mode":"schema_data"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, "-- datadeck:export") || !strings.Contains(body, "CREATE TABLE") {
		t.Errorf("unexpected artifact:\n%s", body)
	}
}

func TestDatabaseExportRejectsBadMode(t *testing.T) {
	h, store, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "src.db")
	seedProfile(t, store, &model.ConnectionProfile{ID: "src", Name: "src", Driver: model.DriverSQLite, DatabaseName: path})

	rec := httptest.NewRecorder()
	h.DatabaseExport(rec, databaseExportRequest("src", `{"database":`+mustJSON(path)+`,"mode":"bogus"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "EXPORT_FORMAT_UNSUPPORTED" {
		t.Errorf("error = %+v", env.Error)
	}
}

// A restore artifact whose manifest engine/database does not match the explicit
// target is rejected before anything runs.
func TestDatabaseImportTargetMismatch(t *testing.T) {
	h, store, _ := newTestHandler(t)
	target := filepath.Join(t.TempDir(), "target.db")
	seedProfile(t, store, &model.ConnectionProfile{ID: "dst", Name: "dst", Driver: model.DriverSQLite, DatabaseName: target})

	foreign := "-- datadeck:export\n-- format_version: 1\n-- engine: sqlite\n-- database: /other.db\n-- mode: schema_data\n\nBEGIN;\nCREATE TABLE t (id INTEGER);\nCOMMIT;\n"
	ct, body := multipartUpload(t, map[string]string{"database": target}, "dump.sql", foreign)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/dst/database-import", body)
	req.Header.Set("Content-Type", ct)
	h.DatabaseImportUpload(rec, withPathParams(req, "id", "dst"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "RESTORE_TARGET_MISMATCH" {
		t.Errorf("error = %+v", env.Error)
	}
}

// Restore into a non-empty target is blocked by preflight (no DROP/REPLACE).
func TestDatabaseImportConflict(t *testing.T) {
	h, store, _ := newTestHandler(t)
	target := filepath.Join(t.TempDir(), "target.db")
	seedProfile(t, store, &model.ConnectionProfile{ID: "dst", Name: "dst", Driver: model.DriverSQLite, DatabaseName: target})
	if _, err := h.manager.Execute(context.Background(), "dst", sqliteConfig(target), "CREATE TABLE existing (id INTEGER)"); err != nil {
		t.Fatal(err)
	}

	artifact := "-- datadeck:export\n-- format_version: 1\n-- engine: sqlite\n-- database: " + target + "\n-- mode: schema_data\n\nBEGIN;\nCREATE TABLE t (id INTEGER);\nCOMMIT;\n"
	ct, body := multipartUpload(t, map[string]string{"database": target}, "dump.sql", artifact)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/dst/database-import", body)
	req.Header.Set("Content-Type", ct)
	h.DatabaseImportUpload(rec, withPathParams(req, "id", "dst"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "RESTORE_CONFLICT" {
		t.Errorf("error = %+v", env.Error)
	}
}

// Restore is disabled for MySQL.
func TestDatabaseImportMySQLUnsupported(t *testing.T) {
	h, store, _ := newTestHandler(t)
	host, port, user := "127.0.0.1", 3306, "u"
	seedProfile(t, store, &model.ConnectionProfile{ID: "my", Name: "my", Driver: model.DriverMySQL, Host: &host, Port: &port, Username: &user, DatabaseName: "app"})

	artifact := "-- datadeck:export\n-- format_version: 1\n-- engine: mysql\n-- database: app\n-- mode: schema_data\n\nBEGIN;\nCREATE TABLE t (id INT);\nCOMMIT;\n"
	ct, body := multipartUpload(t, map[string]string{"database": "app"}, "dump.sql", artifact)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/my/database-import", body)
	req.Header.Set("Content-Type", ct)
	h.DatabaseImportUpload(rec, withPathParams(req, "id", "my"))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501 (body=%s)", rec.Code, rec.Body.String())
	}
	if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "RESTORE_UNSUPPORTED" {
		t.Errorf("error = %+v", env.Error)
	}
}

// Full upload -> preview -> execute restores into an empty database.
func TestDatabaseImportUploadAndExecute(t *testing.T) {
	h, store, _ := newTestHandler(t)
	target := filepath.Join(t.TempDir(), "target.db")
	seedProfile(t, store, &model.ConnectionProfile{ID: "dst", Name: "dst", Driver: model.DriverSQLite, DatabaseName: target})

	artifact := "-- datadeck:export\n-- format_version: 1\n-- engine: sqlite\n-- database: " + target + "\n-- mode: schema_data\n\nBEGIN;\nCREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT);\nINSERT INTO \"t\" (\"id\", \"v\") VALUES (1, 'hi');\nCOMMIT;\n"
	ct, body := multipartUpload(t, map[string]string{"database": target}, "dump.sql", artifact)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections/dst/database-import", body)
	req.Header.Set("Content-Type", ct)
	h.DatabaseImportUpload(rec, withPathParams(req, "id", "dst"))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var preview DatabaseImportPreview
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.Token == "" || preview.Database != target {
		t.Fatalf("preview = %+v", preview)
	}

	execRec := httptest.NewRecorder()
	execReq := httptest.NewRequest(http.MethodPost, "/api/v1/connections/dst/database-import/"+preview.Token+"/execute", nil)
	h.DatabaseImportExecute(execRec, withPathParams(execReq, "id", "dst", "token", preview.Token))
	if execRec.Code != http.StatusOK {
		t.Fatalf("execute status = %d, want 200 (body=%s)", execRec.Code, execRec.Body.String())
	}
	result, err := h.manager.Execute(context.Background(), "dst", sqliteConfig(target), "SELECT v FROM t WHERE id = 1")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][0] != "hi" {
		t.Fatalf("restored rows = %#v", result.Rows)
	}
}
