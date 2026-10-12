package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/tableimport"
)

// Table import (PRF-02/T15). Every import is a session-bound, value-only
// operation: POST the file once (upload), get back a preview and an immutable
// binding (resolved database + schema + table + file SHA-256), then validate
// and execute against that binding. Validate/execute echo the frozen target,
// never accept a new one, and re-hash the spooled upload so a changed file
// cannot be executed. The database layer re-introspects the target at every
// stage, so a relation that appears/disappears cannot slip through (TOCTOU).

const (
	// importMaxPreviewRows bounds the preview cell grid returned at upload.
	importMaxPreviewRows = 50
	// importSessionTTL is how long an upload binding stays usable.
	importSessionTTL = 30 * time.Minute
	// importTimeoutSeconds bounds a single validate/execute request.
	importTimeoutSeconds = 300
)

// maxImportUploadBytes caps the whole multipart body (file + fields). The
// CSV/SQL parsers apply their own 50 MiB content cap as defense in depth. It is
// a var only so tests can lower the cap without allocating the full 64 MiB.
var maxImportUploadBytes int64 = 64 << 20 // 64 MiB

// importFormats mirrors the backend capabilities advertisement
// (postgres/mysql/sqlite all support csv + sql in v0.1.0).
var importFormats = map[string]bool{"csv": true, "sql": true}

// importSession is one frozen upload binding plus the spooled source file.
type importSession struct {
	session  model.TableImportSession
	tempPath string
}

// ImportValidationRequest is the validate/execute body. Only the mapping may be
// tuned; the target must echo the upload binding exactly.
type ImportValidationRequest struct {
	Database string                `json:"database"`
	Schema   string                `json:"schema"`
	Table    string                `json:"table"`
	Mapping  []model.ImportMapping `json:"mapping"`
}

