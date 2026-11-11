package handler

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/dbscript"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/version"
)

// backupTimeoutSeconds bounds a backup operation (PRF-04/Phase 3). A
// connection-scope backup exports every database, so it is given the same
// generous budget as a database script.
const backupTimeoutSeconds = databaseScriptTimeoutSeconds

// TableBackupRequest is the body for POST /connections/{id}/table-backup.
type TableBackupRequest struct {
	Database string `json:"database" example:"alpha"`
	Schema   string `json:"schema" example:"public"`
	Table    string `json:"table" binding:"required" example:"orders"`
	Mode     string `json:"mode" example:"schema_data" enums:"schema_data,schema,data"`
}

// TableBackup godoc
// @Summary      Back up a single table as a SQL script
// @Description  Streams a self-contained DataDeck SQL artifact for exactly one table (structure + indexes + constraints/foreign keys + data, per mode). No external tooling is invoked. The artifact manifest records the DataDeck version, engine, database, scope=table and the table name; it restores through the database-restore path into an empty target.
// @Tags         backup
// @Accept       json
// @Produce      application/octet-stream
// @Param        id    path  string                     true "Connection id"
// @Param        body  body  handler.TableBackupRequest true "Backup options"
// @Success      200   {file} binary
// @Failure      400   {object} response.ErrorEnvelope
// @Failure      404   {object} response.ErrorEnvelope
// @Failure      502   {object} response.ErrorEnvelope
// @Failure      504   {object} response.ErrorEnvelope
// @Router       /connections/{id}/table-backup [post]
func (h *ConnectionHandler) TableBackup(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}

	var body TableBackupRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	table := strings.TrimSpace(body.Table)
	if table == "" {
		response.ValidationError(w, "table is required")
		return
	}
	mode, err := dbscript.ParseMode(body.Mode)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "EXPORT_FORMAT_UNSUPPORTED", err.Error())
		return
	}

	profile, password, effectiveDatabase, ok := h.resolveScriptTarget(w, r, id, body.Database)
	if !ok {
		return
	}
	schema := strings.TrimSpace(body.Schema)
	if profile.Driver == model.DriverPostgres && schema == "" {
		response.ValidationError(w, "schema is required for postgresql table backup")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), backupTimeoutSeconds*time.Second)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase
	source := &databaseScriptSource{
		handler: h,
		id:      id,
		cfg:     cfg,
		driver:  string(profile.Driver),
		only:    &dbscript.TableRef{Schema: schema, Name: table},
	}
	manifest := dbscript.Manifest{
		Engine:          string(profile.Driver),
		Database:        effectiveDatabase,
		Mode:            mode,
		GeneratedAt:     time.Now(),
		Scope:           dbscript.ScopeTable,
		Table:           table,
		DataDeckVersion: version.Version,
		Excluded:        []string{"views", "sequences", "triggers", "functions", "extensions"},
	}

	h.streamScript(ctx, w, r, source, manifest,
		sanitizeFilenamePart(table)+"_backup_"+string(mode)+"_"+time.Now().Format("20060102")+".sql")
}

// ConnectionBackupRequest is the body for POST /connections/{id}/connection-backup.
type ConnectionBackupRequest struct {
	Mode string `json:"mode" example:"schema_data" enums:"schema_data,schema,data"`
}

