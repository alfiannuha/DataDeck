package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
)

// TableStructure godoc
// @Summary      Get table structure
// @Description  Returns the read-only structure of one table or view (columns, primary key, indexes, constraints, foreign keys) for the explicit connection + database + schema + table. It never reads a row of data and never mutates anything.
// @Tags         table-structure
// @Produce      json
// @Param        id        path     string  true  "Connection id"
// @Param        database  query    string  false  "Target database (required for server-level PostgreSQL)"
// @Param        schema    query    string  false  "Schema (required for PostgreSQL)"
// @Param        table     query    string  true   "Table or view name"
// @Success      200  {object}  response.Envelope{data=model.TableStructure}
// @Failure      400  {object}  response.ErrorEnvelope
// @Failure      404  {object}  response.ErrorEnvelope
// @Failure      501  {object}  response.ErrorEnvelope
// @Failure      502  {object}  response.ErrorEnvelope
// @Failure      504  {object}  response.ErrorEnvelope
// @Router       /connections/{id}/table-structure [get]
// TableStructure reads metadata for only the requested relation through the
// driver's bounded StructureReader. No identifier from the client is ever
// concatenated into the catalog queries; all values are bound parameters.
func (h *ConnectionHandler) TableStructure(w http.ResponseWriter, r *http.Request) {
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
		response.ValidationError(w, "schema is required for postgresql table structure")
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

	ctx, cancel := context.WithTimeout(r.Context(), introspectionTimeout)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase

	structure, err := h.manager.TableStructure(ctx, id, cfg, schema, table)
	if err != nil {
		h.tableStructureError(w, r, err)
		return
	}
	response.Success(w, structure)
}

// tableStructureError maps structure failures to sanitized API errors. Metadata
// failures reuse the introspection taxonomy (INTROSPECTION_TIMEOUT /
// INTROSPECTION_ERROR); a missing relation is a 404. No driver DSN or SQL is
// ever returned.
func (h *ConnectionHandler) tableStructureError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, database.ErrTableNotFound):
		response.WriteError(w, http.StatusNotFound, "TABLE_NOT_FOUND", "the requested table was not found")
	case errors.Is(err, database.ErrConnection):
		response.WriteError(w, http.StatusBadGateway, "CONNECTION_ERROR", "unable to connect to the database")
	default:
		h.introspectionError(w, r, err)
	}
}
