package handler

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/dbscript"
	"github.com/datadeck/datadeck/backend/internal/model"
)

const (
	// maxRestoreUploadBytes caps the whole restore multipart body.
	maxRestoreUploadBytes = 512 << 20 // 512 MiB
	// scriptSessionTTL is how long a validated restore preview stays executable.
	scriptSessionTTL = 30 * time.Minute
)

// scriptSession is one validated, bindable restore preview.
type scriptSession struct {
	tempPath   string
	database   string
	createdAt  time.Time
	statements int
}

// DatabaseImportPreview is returned by the restore upload/preflight endpoint.
// It is a preview only: nothing is written to the target database.
type DatabaseImportPreview struct {
	Token       string   `json:"token"`
	Engine      string   `json:"engine"`
	Database    string   `json:"database"`
	Mode        string   `json:"mode"`
	Tables      int      `json:"tables"`
	Indexes     int      `json:"indexes"`
	ForeignKeys int      `json:"foreign_keys"`
	Statements  int      `json:"statements"`
	Excluded    []string `json:"excluded,omitempty"`
}

// DatabaseRestoreResult is returned by the restore execute endpoint.
type DatabaseRestoreResult struct {
	Status     string `json:"status" example:"ok"`
	Database   string `json:"database"`
	Statements int    `json:"statements"`
}

// DatabaseImportUpload godoc
// @Summary      Upload and preflight a database restore artifact
// @Description  Validates a DataDeck SQL export artifact against the explicit target (engine + database), checks every statement against the restore allowlist, and preflights the target for conflicts. Nothing is written: this only produces an executable preview bound to a token. The target database must be empty. Restore is disabled for MySQL (DDL implicitly commits).
// @Tags         database
// @Accept       multipart/form-data
// @Produce      json
// @Param        id        path     string  true  "Connection id"
// @Param        file      formData file    true  "DataDeck SQL export artifact"
// @Param        database  formData string  true  "Target database"
// @Success      200  {object} response.Envelope{data=handler.DatabaseImportPreview}
// @Failure      400  {object} response.ErrorEnvelope
// @Failure      404  {object} response.ErrorEnvelope
// @Failure      409  {object} response.ErrorEnvelope
// @Failure      413  {object} response.ErrorEnvelope
// @Failure      501  {object} response.ErrorEnvelope
// @Router       /connections/{id}/database-import [post]
func (h *ConnectionHandler) DatabaseImportUpload(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		response.ValidationError(w, "Content-Type must be multipart/form-data")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRestoreUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.WriteError(w, http.StatusRequestEntityTooLarge, "IMPORT_TOO_LARGE",
				fmt.Sprintf("the artifact exceeds the %d byte limit", maxRestoreUploadBytes))
			return
		}
		response.ValidationError(w, "invalid multipart upload")
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) != 1 {
		response.ValidationError(w, "exactly one file part named \"file\" is required")
		return
	}

	profile, password, effectiveDatabase, ok := h.resolveScriptTarget(w, r, id, r.FormValue("database"))
	if !ok {
		return
	}
	if profile.Driver == model.DriverMySQL {
		response.WriteError(w, http.StatusNotImplemented, "RESTORE_UNSUPPORTED",
			"SQL restore is not supported for MySQL (DDL implicitly commits)")
		return
	}

	tempPath, manifest, statements, err := h.spoolAndValidateUpload(files[0], string(profile.Driver), effectiveDatabase)
	if err != nil {
		h.restoreUploadError(w, r, err)
		return
	}

	// Preflight: the target must be empty (no base tables). Never DROP/REPLACE.
	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase
	databases, err := h.manager.Introspect(r.Context(), id, cfg)
	if err != nil {
		_ = os.Remove(tempPath)
		h.tableDataError(w, r, err)
		return
	}
	if existing := collectBaseTables(databases); len(existing) > 0 {
		_ = os.Remove(tempPath)
		response.WriteError(w, http.StatusConflict, "RESTORE_CONFLICT",
			fmt.Sprintf("the target database already contains %d table(s); restore requires an empty database", len(existing)))
		return
	}

	token, err := newRestoreToken()
	if err != nil {
		_ = os.Remove(tempPath)
		h.internalError(w, r, "generate_restore_token", err)
		return
	}
	h.sweepScriptSessions()
	h.scriptMu.Lock()
	h.scriptSessions[token] = &scriptSession{
		tempPath:   tempPath,
		database:   effectiveDatabase,
		createdAt:  time.Now(),
		statements: len(statements),
	}
	h.scriptMu.Unlock()

	response.Success(w, DatabaseImportPreview{
		Token:       token,
		Engine:      string(profile.Driver),
		Database:    effectiveDatabase,
		Mode:        string(manifest.Mode),
		Tables:      manifest.Tables,
		Indexes:     manifest.Indexes,
		ForeignKeys: manifest.ForeignKeys,
		Statements:  len(statements),
		Excluded:    manifest.Excluded,
	})
}