// ConnectionBackup godoc
// @Summary      Back up an entire connection as a zip of per-database SQL scripts
// @Description  Streams a zip archive containing one DataDeck SQL artifact per database visible to the connection (PostgreSQL: all accessible databases; MySQL/SQLite: the single profile database), plus a manifest.json describing the backup. No external tooling is invoked. Connection-scope restore is not yet supported; each contained script restores through the database-restore path.
// @Tags         backup
// @Accept       json
// @Produce      application/zip
// @Param        id    path  string                          true "Connection id"
// @Param        body  body  handler.ConnectionBackupRequest true "Backup options"
// @Success      200   {file} binary
// @Failure      400   {object} response.ErrorEnvelope
// @Failure      404   {object} response.ErrorEnvelope
// @Failure      501   {object} response.ErrorEnvelope
// @Failure      502   {object} response.ErrorEnvelope
// @Failure      504   {object} response.ErrorEnvelope
// @Router       /connections/{id}/connection-backup [post]
func (h *ConnectionHandler) ConnectionBackup(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}

	var body ConnectionBackupRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	mode, err := dbscript.ParseMode(body.Mode)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "EXPORT_FORMAT_UNSUPPORTED", err.Error())
		return
	}

	profile, password, ok := h.loadProfilePassword(w, r, id)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), backupTimeoutSeconds*time.Second)
	defer cancel()

	databases, err := h.connectionDatabases(ctx, id, profile, password)
	if err != nil {
		h.backupError(w, r, err, "connection_backup_list_databases")
		return
	}
	if len(databases) == 0 {
		response.WriteError(w, http.StatusBadGateway, "CONNECTION_ERROR", "no databases are accessible on this connection")
		return
	}

	// Spool the whole archive to a temp file so a mid-backup failure still
	// returns a JSON envelope instead of a truncated download.
	tmp, err := os.CreateTemp("", "datadeck-connbackup-*")
	if err != nil {
		h.internalError(w, r, "create_backup_temp", err)
		return
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	zw := zip.NewWriter(tmp)
	manifest := connectionBackupManifest{
		FormatVersion:   dbscript.FormatVersion,
		Scope:           dbscript.ScopeConnection,
		Engine:          string(profile.Driver),
		Databases:       databases,
		Mode:            string(mode),
		GeneratedAt:     time.Now().UTC(),
		DataDeckVersion: version.Version,
		Excluded:        []string{"views", "sequences", "triggers", "functions", "extensions"},
	}
	if err := writeJSONEntry(zw, "manifest.json", manifest); err != nil {
		h.internalError(w, r, "write_backup_manifest", err)
		return
	}

	for _, databaseName := range databases {
		cfg := toDatabaseConfig(profile, password)
		cfg.Database = databaseName
		source := &databaseScriptSource{handler: h, id: id, cfg: cfg, driver: string(profile.Driver)}
		dbManifest := dbscript.Manifest{
			Engine:          string(profile.Driver),
			Database:        databaseName,
			Mode:            mode,
			GeneratedAt:     time.Now(),
			Scope:           dbscript.ScopeDatabase,
			DataDeckVersion: version.Version,
			Excluded:        []string{"views", "sequences", "triggers", "functions", "extensions"},
		}
		var buf bytes.Buffer
		if err := dbscript.Export(ctx, &buf, dbManifest, source); err != nil {
			h.backupError(w, r, err, "connection_backup_export")
			return
		}
		entry, err := zw.Create(sanitizeFilenamePart(databaseName) + ".sql")
		if err != nil {
			h.internalError(w, r, "write_backup_entry", err)
			return
		}
		if _, err := entry.Write(buf.Bytes()); err != nil {
			h.internalError(w, r, "write_backup_entry", err)
			return
		}
	}
	if err := zw.Close(); err != nil {
		h.internalError(w, r, "close_backup_archive", err)
		return
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		h.internalError(w, r, "rewind_backup_temp", err)
		return
	}

	filename := sanitizeFilenamePart(profile.Name) + "_backup_" + time.Now().Format("20060102") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, tmp); err != nil {
		h.logger.Warn("connection_backup_response_write", "error", err.Error())
	}
}

// connectionBackupManifest is the manifest.json embedded in a connection-scope
// backup archive.
type connectionBackupManifest struct {
	FormatVersion   int       `json:"format_version"`
	Scope           string    `json:"scope"`
	Engine          string    `json:"engine"`
	Databases       []string  `json:"databases"`
	Mode            string    `json:"mode"`
	GeneratedAt     time.Time `json:"generated_at"`
	DataDeckVersion string    `json:"datadeck_version"`
	Excluded        []string  `json:"excluded,omitempty"`
}

