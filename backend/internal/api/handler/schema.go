package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/repository"
)

// introspectionTimeout bounds a schema introspection request end to end.
const introspectionTimeout = 15 * time.Second

// Schemas godoc
// @Summary      Get database schema
// @Description  Returns a database-neutral schema tree (schemas, tables, columns, primary keys, foreign keys, indexes) for a stored connection.
// @Tags         schema
// @Produce      json
// @Param        id  path      string  true  "Connection id"
// @Success      200  {object}  response.Envelope{data=[]model.Database}
// @Failure      400  {object}  response.ErrorEnvelope
// @Failure      404  {object}  response.ErrorEnvelope
// @Failure      502  {object}  response.ErrorEnvelope
// @Failure      504  {object}  response.ErrorEnvelope
// @Router       /connections/{id}/schemas [get]
// Schemas returns the database-neutral schema tree for a stored connection. It
// activates (or reuses) the managed pool and never opens an unmanaged pool.
func (h *ConnectionHandler) Schemas(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
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

	ctx, cancel := context.WithTimeout(r.Context(), introspectionTimeout)
	defer cancel()

	databases, err := h.manager.Introspect(ctx, id, toDatabaseConfig(profile, password))
	if err != nil {
		h.introspectionError(w, r, err)
		return
	}
	response.Success(w, databases)
}

func (h *ConnectionHandler) introspectionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, database.ErrUnsupportedDriver):
		response.ValidationError(w, "the connection driver is not supported yet")
	case errors.Is(err, database.ErrNotImplemented):
		response.WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
			"schema introspection is not implemented for this driver yet")
	case errors.Is(err, context.DeadlineExceeded):
		h.logger.Warn("schema_introspection_timeout",
			"request_id", chimiddleware.GetReqID(r.Context()),
			"error", err.Error(),
		)
		response.WriteError(w, http.StatusGatewayTimeout, "INTROSPECTION_TIMEOUT",
			"schema introspection timed out")
	default:
		h.logger.Warn("schema_introspection_failed",
			"request_id", chimiddleware.GetReqID(r.Context()),
			"error", err.Error(),
		)
		response.WriteError(w, http.StatusBadGateway, "INTROSPECTION_ERROR",
			"failed to read the database schema")
	}
}
