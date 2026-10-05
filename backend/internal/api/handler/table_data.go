package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
)

// TableData godoc
// @Summary      Browse table data
// @Description  Returns one bounded, paginated page of rows from a specific table without requiring the caller to write SQL. The target is the explicit connection + database + schema + table; global/explorer selection is never used.
// @Tags         table-data
// @Produce      json
// @Param        id         path      string  true   "Connection id"
// @Param        database   query     string  false  "Target database (required for server-level PostgreSQL)"
// @Param        schema     query     string  false  "Schema (required for PostgreSQL)"
// @Param        table      query     string  true   "Table or view name"
// @Param        page       query     int     false  "1-based page number (default 1)"
// @Param        page_size  query     int     false  "Rows per page (default 100, max 200)"
// @Param        sort_column query    string  false  "Column to sort by (validated against metadata)"
// @Param        sort_direction query string false  "Sort direction" Enums(asc, desc)
// @Success      200  {object}  response.Envelope{data=model.TableDataPage}
// @Failure      400  {object}  response.ErrorEnvelope
// @Failure      404  {object}  response.ErrorEnvelope
// @Failure      502  {object}  response.ErrorEnvelope
// @Failure      504  {object}  response.ErrorEnvelope
// @Router       /connections/{id}/table-data [get]
// TableData resolves the relation through introspection metadata and generates a
// bounded, dialect-quoted SELECT. It never accepts SQL, a WHERE clause, or a
// sort expression from the client (those arrive in PRF02-T05/T06).
func (h *ConnectionHandler) TableData(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}

	table := strings.TrimSpace(r.URL.Query().Get("table"))
	if table == "" {
		response.ValidationError(w, "table is required")
		return
	}
	schema := strings.TrimSpace(r.URL.Query().Get("schema"))

	params, err := parseTablePageParams(r)
	if err != nil {
		response.ValidationError(w, err.Error())
		return
	}

	sortSpec, err := parseTableSort(r)
	if err != nil {
		response.ValidationError(w, err.Error())
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

	if profile.Driver == model.DriverPostgres && schema == "" {
		response.ValidationError(w, "schema is required for postgresql table browsing")
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

	effectiveDatabase, err := resolveTargetDatabase(profile, r.URL.Query().Get("database"))
	if err != nil {
		writeResolveDatabaseError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(defaultQueryTimeoutSeconds)*time.Second)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase

	page, err := h.manager.BrowseTable(ctx, id, cfg, database.TableBrowseRequest{
		Schema: schema,
		Table:  table,
		Limit:  params.PageSize,
		Offset: params.Offset,
		Sort:   sortSpec,
	})
	if err != nil {
		h.tableDataError(w, r, err)
		return
	}

	page.Database = effectiveDatabase
	page.Pagination = model.TablePagination{
		Page:     params.Page,
		PageSize: params.PageSize,
		HasMore:  page.Pagination.HasMore,
	}

	response.SuccessWithMeta(w, page, map[string]any{
		"page":      params.Page,
		"page_size": params.PageSize,
		"has_more":  page.Pagination.HasMore,
	})
}

// parseTableSort validates the structured sort query parameters. Both are
// optional; when neither is present there is no sort. When present, the
// direction must be exactly asc/desc (the column is verified against metadata
// later, after introspection).
func parseTableSort(r *http.Request) (*model.TableSort, error) {
	column := strings.TrimSpace(r.URL.Query().Get("sort_column"))
	direction := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_direction")))
	if column == "" && direction == "" {
		return nil, nil
	}
	if column == "" {
		return nil, errors.New("sort_column is required when sort_direction is set")
	}
	if direction == "" {
		return nil, errors.New("sort_direction is required when sort_column is set")
	}
	if direction != "asc" && direction != "desc" {
		return nil, errors.New("sort_direction must be one of: asc, desc")
	}
	return &model.TableSort{Column: column, Direction: direction}, nil
}

// tableDataError maps browse failures to sanitized API errors. It reuses the
// existing connection/database taxonomy and never returns a driver DSN or SQL.
func (h *ConnectionHandler) tableDataError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, database.ErrSortColumnNotFound):
		response.ValidationError(w, "unknown sort column")
	case errors.Is(err, database.ErrTableNotFound):
		response.WriteError(w, http.StatusNotFound, "TABLE_NOT_FOUND", "the requested table was not found")
	case errors.Is(err, database.ErrDatabaseNotFound):
		response.WriteError(w, http.StatusBadRequest, "DATABASE_NOT_FOUND", "the requested database does not exist")
	case errors.Is(err, database.ErrDatabaseConnectDenied):
		response.WriteError(w, http.StatusBadRequest, "DATABASE_CONNECT_DENIED", "the connection has no CONNECT permission on that database")
	case errors.Is(err, database.ErrNoBootstrapDatabase):
		response.WriteError(w, http.StatusBadGateway, "BOOTSTRAP_DATABASE_UNAVAILABLE", "no usable database: select one explicitly")
	case errors.Is(err, database.ErrUnsupportedDriver):
		response.ValidationError(w, "the connection driver is not supported yet")
	case errors.Is(err, database.ErrNotImplemented):
		response.WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "table browsing is not implemented for this driver yet")
	case errors.Is(err, context.DeadlineExceeded):
		response.WriteError(w, http.StatusGatewayTimeout, "QUERY_TIMEOUT", "table browsing exceeded the timeout")
	case errors.Is(err, database.ErrConnection):
		response.WriteError(w, http.StatusBadGateway, "CONNECTION_ERROR", "unable to connect to the database")
	default:
		writeInternalError(h.logger, w, r, "table_data_failed", err)
	}
}