// TableImportUpload godoc
// @Summary      Upload a file for table import
// @Description  Accepts a CSV (RFC 4180) or restricted SQL INSERT-only script for PRF-02/T15 import against the explicit connection + database + schema + table. The file is spooled, hashed, parsed into cells, and bound to the resolved target; returns a preview and an immutable import_id used by the validate/execute stages. The SQL format only parses single-table INSERT statements — never raw SQL/expressions.
// @Tags         table-data
// @Accept       multipart/form-data
// @Produce      json
// @Param        id         path   string  true  "Connection id"
// @Param        file       formData file    true  "CSV or SQL script file"
// @Param        database   formData string  true  "Target database (optional for postgres profiles with a default)"
// @Param        schema     formData string  false "Target schema (required for postgres)"
// @Param        table      formData string  true  "Target table"
// @Param        format     formData string  true  "Import format" enums(csv,sql)
// @Param        csv_header formData string  false "Treat the first record as a header (csv only)" enums(true,false)
// @Success      200  {object}  response.Envelope{data=model.TableImportSession}
// @Failure      400  {object}  response.ErrorEnvelope
// @Failure      404  {object}  response.ErrorEnvelope
// @Failure      413  {object}  response.ErrorEnvelope
// @Failure      500  {object}  response.ErrorEnvelope
// @Failure      502  {object}  response.ErrorEnvelope
// @Failure      504  {object}  response.ErrorEnvelope
// @Router       /connections/{id}/table-import [post]
func (h *ConnectionHandler) TableImportUpload(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		response.ValidationError(w, "Content-Type must be multipart/form-data")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImportUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.WriteError(w, http.StatusRequestEntityTooLarge, "IMPORT_TOO_LARGE",
				fmt.Sprintf("the uploaded file exceeds the %d byte limit", maxImportUploadBytes))
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
	fileHeader := files[0]

	format := strings.ToLower(strings.TrimSpace(r.FormValue("format")))
	if !importFormats[format] {
		response.WriteError(w, http.StatusBadRequest, "IMPORT_FORMAT_UNSUPPORTED", "format must be one of: csv, sql")
		return
	}
	table := strings.TrimSpace(r.FormValue("table"))
	if table == "" {
		response.ValidationError(w, "table is required")
		return
	}
	schema := strings.TrimSpace(r.FormValue("schema"))
	csvHeader := strings.TrimSpace(r.FormValue("csv_header")) == "true"

	profile, err := h.connections.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "connection profile not found")
			return
		}
		h.internalError(w, r, "get_connection", err)
		return
	}
	if profile.Driver == model.DriverPostgres && schema == "" {
		response.ValidationError(w, "schema is required for postgresql table import")
		return
	}
	effectiveDatabase, err := resolveTargetDatabase(profile, r.FormValue("database"))
	if err != nil {
		writeResolveDatabaseError(w, err)
		return
	}

	// Spool the upload to a private temp file while hashing it, then parse the
	// spooled bytes for the source columns/preview/row count.
	temp, err := os.CreateTemp("", "datadeck-import-*")
	if err != nil {
		h.internalError(w, r, "create_import_temp", err)
		return
	}
	tempPath := temp.Name()

	file, err := fileHeader.Open()
	if err != nil {
		h.internalError(w, r, "open_upload", err)
		return
	}
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temp, hasher), file); err != nil {
		_ = file.Close()
		_ = temp.Close()
		h.internalError(w, r, "spool_upload", err)
		return
	}
	if err := file.Close(); err != nil {
		_ = temp.Close()
		h.internalError(w, r, "close_upload", err)
		return
	}
	info, err := temp.Stat()
	if err != nil {
		_ = temp.Close()
		h.internalError(w, r, "stat_import_temp", err)
		return
	}
	if err := temp.Close(); err != nil {
		h.internalError(w, r, "close_import_temp", err)
		return
	}

	var columns []string
	var preview [][]model.ImportCell
	var rowCount int
	parse := func() error {
		preview = preview[:0]
		collect := func(row []model.ImportCell) error {
			if len(preview) < importMaxPreviewRows {
				preview = append(preview, row)
			}
			return nil
		}
		source, err := os.Open(tempPath)
		if err != nil {
			return err
		}
		defer func() { _ = source.Close() }()
		switch format {
		case "csv":
			columns, rowCount, err = tableimport.ParseCSV(source, tableimport.CSVOptions{HeaderRow: csvHeader}, collect)
		case "sql":
			var relation string
			relation, columns, rowCount, err = tableimport.ParseSQLInserts(source, tableimport.SQLOptions{Dialect: string(profile.Driver)}, collect)
			if err == nil && scriptTargetMismatch(relation, schema, table) {
				err = fmt.Errorf("sql: script targets %q, not %q", relation, table)
			}
		}
		return err
	}
	if err := parse(); err != nil {
		response.WriteError(w, http.StatusBadRequest, "IMPORT_VALIDATION_ERROR",
			"file could not be parsed: "+sanitizeImportError(err))
		return
	}

	h.sweepImportSessions()
	importID, err := newID()
	if err != nil {
		h.internalError(w, r, "generate_import_id", err)
		return
	}
	entry := &importSession{
		session: model.TableImportSession{
			ImportID:     importID,
			ConnectionID: id,
			Database:     effectiveDatabase,
			Schema:       schema,
			Table:        table,
			Format:       format,
			Filename:     fileHeader.Filename,
			SizeBytes:    info.Size(),
			SHA256:       hex.EncodeToString(hasher.Sum(nil)),
			HeaderRow:    csvHeader,
			Columns:      columns,
			Preview:      preview,
			RowCount:     rowCount,
			CreatedAt:    time.Now(),
		},
		tempPath: tempPath,
	}
	h.putImportSession(entry)

	response.Success(w, entry.session)
}

// scriptTargetMismatch reports a SQL script whose INSERT relation does not match
// the upload binding. The script may schema-qualify its table ("public.users")
// while the binding separates schema/table; only the final segment must agree.
func scriptTargetMismatch(relation, schema, table string) bool {
	relation = strings.TrimSpace(relation)
	if relation == "" {
		return false
	}
	if relation == table {
		return false
	}
	if i := strings.LastIndexByte(relation, '.'); i >= 0 {
		if relation[i+1:] == table {
			return false
		}
		if schema != "" && relation[:i] == schema {
			return false
		}
	}
	return true
}

// TableImportValidate godoc
// @Summary      Validate an uploaded import
// @Description  Re-introspects the target and converts every row against the resolved mapping without writing anything, returning localized validation errors (first 50), the effective mapping and the column plan. The request target must exactly echo the upload binding; the spooled file is re-hashed to guarantee immutability. No data is written.
// @Tags         table-data
// @Accept       json
// @Produce      json
// @Param        id         path  string                          true "Connection id"
// @Param        import_id  path  string                          true "Import session id"
// @Param        body       body  handler.ImportValidationRequest  true "Echo the upload binding plus the resolved mapping"
// @Success      200  {object}  response.Envelope{data=model.TableImportValidation}
// @Failure      400  {object}  response.ErrorEnvelope
// @Failure      404  {object}  response.ErrorEnvelope
// @Failure      502  {object}  response.ErrorEnvelope
// @Failure      504  {object}  response.ErrorEnvelope
// @Router       /connections/{id}/table-import/{import_id}/validate [post]
func (h *ConnectionHandler) TableImportValidate(w http.ResponseWriter, r *http.Request) {
	h.importStage(w, r, false)
}

