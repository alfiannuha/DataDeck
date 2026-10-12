package handler

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/dbscript"
	"github.com/datadeck/datadeck/backend/internal/export"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
)

// databaseScriptTimeoutSeconds bounds a database-level export/restore
// (PRF-02/T17): larger than a single table but still bounded. Client
// disconnects cancel the work via the request context.
const databaseScriptTimeoutSeconds = 600

// DatabaseExportRequest is the body for POST /connections/{id}/database-export.
type DatabaseExportRequest struct {
	Database string `json:"database" example:"alpha"`
	Mode     string `json:"mode" example:"schema_data" enums:"schema_data,schema,data"`
}

// DatabaseExport godoc
// @Summary      Export a database as a SQL script
// @Description  Streams a dialect SQL script for the base tables of the explicit database (schema+data / schema-only / data-only). Self-contained: no external pg_dump/mysqldump tooling. Views and other objects are excluded and listed in the artifact header. The target is the explicit connection + database; global/explorer selection is never used.
// @Tags         database
// @Accept       json
// @Produce      application/octet-stream
// @Param        id   path     string                  true "Connection id"
// @Param        body body     handler.DatabaseExportRequest true "Export options"
// @Success      200  {file}   binary
// @Failure      400  {object} response.ErrorEnvelope
// @Failure      404  {object} response.ErrorEnvelope
// @Failure      500  {object} response.ErrorEnvelope
// @Failure      502  {object} response.ErrorEnvelope
// @Router       /connections/{id}/database-export [post]
func (h *ConnectionHandler) DatabaseExport(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}

	var body DatabaseExportRequest
	if !decodeJSON(w, r, &body) {
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

	ctx, cancel := context.WithTimeout(r.Context(), databaseScriptTimeoutSeconds*time.Second)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase
	source := &databaseScriptSource{
		handler: h,
		id:      id,
		cfg:     cfg,
		driver:  string(profile.Driver),
	}

	manifest := dbscript.Manifest{
		Engine:      string(profile.Driver),
		Database:    effectiveDatabase,
		Mode:        mode,
		GeneratedAt: time.Now(),
		Excluded:    []string{"views", "sequences", "triggers", "functions", "extensions"},
	}

	// Spool to a temp file so a mid-export failure still returns a JSON envelope
	// rather than a truncated download.
	tmp, err := os.CreateTemp("", "datadeck-dbexport-*")
	if err != nil {
		h.internalError(w, r, "create_dbexport_temp", err)
		return
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	bufw := bufio.NewWriter(tmp)

	if err := dbscript.Export(ctx, bufw, manifest, source); err != nil {
		h.databaseScriptError(w, r, err, "database_export_failed")
		return
	}
	if err := bufw.Flush(); err != nil {
		h.databaseScriptError(w, r, err, "database_export_flush")
		return
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		h.internalError(w, r, "rewind_dbexport_temp", err)
		return
	}

	filename := sanitizeFilenamePart(effectiveDatabase) + "_" + string(mode) + "_" +
		time.Now().Format("20060102") + ".sql"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, tmp); err != nil {
		h.logger.Warn("database_export_response_write",
			slog.String("request_id", chimiddleware.GetReqID(r.Context())),
			slog.String("error", err.Error()),
		)
	}
}

// databaseScriptSource adapts the connection manager to dbscript.Source.
type databaseScriptSource struct {
	handler *ConnectionHandler
	id      string
	cfg     database.Config
	driver  string
}

func (s *databaseScriptSource) Tables(ctx context.Context) ([]dbscript.TableRef, error) {
	databases, err := s.handler.manager.Introspect(ctx, s.id, s.cfg)
	if err != nil {
		return nil, err
	}
	return collectBaseTables(databases), nil
}

func (s *databaseScriptSource) Structure(ctx context.Context, ref dbscript.TableRef) (model.TableStructure, error) {
	return s.handler.manager.TableStructure(ctx, s.id, s.cfg, ref.Schema, ref.Name)
}

func (s *databaseScriptSource) Data(ctx context.Context, ref dbscript.TableRef, w io.Writer) error {
	req := model.TableExportRequest{
		Database: s.cfg.Database,
		Schema:   ref.Schema,
		Table:    ref.Name,
		Format:   "sql",
	}
	sink := export.NewSQL(w, export.SQLOptions{
		Driver:      s.driver,
		Schema:      ref.Schema,
		Table:       ref.Name,
		Transaction: false,
	})
	return s.handler.manager.ExportTable(ctx, s.id, s.cfg, req, sink)
}

// collectBaseTables flattens an introspection tree to its base tables.
func collectBaseTables(databases []model.Database) []dbscript.TableRef {
	refs := []dbscript.TableRef{}
	for _, db := range databases {
		for _, schema := range db.Schemas {
			for _, table := range schema.Tables {
				if isBaseTableType(table.Type) {
					refs = append(refs, dbscript.TableRef{Schema: schema.Name, Name: table.Name})
				}
			}
		}
		for _, table := range db.Tables {
			if isBaseTableType(table.Type) {
				refs = append(refs, dbscript.TableRef{Name: table.Name})
			}
		}
	}
	return refs
}

func isBaseTableType(tableType string) bool {
	upper := strings.ToUpper(tableType)
	return strings.Contains(upper, "BASE TABLE") || strings.Contains(upper, "PARTITIONED TABLE")
}

// resolveScriptTarget resolves the profile and effective database for a
// database-level script operation. It writes the appropriate error and returns
// ok=false on failure.
func (h *ConnectionHandler) resolveScriptTarget(w http.ResponseWriter, r *http.Request, id, databaseName string) (*model.ConnectionProfile, string, string, bool) {
	profile, err := h.connections.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "connection profile not found")
			return nil, "", "", false
		}
		h.internalError(w, r, "get_connection", err)
		return nil, "", "", false
	}

	password := ""
	if profile.EncryptedPassword != nil {
		plaintext, err := h.cipher.Decrypt(*profile.EncryptedPassword)
		if err != nil {
			h.internalError(w, r, "decrypt_password", err)
			return nil, "", "", false
		}
		password = string(plaintext)
	}

	effectiveDatabase, err := resolveTargetDatabase(profile, databaseName)
	if err != nil {
		writeResolveDatabaseError(w, err)
		return nil, "", "", false
	}
	return profile, password, effectiveDatabase, true
}

// databaseScriptError maps export/restore failures to sanitized API errors.
func (h *ConnectionHandler) databaseScriptError(w http.ResponseWriter, r *http.Request, err error, action string) {
	var sqlErr *database.SQLError
	switch {
	case errors.As(err, &sqlErr):
		response.WriteError(w, http.StatusBadRequest, "RESTORE_ERROR", sqlErr.Message)
	case errors.Is(err, database.ErrNotImplemented):
		response.WriteError(w, http.StatusNotImplemented, "RESTORE_UNSUPPORTED", "this database engine does not support SQL restore")
	case errors.Is(err, database.ErrConnection):
		response.WriteError(w, http.StatusBadGateway, "CONNECTION_ERROR", "unable to connect to the database")
	case errors.Is(err, context.DeadlineExceeded):
		response.WriteError(w, http.StatusGatewayTimeout, "QUERY_TIMEOUT", "the operation exceeded its timeout")
	case errors.Is(err, context.Canceled):
		response.WriteError(w, 499, "QUERY_CANCELED", "the operation was canceled")
	default:
		h.logger.Error(action,
			slog.String("request_id", chimiddleware.GetReqID(r.Context())),
			slog.String("error", err.Error()),
		)
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "the operation failed")
	}
}
