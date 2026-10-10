package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/datadeck/datadeck/backend/internal/export"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
)

// exportTimeoutSeconds is the budget for a full table export (PRF-02/T14). A
// whole-table export can legitimately stream far more rows than one 30s query
// page, so it gets its own, larger, still-bounded timeout. Client disconnects
// cancel the stream via the request context.
const exportTimeoutSeconds = 300

// exportFileFormats maps a served format to its content type. It mirrors the
// backend capabilities advertisement (all three drivers support csv/xlsx/sql).
var exportFileFormats = map[string]string{
	"csv":  "text/csv",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"sql":  "application/octet-stream",
}

// TableExport godoc
// @Summary      Export table data
// @Description  Streams a whole table (or the current filter set) as CSV, XLSX or an SQL INSERT script — no SQL required. The target is the explicit connection + database + schema + table; global/explorer selection is never used. CSV is served inline as an attachment as it streams; XLSX/SQL are spooled to a temp file and served only when the export completes, so client-visible failures always return a JSON envelope.
// @Tags         table-data
// @Accept       json
// @Produce      json,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/octet-stream
// @Param        id   path     string                    true "Connection id"
// @Param        body body     model.TableExportRequest   true "Export options"
// @Success      200  {file}   binary
// @Failure      400  {object} response.ErrorEnvelope
// @Failure      404  {object} response.ErrorEnvelope
// @Failure      500  {object} response.ErrorEnvelope
// @Failure      502  {object} response.ErrorEnvelope
// @Failure      504  {object} response.ErrorEnvelope
// @Router       /connections/{id}/table-data/export [post]
func (h *ConnectionHandler) TableExport(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}

	var body model.TableExportRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&body); err != nil {
		response.ValidationError(w, "invalid request body")
		return
	}

	format := strings.ToLower(strings.TrimSpace(body.Format))
	contentType, ok := exportFileFormats[format]
	if !ok {
		response.WriteError(w, http.StatusBadRequest, "EXPORT_FORMAT_UNSUPPORTED", "format must be one of: csv, xlsx, sql")
		return
	}
	body.Table = strings.TrimSpace(body.Table)
	if body.Table == "" {
		response.ValidationError(w, "table is required")
		return
	}
	body.Schema = strings.TrimSpace(body.Schema)
	body.Database = strings.TrimSpace(body.Database)
	if len(body.Filters) > maxTableFilters {
		response.ValidationError(w, fmt.Sprintf("at most %d filters are allowed", maxTableFilters))
		return
	}

	var sort *model.TableSort
	if body.Sort != nil && body.Sort.Column != "" {
		direction := strings.ToLower(strings.TrimSpace(body.Sort.Direction))
		if direction != "asc" && direction != "desc" {
			response.ValidationError(w, "sort_direction must be one of: asc, desc")
			return
		}
		sort = &model.TableSort{Column: strings.TrimSpace(body.Sort.Column), Direction: direction}
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

	if profile.Driver == model.DriverPostgres && body.Schema == "" {
		response.ValidationError(w, "schema is required for postgresql table export")
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

	effectiveDatabase, err := resolveTargetDatabase(profile, body.Database)
	if err != nil {
		writeResolveDatabaseError(w, err)
		return
	}

	includeHeaders := true
	if body.IncludeHeaders != nil {
		includeHeaders = *body.IncludeHeaders
	}
	transaction := true
	if body.Transaction != nil {
		transaction = *body.Transaction
	}

	req := model.TableExportRequest{
		Database: effectiveDatabase,
		Format:   format,
		Schema:   body.Schema,
		Table:    body.Table,
		Sort:     sort,
		Filters:  body.Filters,
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(exportTimeoutSeconds)*time.Second)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase

	filename := exportFilename(effectiveDatabase, req.Schema, req.Table, format)

	// CSV streams straight to the response. The lazy header writer keeps
	// pre-stream validation failures returning a JSON envelope.
	if format == "csv" {
		committed := false
		lazy := &lazyHeaderWriter{w: w, contentType: contentType, filename: filename, committed: &committed}
		csvErr := h.runExport(ctx, id, cfg, req, export.NewCSV(lazy, includeHeaders))
		h.exportError(w, r, csvErr, func() bool { return committed })
		return
	}

	// XLSX/SQL spool to a temp file and are served only on success; on failure
	// the file is removed and a JSON envelope is returned.
	tmp, err := os.CreateTemp("", "datadeck-export-*")
	if err != nil {
		h.internalError(w, r, "create_export_temp", err)
		return
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	bufw := bufio.NewWriter(tmp)

	var sink database.ExportSink
	switch format {
	case "xlsx":
		sink = export.NewXLSX(bufw)
	case "sql":
		sink = export.NewSQL(bufw, export.SQLOptions{
			Driver:      string(cfg.Driver),
			Schema:      req.Schema,
			Table:       req.Table,
			Transaction: transaction,
		})
	}
	if err := h.runExport(ctx, id, cfg, req, sink); err != nil {
		h.exportError(w, r, err, nil)
		return
	}
	if err := bufw.Flush(); err != nil {
		h.exportError(w, r, err, nil)
		return
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		h.internalError(w, r, "rewind_export_temp", err)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, tmp); err != nil {
		// Headers are committed; there is nothing left to send but a log line.
		h.logger.Warn("export_response_write",
			slog.String("request_id", chimiddleware.GetReqID(r.Context())),
			slog.String("error", err.Error()),
		)
	}
}

// runExport drives one sink to completion: stream all rows, then finalize the
// sink (flush csv, write the workbook, COMMIT the script). End is deliberately
// skipped when the stream failed so partial output is never committed.
func (h *ConnectionHandler) runExport(ctx context.Context, id string, cfg database.Config, req model.TableExportRequest, sink database.ExportSink) error {
	if err := h.manager.ExportTable(ctx, id, cfg, req, sink); err != nil {
		return err
	}
	return sink.End()
}

// lazyHeaderWriter commits the download headers only when the first payload
// byte is written. Until then, handlers can still return a JSON error
// envelope; after that the status is 200 and only server-side logging makes
// sense.
type lazyHeaderWriter struct {
	w           http.ResponseWriter
	contentType string
	filename    string
	committed   *bool
}

func (l *lazyHeaderWriter) Write(p []byte) (int, error) {
	if !*l.committed {
		*l.committed = true
		l.w.Header().Set("Content-Type", l.contentType)
		l.w.Header().Set("Content-Disposition", `attachment; filename="`+l.filename+`"`)
		l.w.Header().Set("Cache-Control", "no-store")
		l.w.WriteHeader(http.StatusOK)
	}
	return l.w.Write(p)
}

// exportError maps export failures to sanitized API errors, reusing the
// browsing taxonomy where applicable. committed reports whether the response
// headers were already sent (CSV streaming): in that case the failure is only
// logged.
func (h *ConnectionHandler) exportError(w http.ResponseWriter, r *http.Request, err error, committed func() bool) {
	if err == nil {
		return
	}
	if committed != nil && committed() {
		h.logger.Warn("export_aborted_after_stream",
			slog.String("request_id", chimiddleware.GetReqID(r.Context())),
			slog.String("error", err.Error()),
		)
		return
	}
	switch {
	case errors.Is(err, database.ErrExportFormatUnsupported):
		response.WriteError(w, http.StatusBadRequest, "EXPORT_FORMAT_UNSUPPORTED", "sql export requires a base table")
	case errors.Is(err, database.ErrNoExportableColumns):
		response.WriteError(w, http.StatusBadRequest, "EXPORT_FORMAT_UNSUPPORTED", "the table has no exportable columns")
	case errors.Is(err, export.ErrRowLimit):
		response.WriteError(w, http.StatusBadRequest, "EXPORT_LIMIT_EXCEEDED", "the table exceeds the spreadsheet row limit")
	default:
		h.tableDataError(w, r, err)
	}
}

// exportFilename builds a sanitized attachment filename:
// {database}_{schema}_{table}_{YYYYMMDD}.{format}.
func exportFilename(database, schema, table, format string) string {
	parts := []string{sanitizeFilenamePart(database)}
	if schema != "" {
		parts = append(parts, sanitizeFilenamePart(schema))
	}
	parts = append(parts, sanitizeFilenamePart(table), time.Now().Format("20060102"))
	return strings.Join(parts, "_") + "." + format
}

// sanitizeFilenamePart keeps [A-Za-z0-9._-], collapses everything else into a
// single underscore separator, and caps the part at 64 characters. It matches
// the frontend's filename sanitizer so browser-saved names stay predictable.
func sanitizeFilenamePart(part string) string {
	var b strings.Builder
	b.Grow(len(part))
	lastSep := false
	for _, r := range part {
		valid := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !valid {
			if b.Len() > 0 && !lastSep {
				b.WriteByte('_')
				lastSep = true
			}
			continue
		}
		if b.Len() >= 64 {
			break
		}
		b.WriteRune(r)
		lastSep = false
	}
	name := strings.Trim(b.String(), "_.")
	if name == "" {
		return "export"
	}
	return name
}