// TableImportExecute godoc
// @Summary      Execute an uploaded import
// @Description  Streams the spooled upload into one all-or-nothing transaction of parameterized INSERTs. The target and mapping are re-resolved fresh from introspection; any single failure (constraint, NOT NULL, timeout) rolls the whole import back — never a partial commit. The request target must exactly echo the upload binding.
// @Tags         table-data
// @Accept       json
// @Produce      json
// @Param        id         path  string                          true "Connection id"
// @Param        import_id  path  string                          true "Import session id"
// @Param        body       body  handler.ImportValidationRequest  true "Echo the upload binding plus the resolved mapping"
// @Success      200  {object}  response.Envelope{data=model.TableImportExecutionResult}
// @Failure      400  {object}  response.ErrorEnvelope
// @Failure      404  {object}  response.ErrorEnvelope
// @Failure      409  {object}  response.ErrorEnvelope
// @Failure      502  {object}  response.ErrorEnvelope
// @Failure      504  {object}  response.ErrorEnvelope
// @Router       /connections/{id}/table-import/{import_id}/execute [post]
func (h *ConnectionHandler) TableImportExecute(w http.ResponseWriter, r *http.Request) {
	h.importStage(w, r, true)
}

// importStage is the shared validate/execute pipeline: load and verify the
// binding, then delegate to ValidateImportRows or ExecuteImportRows with a
// stream that re-parses the spooled file.
func (h *ConnectionHandler) importStage(w http.ResponseWriter, r *http.Request, execute bool) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	importID := strings.TrimSpace(chi.URLParam(r, "import_id"))
	if id == "" || importID == "" {
		response.ValidationError(w, "connection id and import_id are required")
		return
	}
	var req ImportValidationRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	entry := h.getImportSession(id, importID)
	if entry == nil {
		response.WriteError(w, http.StatusNotFound, "NOT_FOUND",
			"import session not found or expired; re-upload the file")
		return
	}
	session := entry.session
	if err := h.verifyImportFile(entry); err != nil {
		response.WriteError(w, http.StatusBadRequest, "IMPORT_VALIDATION_ERROR", err.Error())
		return
	}
	if !bindingMatches(session, req.Database, req.Schema, req.Table) {
		response.WriteError(w, http.StatusBadRequest, "IMPORT_VALIDATION_ERROR",
			"the target must exactly match the upload binding; re-upload to import elsewhere")
		return
	}

	profile, err := h.connections.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "connection profile not found")
			return
		}
		h.internalError(w, r, "get_connection", err)
		return
	}
	password := ""
	if profile.EncryptedPassword != nil {
		plaintext, err := h.cipher.Decrypt(*profile.EncryptedPassword)
		if err != nil {
			h.internalError(w, r, "decrypt_password", err)
			return
		}
		password = string(plaintext)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(importTimeoutSeconds)*time.Second)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = session.Database

	planReq := database.ImportPlanRequest{
		Schema: session.Schema,
		Table:  session.Table,
		// SourceColumns are the frozen, parsed source columns from the upload; a
		// nil value (columns-implied SQL) resolves to the insertable columns.
		SourceColumns: session.Columns,
		Mapping:       req.Mapping,
	}
	stream := h.importStream(entry, string(profile.Driver))

	if !execute {
		plan, err := h.manager.ResolveImportPlan(ctx, id, cfg, planReq)
		if err != nil {
			h.importError(w, r, err)
			return
		}
		findings, rowCount, err := h.manager.ValidateImportRows(ctx, plan, 50, stream)
		if err != nil {
			h.importError(w, r, err)
			return
		}
		response.Success(w, model.TableImportValidation{
			Database:   session.Database,
			Schema:     plan.Schema,
			Table:      plan.Table,
			ObjectType: plan.ObjectType,
			Columns:    plan.ColumnInfo,
			Mapping:    plan.Mapping,
			RowCount:   rowCount,
			Valid:      len(findings) == 0,
			Errors:     findings,
		})
		return
	}

	result, err := h.manager.ExecuteImportRows(ctx, id, cfg, planReq, stream)
	if err != nil {
		h.importError(w, r, err)
		return
	}
	response.Success(w, result)
}