// connectionDatabases returns the databases to back up for a connection:
// every accessible database for PostgreSQL, or the single profile database for
// MySQL/SQLite.
func (h *ConnectionHandler) connectionDatabases(ctx context.Context, id string, profile *model.ConnectionProfile, password string) ([]string, error) {
	if profile.Driver != model.DriverPostgres {
		if strings.TrimSpace(profile.DatabaseName) == "" {
			return nil, errors.New("the connection has no database to back up")
		}
		return []string{profile.DatabaseName}, nil
	}
	infos, err := h.manager.ListDatabases(ctx, id, toDatabaseConfig(profile, password))
	if err != nil {
		return nil, err
	}
	databases := make([]string, 0, len(infos))
	seen := map[string]bool{}
	for _, info := range infos {
		if name := strings.TrimSpace(info.Name); name != "" && !seen[name] {
			seen[name] = true
			databases = append(databases, name)
		}
	}
	return databases, nil
}

// loadProfilePassword loads a profile and decrypts its password without
// resolving a single target database (used by connection-scope operations).
func (h *ConnectionHandler) loadProfilePassword(w http.ResponseWriter, r *http.Request, id string) (*model.ConnectionProfile, string, bool) {
	profile, err := h.connections.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "connection profile not found")
			return nil, "", false
		}
		h.internalError(w, r, "get_connection", err)
		return nil, "", false
	}
	password := ""
	if profile.EncryptedPassword != nil {
		plaintext, err := h.cipher.Decrypt(*profile.EncryptedPassword)
		if err != nil {
			h.internalError(w, r, "decrypt_password", err)
			return nil, "", false
		}
		password = string(plaintext)
	}
	return profile, password, true
}

// streamScript spools a dbscript export to a temp file and streams it as a
// download, so a mid-export failure returns a JSON envelope instead of a
// truncated file.
func (h *ConnectionHandler) streamScript(ctx context.Context, w http.ResponseWriter, r *http.Request, source *databaseScriptSource, manifest dbscript.Manifest, filename string) {
	tmp, err := os.CreateTemp("", "datadeck-backup-*")
	if err != nil {
		h.internalError(w, r, "create_backup_temp", err)
		return
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	bufw := bufio.NewWriter(tmp)

	if err := dbscript.Export(ctx, bufw, manifest, source); err != nil {
		h.backupError(w, r, err, "table_backup_export")
		return
	}
	if err := bufw.Flush(); err != nil {
		h.backupError(w, r, err, "table_backup_flush")
		return
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		h.internalError(w, r, "rewind_backup_temp", err)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, tmp); err != nil {
		h.logger.Warn("table_backup_response_write", "error", err.Error())
	}
}

func writeJSONEntry(zw *zip.Writer, name string, value any) error {
	entry, err := zw.Create(name)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(entry)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// backupError maps backup failures to sanitized API errors.
func (h *ConnectionHandler) backupError(w http.ResponseWriter, r *http.Request, err error, action string) {
	var sqlErr *database.SQLError
	switch {
	case errors.As(err, &sqlErr):
		response.WriteError(w, http.StatusBadRequest, "SQL_ERROR", sqlErr.Message)
	case errors.Is(err, database.ErrNoBootstrapDatabase):
		response.WriteError(w, http.StatusBadGateway, "BOOTSTRAP_DATABASE_UNAVAILABLE", "no bootstrap database is reachable; set a default database")
	case errors.Is(err, database.ErrNotImplemented):
		response.WriteError(w, http.StatusNotImplemented, "BACKUP_UNSUPPORTED", "this engine does not support the requested backup")
	case errors.Is(err, database.ErrConnection):
		response.WriteError(w, http.StatusBadGateway, "CONNECTION_ERROR", "unable to connect to the database")
	case errors.Is(err, context.DeadlineExceeded):
		response.WriteError(w, http.StatusGatewayTimeout, "QUERY_TIMEOUT", "the backup exceeded its timeout")
	case errors.Is(err, context.Canceled):
		response.WriteError(w, 499, "QUERY_CANCELED", "the backup was canceled")
	default:
		h.internalError(w, r, action, err)
	}
}