// DatabaseImportExecute godoc
// @Summary      Execute a preflighted database restore
// @Description  Re-validates the spooled artifact and applies it to the explicit target in a single all-or-nothing transaction (PostgreSQL/SQLite). Rejects MySQL. Never replays a mutation and never redirects to another database.
// @Tags         database
// @Produce      json
// @Param        id     path     string  true  "Connection id"
// @Param        token  path     string  true  "Restore token"
// @Success      200  {object} response.Envelope{data=handler.DatabaseRestoreResult}
// @Failure      400  {object} response.ErrorEnvelope
// @Failure      404  {object} response.ErrorEnvelope
// @Failure      501  {object} response.ErrorEnvelope
// @Router       /connections/{id}/database-import/{token}/execute [post]
func (h *ConnectionHandler) DatabaseImportExecute(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if id == "" || token == "" {
		response.ValidationError(w, "connection id and token are required")
		return
	}

	h.scriptMu.Lock()
	session, ok := h.scriptSessions[token]
	if ok {
		delete(h.scriptSessions, token)
	}
	h.scriptMu.Unlock()
	if !ok || time.Since(session.createdAt) > scriptSessionTTL {
		if ok {
			_ = os.Remove(session.tempPath)
		}
		response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "unknown or expired restore session")
		return
	}
	defer func() { _ = os.Remove(session.tempPath) }()

	profile, password, effectiveDatabase, ok := h.resolveScriptTarget(w, r, id, session.database)
	if !ok {
		return
	}
	if profile.Driver == model.DriverMySQL {
		response.WriteError(w, http.StatusNotImplemented, "RESTORE_UNSUPPORTED",
			"SQL restore is not supported for MySQL (DDL implicitly commits)")
		return
	}

	statements, err := h.parseSpooledArtifact(session.tempPath, string(profile.Driver), effectiveDatabase)
	if err != nil {
		h.restoreUploadError(w, r, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), databaseScriptTimeoutSeconds*time.Second)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase
	sql := make([]string, 0, len(statements))
	for _, statement := range statements {
		if statement.Kind == dbscript.StmtTransaction {
			continue // the manager owns the transaction
		}
		sql = append(sql, statement.SQL)
	}

	if err := h.manager.RestoreScript(ctx, id, cfg, sql); err != nil {
		h.databaseScriptError(w, r, err, "database_restore_failed")
		return
	}
	response.Success(w, DatabaseRestoreResult{Status: "ok", Database: effectiveDatabase, Statements: len(sql)})
}

// spoolAndValidateUpload spools the upload to a temp file, then validates the
// manifest and every statement. On error the temp file is removed and a nil
// path is returned.
func (h *ConnectionHandler) spoolAndValidateUpload(header *multipart.FileHeader, engine, database string) (string, dbscript.Manifest, []dbscript.Statement, error) {
	temp, err := os.CreateTemp("", "datadeck-restore-*")
	if err != nil {
		return "", dbscript.Manifest{}, nil, err
	}
	tempPath := temp.Name()
	remove := func() { _ = temp.Close(); _ = os.Remove(tempPath) }

	src, err := header.Open()
	if err != nil {
		remove()
		return "", dbscript.Manifest{}, nil, err
	}
	if _, err := io.Copy(temp, src); err != nil {
		_ = src.Close()
		remove()
		return "", dbscript.Manifest{}, nil, err
	}
	_ = src.Close()
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		remove()
		return "", dbscript.Manifest{}, nil, err
	}

	reader := bufio.NewReader(temp)
	manifest, err := dbscript.ReadHeader(reader)
	if err != nil {
		remove()
		return "", dbscript.Manifest{}, nil, err
	}
	if err := dbscript.ValidateManifest(manifest, engine, database); err != nil {
		remove()
		return "", dbscript.Manifest{}, nil, err
	}
	statements, err := dbscript.ParseStatements(reader)
	if err != nil {
		remove()
		return "", dbscript.Manifest{}, nil, err
	}
	_ = temp.Close()
	return tempPath, manifest, statements, nil
}

func (h *ConnectionHandler) parseSpooledArtifact(tempPath, engine, database string) ([]dbscript.Statement, error) {
	file, err := os.Open(tempPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	reader := bufio.NewReader(file)
	manifest, err := dbscript.ReadHeader(reader)
	if err != nil {
		return nil, err
	}
	if err := dbscript.ValidateManifest(manifest, engine, database); err != nil {
		return nil, err
	}
	return dbscript.ParseStatements(reader)
}

// restoreUploadError maps artifact validation failures to contract errors.
func (h *ConnectionHandler) restoreUploadError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, dbscript.ErrNotDataDeckArtifact):
		response.WriteError(w, http.StatusBadRequest, "RESTORE_FORMAT_UNSUPPORTED",
			"the file is not a DataDeck SQL export")
	case errors.Is(err, dbscript.ErrManifestMismatch):
		response.WriteError(w, http.StatusBadRequest, "RESTORE_TARGET_MISMATCH", err.Error())
	case errors.Is(err, dbscript.ErrUnsupportedStatement):
		response.WriteError(w, http.StatusBadRequest, "RESTORE_UNSUPPORTED_STATEMENT", err.Error())
	default:
		h.internalError(w, r, "restore_upload", err)
	}
}

// sweepScriptSessions removes expired sessions and their temp files.
func (h *ConnectionHandler) sweepScriptSessions() {
	h.scriptMu.Lock()
	var stale []string
	for token, session := range h.scriptSessions {
		if time.Since(session.createdAt) > scriptSessionTTL {
			stale = append(stale, session.tempPath)
			delete(h.scriptSessions, token)
		}
	}
	h.scriptMu.Unlock()
	for _, path := range stale {
		_ = os.Remove(path)
	}
}

func newRestoreToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
