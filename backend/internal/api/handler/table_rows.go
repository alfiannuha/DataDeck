package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
)

// RowMutationRequest is the body for a single-row UPDATE/DELETE (PRF-02/T07).
// The target is explicit; identity/expected/changes are structured. There is no
// raw WHERE/SQL field. Numbers decode via json.Number so BIGINT stays exact.
type RowMutationRequest struct {
	Database string         `json:"database" example:"alpha"`
	Schema   string         `json:"schema" example:"public"`
	Table    string         `json:"table" example:"users"`
	Identity map[string]any `json:"identity" example:"{\"id\":\"10\"}"`
	Expected map[string]any `json:"expected,omitempty"`
	Changes  map[string]any `json:"changes,omitempty"`
}

// UpdateRow godoc
// @Summary      Update one row
// @Description  Applies a single-row, primary-key-identified UPDATE with optional optimistic-concurrency expected values. Values are bound; affected-row safety is enforced (0 = conflict/not found, >1 = abort).
// @Tags         table-data
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "Connection id"
// @Param        body  body      handler.RowMutationRequest  true  "Mutation target, identity, expected and changes"
// @Success      200  {object}  response.Envelope{data=model.RowMutationResult}
// @Failure      400  {object}  response.ErrorEnvelope
// @Failure      404  {object}  response.ErrorEnvelope
// @Failure      409  {object}  response.ErrorEnvelope
// @Failure      502  {object}  response.ErrorEnvelope
// @Failure      504  {object}  response.ErrorEnvelope
// @Router       /connections/{id}/table-data/rows [patch]
func (h *ConnectionHandler) UpdateRow(w http.ResponseWriter, r *http.Request) {
	h.mutateRow(w, r, true)
}

// DeleteRow godoc
// @Summary      Delete one row
// @Description  Applies a single-row, primary-key-identified DELETE with optional optimistic-concurrency expected values. Affected-row safety is enforced.
// @Tags         table-data
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "Connection id"
// @Param        body  body      handler.RowMutationRequest  true  "Mutation target, identity and expected"
// @Success      200  {object}  response.Envelope{data=model.RowMutationResult}
// @Failure      400  {object}  response.ErrorEnvelope
// @Failure      404  {object}  response.ErrorEnvelope
// @Failure      409  {object}  response.ErrorEnvelope
// @Failure      502  {object}  response.ErrorEnvelope
// @Failure      504  {object}  response.ErrorEnvelope
// @Router       /connections/{id}/table-data/rows [delete]
func (h *ConnectionHandler) DeleteRow(w http.ResponseWriter, r *http.Request) {
	h.mutateRow(w, r, false)
}

func (h *ConnectionHandler) mutateRow(w http.ResponseWriter, r *http.Request, update bool) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		response.ValidationError(w, "connection id is required")
		return
	}

	var body RowMutationRequest
	if !decodeMutationBody(w, r, &body) {
		return
	}
	body.Table = strings.TrimSpace(body.Table)
	body.Schema = strings.TrimSpace(body.Schema)
	if body.Table == "" {
		response.ValidationError(w, "table is required")
		return
	}
	if len(body.Identity) == 0 {
		response.WriteError(w, http.StatusBadRequest, "ROW_IDENTITY_REQUIRED", "a row identity is required")
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
	if profile.Driver == model.DriverPostgres && body.Schema == "" {
		response.ValidationError(w, "schema is required for postgresql row mutations")
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

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(defaultQueryTimeoutSeconds)*time.Second)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase
	mutation := database.RowMutationRequest{
		Schema:   body.Schema,
		Table:    body.Table,
		Identity: body.Identity,
		Expected: body.Expected,
		Changes:  body.Changes,
	}

	var result model.RowMutationResult
	if update {
		result, err = h.manager.UpdateRow(ctx, id, cfg, mutation)
	} else {
		result, err = h.manager.DeleteRow(ctx, id, cfg, mutation)
	}
	if err != nil {
		h.rowMutationError(w, r, err)
		return
	}
	response.Success(w, result)
}

// decodeMutationBody decodes the JSON body with json.Number so BIGINT identity
// and change values keep their exact representation.
func decodeMutationBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		response.ValidationError(w, "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.ValidationError(w, "request body is too large")
			return false
		}
		response.ValidationError(w, "invalid JSON request body")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		response.ValidationError(w, "request body must contain a single JSON object")
		return false
	}
	return true
}

// rowMutationError maps mutation failures to sanitized API errors.
func (h *ConnectionHandler) rowMutationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, database.ErrRowNotFound):
		response.WriteError(w, http.StatusNotFound, "ROW_NOT_FOUND", "the row no longer exists")
	case errors.Is(err, database.ErrRowConflict):
		response.WriteError(w, http.StatusConflict, "ROW_CONFLICT", "the row changed since it was loaded")
	case errors.Is(err, database.ErrRowNotMutable):
		response.WriteError(w, http.StatusBadRequest, "ROW_NOT_MUTABLE", "this object is read-only")
	case errors.Is(err, database.ErrRowIdentityRequired):
		response.WriteError(w, http.StatusBadRequest, "ROW_IDENTITY_REQUIRED", "this table has no primary key; row mutation is disabled")
	case errors.Is(err, database.ErrRowIdentityInvalid):
		response.WriteError(w, http.StatusBadRequest, "ROW_IDENTITY_INVALID", "the row identity is invalid")
	case errors.Is(err, database.ErrColumnReadOnly):
		response.WriteError(w, http.StatusBadRequest, "COLUMN_READ_ONLY", "the column cannot be modified")
	case errors.Is(err, database.ErrAffectedMultipleRows):
		response.WriteError(w, http.StatusInternalServerError, "MUTATION_AFFECTED_MULTIPLE_ROWS", "the mutation matched more than one row and was rolled back")
	case errors.Is(err, database.ErrTableNotFound):
		response.WriteError(w, http.StatusNotFound, "TABLE_NOT_FOUND", "the requested table was not found")
	case errors.Is(err, database.ErrDatabaseNotFound):
		response.WriteError(w, http.StatusBadRequest, "DATABASE_NOT_FOUND", "the requested database does not exist")
	case errors.Is(err, database.ErrDatabaseConnectDenied):
		response.WriteError(w, http.StatusBadRequest, "DATABASE_CONNECT_DENIED", "the connection has no CONNECT permission on that database")
	case errors.Is(err, database.ErrNoBootstrapDatabase):
		response.WriteError(w, http.StatusBadGateway, "BOOTSTRAP_DATABASE_UNAVAILABLE", "no usable database: select one explicitly")
	case errors.Is(err, database.ErrNotImplemented):
		response.WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "row mutation is not implemented for this driver yet")
	case errors.Is(err, context.DeadlineExceeded):
		response.WriteError(w, http.StatusGatewayTimeout, "QUERY_TIMEOUT", "the mutation exceeded the timeout")
	case errors.Is(err, database.ErrConnection):
		response.WriteError(w, http.StatusBadGateway, "CONNECTION_ERROR", "unable to connect to the database")
	default:
		writeInternalError(h.logger, w, r, "row_mutation_failed", err)
	}
}