// importStream re-parses the spooled file into cells for one stage. Emit is
// called for every data row in order; the parser enforces the same width and
// grammar as the upload stage, and the file was verified unchanged first.
func (h *ConnectionHandler) importStream(entry *importSession, driver string) func(emit func(row []model.ImportCell) error) error {
	return func(emit func(row []model.ImportCell) error) error {
		file, err := os.Open(entry.tempPath)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		switch entry.session.Format {
		case "csv":
			_, _, err := tableimport.ParseCSV(file, tableimport.CSVOptions{HeaderRow: entry.session.HeaderRow}, emit)
			return err
		case "sql":
			_, _, _, err := tableimport.ParseSQLInserts(file, tableimport.SQLOptions{Dialect: driver}, emit)
			return err
		default:
			return fmt.Errorf("unknown import format %q", entry.session.Format)
		}
	}
}

// verifyImportFile re-hashes the spooled upload so a modified or truncated file
// cannot be validated or executed.
func (h *ConnectionHandler) verifyImportFile(entry *importSession) error {
	file, err := os.Open(entry.tempPath)
	if err != nil {
		return errors.New("the uploaded file is no longer available; re-upload")
	}
	defer func() { _ = file.Close() }()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return errors.New("could not re-verify the uploaded file; re-upload")
	}
	if hex.EncodeToString(hasher.Sum(nil)) != entry.session.SHA256 {
		return errors.New("the uploaded file changed since upload; re-upload")
	}
	return nil
}

// bindingMatches reports whether the request echoes the frozen upload target.
func bindingMatches(session model.TableImportSession, database, schema, table string) bool {
	return strings.TrimSpace(database) == session.Database &&
		strings.TrimSpace(schema) == session.Schema &&
		strings.TrimSpace(table) == session.Table
}

// import session registry ---------------------------------------------------

func (h *ConnectionHandler) putImportSession(entry *importSession) {
	h.importMu.Lock()
	defer h.importMu.Unlock()
	h.importSessions[entry.session.ImportID] = entry
}

// getImportSession returns the live session for the connection, or nil when the
// session is unknown, expired, or bound to another connection.
func (h *ConnectionHandler) getImportSession(connectionID, importID string) *importSession {
	h.importMu.Lock()
	defer h.importMu.Unlock()
	entry := h.importSessions[importID]
	if entry == nil || entry.session.ConnectionID != connectionID {
		return nil
	}
	if time.Since(entry.session.CreatedAt) > importSessionTTL {
		delete(h.importSessions, importID)
		_ = os.Remove(entry.tempPath)
		return nil
	}
	return entry
}

// sweepImportSessions removes expired sessions and their temp files.
func (h *ConnectionHandler) sweepImportSessions() {
	h.importMu.Lock()
	defer h.importMu.Unlock()
	for id, entry := range h.importSessions {
		if time.Since(entry.session.CreatedAt) > importSessionTTL {
			delete(h.importSessions, id)
			_ = os.Remove(entry.tempPath)
		}
	}
}

// importError maps import-stage failures to sanitized API errors. Conversion
// failures are validation errors, engine constraints are 409s, and the
// connection/database taxonomy is shared with browsing.
func (h *ConnectionHandler) importError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, database.ErrImportMappingInvalid):
		response.WriteError(w, http.StatusBadRequest, "IMPORT_VALIDATION_ERROR", sanitizeImportError(err))
	case errors.Is(err, database.ErrRowNotMutable):
		response.WriteError(w, http.StatusBadRequest, "ROW_NOT_MUTABLE", "the import target is not writable")
	case errors.Is(err, database.ErrTableNotFound):
		response.WriteError(w, http.StatusNotFound, "TABLE_NOT_FOUND", "the import target table was not found")
	case errors.Is(err, database.ErrConstraintViolation):
		response.WriteError(w, http.StatusConflict, "CONSTRAINT_VIOLATION", sanitizeImportError(err))
	default:
		h.tableDataError(w, r, err)
	}
}

// sanitizeImportError keeps a localized message without leaking DSN/SQL/values.
func sanitizeImportError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
