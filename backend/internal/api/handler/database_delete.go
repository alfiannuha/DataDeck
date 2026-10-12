package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
)

const (
	// deleteTokenTTL bounds how long a delete-preflight token stays valid.
	deleteTokenTTL = 5 * time.Minute
	// databaseDeleteTimeoutSeconds bounds one preflight/execute request.
	databaseDeleteTimeoutSeconds = 60
)

// deleteToken is a short-lived, single-use confirmation bound to one exact
// target. It is never sufficient on its own — the backend re-validates the
// target, protection policy, existence and active operations before DROP.
type deleteToken struct {
	connectionID string
	database     string
	engine       string
	createdAt    time.Time
}

// DatabaseDeletePreflight is returned by the preflight endpoint.
type DatabaseDeletePreflight struct {
	Token        string    `json:"token"`
	ConnectionID string    `json:"connection_id"`
	Database     string    `json:"database"`
	Engine       string    `json:"engine"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// DatabaseDeleteRequest is the execute body. The frontend booleans are intent
// signals only; the backend still validates the token, target and database state.
type DatabaseDeleteRequest struct {
	ConfirmationToken string `json:"confirmation_token"`
	TypedDatabaseName string `json:"typed_database_name"`
	Acknowledged      bool   `json:"acknowledged"`
}

// DatabaseDeleteResult is returned by the execute endpoint after verification.
type DatabaseDeleteResult struct {
	Status   string `json:"status" example:"deleted"`
	Database string `json:"database"`
}

// DatabaseDeletePreflight godoc
// @Summary      Preflight a database deletion
// @Description  Validates a delete-database request and returns a short-lived, single-use confirmation token bound to the exact target. Never runs DROP. Only PostgreSQL and MySQL are supported; protected system databases and targets with active operations are rejected.
// @Tags         database
// @Produce      json
// @Param        id        path     string  true  "Connection id"
// @Param        database  path     string  true  "Target database"
// @Success      200  {object} response.Envelope{data=handler.DatabaseDeletePreflight}
// @Failure      400  {object} response.ErrorEnvelope
// @Failure      404  {object} response.ErrorEnvelope
// @Failure      409  {object} response.ErrorEnvelope
// @Failure      501  {object} response.ErrorEnvelope
// @Router       /connections/{id}/databases/{database}/delete-preflight [post]
func (h *ConnectionHandler) DatabaseDeletePreflight(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	target := pathUnescape(chi.URLParam(r, "database"))
	if id == "" || target == "" {
		response.ValidationError(w, "connection id and database are required")
		return
	}

	profile, password, effectiveDatabase, ok := h.resolveDeleteTarget(w, r, id, target)
	if !ok {
		return
	}
	if effectiveDatabase == "" {
		response.ValidationError(w, "database is required")
		return
	}
	if database.IsProtectedDatabase(profile.Driver, effectiveDatabase) {
		response.WriteError(w, http.StatusBadRequest, "DATABASE_DELETE_PROTECTED",
			"this database is protected and cannot be deleted")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), databaseDeleteTimeoutSeconds*time.Second)
	defer cancel()

	if reason, err := h.deleteBlockingCondition(ctx, id, profile, password, effectiveDatabase); err != nil {
		h.databaseDeleteError(w, r, err)
		return
	} else if reason != "" {
		response.WriteError(w, http.StatusConflict, "DATABASE_IN_USE", reason)
		return
	}

	execCfg, ok := h.deleteExecutionConfig(profile, password, effectiveDatabase)
	if !ok {
		response.WriteError(w, http.StatusBadGateway, "DATABASE_DELETE_PRECHECK_FAILED",
			"no maintenance connection is available")
		return
	}
	exists, err := h.manager.DatabaseExists(ctx, id, execCfg, effectiveDatabase)
	if err != nil {
		h.databaseDeleteError(w, r, err)
		return
	}
	if !exists {
		response.WriteError(w, http.StatusNotFound, "DATABASE_NOT_FOUND", "the database does not exist")
		return
	}

	token, err := newRestoreToken()
	if err != nil {
		h.internalError(w, r, "generate_delete_token", err)
		return
	}
	h.sweepDeleteTokens()
	h.deleteMu.Lock()
	h.deleteTokens[token] = &deleteToken{
		connectionID: id,
		database:     effectiveDatabase,
		engine:       string(profile.Driver),
		createdAt:    time.Now(),
	}
	h.deleteMu.Unlock()

	response.Success(w, DatabaseDeletePreflight{
		Token:        token,
		ConnectionID: id,
		Database:     effectiveDatabase,
		Engine:       string(profile.Driver),
		ExpiresAt:    time.Now().Add(deleteTokenTTL),
	})
}

// DatabaseDelete godoc
// @Summary      Delete a database
// @Description  Executes DROP DATABASE for the exact preflighted target after re-validating the token, target binding, protection policy and active operations. Never force-terminates sessions, never uses CASCADE/FORCE, never retries automatically, and reports success only after independently verifying the database is gone.
// @Tags         database
// @Accept       json
// @Produce      json
// @Param        id        path     string                      true  "Connection id"
// @Param        database  path     string                      true  "Target database"
// @Param        request   body     handler.DatabaseDeleteRequest true  "Confirmation"
// @Success      200  {object} response.Envelope{data=handler.DatabaseDeleteResult}
// @Failure      400  {object} response.ErrorEnvelope
// @Failure      404  {object} response.ErrorEnvelope
// @Failure      409  {object} response.ErrorEnvelope
// @Failure      501  {object} response.ErrorEnvelope
// @Failure      502  {object} response.ErrorEnvelope
// @Router       /connections/{id}/databases/{database}/delete [post]
func (h *ConnectionHandler) DatabaseDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	target := pathUnescape(chi.URLParam(r, "database"))
	if id == "" || target == "" {
		response.ValidationError(w, "connection id and database are required")
		return
	}

	var body DatabaseDeleteRequest
	if !decodeJSON(w, r, &body) {
		return
	}

	// Consume the token first: it is single-use, and a consumed token is never
	// retried automatically.
	h.deleteMu.Lock()
	token, ok := h.deleteTokens[body.ConfirmationToken]
	if ok {
		delete(h.deleteTokens, body.ConfirmationToken)
	}
	h.deleteMu.Unlock()
	if !ok || token == nil || time.Since(token.createdAt) > deleteTokenTTL {
		code := "DATABASE_DELETE_CONFIRMATION_INVALID"
		if ok {
			code = "DATABASE_DELETE_TOKEN_EXPIRED"
		}
		response.WriteError(w, http.StatusBadRequest, code, "the confirmation is invalid or expired")
		return
	}

	profile, password, effectiveDatabase, ok := h.resolveDeleteTarget(w, r, id, target)
	if !ok {
		return
	}
	// Bind the token to the exact target: connection, database and engine must
	// all match, or the token is not usable.
	if token.connectionID != id || token.database != effectiveDatabase || token.engine != string(profile.Driver) {
		response.WriteError(w, http.StatusBadRequest, "DATABASE_DELETE_CONFIRMATION_INVALID",
			"the confirmation does not match the requested target")
		return
	}
	// Typed name is a case-sensitive exact match; the boolean is intent only.
	if strings.TrimSpace(body.TypedDatabaseName) != effectiveDatabase || !body.Acknowledged {
		response.WriteError(w, http.StatusBadRequest, "DATABASE_DELETE_CONFIRMATION_INVALID",
			"the typed database name or acknowledgment is missing")
		return
	}
	if database.IsProtectedDatabase(profile.Driver, effectiveDatabase) {
		response.WriteError(w, http.StatusBadRequest, "DATABASE_DELETE_PROTECTED",
			"this database is protected and cannot be deleted")
		return
	}

	// Serialize per target: a second concurrent delete is refused.
	inFlightKey := id + "/" + effectiveDatabase
	h.deleteMu.Lock()
	if h.deleteInFlight[inFlightKey] {
		h.deleteMu.Unlock()
		response.WriteError(w, http.StatusConflict, "DATABASE_DELETE_CONFLICT", "a delete is already in progress for this database")
		return
	}
	h.deleteInFlight[inFlightKey] = true
	h.deleteMu.Unlock()
	defer func() {
		h.deleteMu.Lock()
		delete(h.deleteInFlight, inFlightKey)
		h.deleteMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(r.Context(), databaseDeleteTimeoutSeconds*time.Second)
	defer cancel()

	if reason, err := h.deleteBlockingCondition(ctx, id, profile, password, effectiveDatabase); err != nil {
		h.databaseDeleteError(w, r, err)
		return
	} else if reason != "" {
		response.WriteError(w, http.StatusConflict, "DATABASE_IN_USE", reason)
		return
	}

	execCfg, ok := h.deleteExecutionConfig(profile, password, effectiveDatabase)
	if !ok {
		response.WriteError(w, http.StatusBadGateway, "DATABASE_DELETE_PRECHECK_FAILED",
			"no maintenance connection is available")
		return
	}
	exists, err := h.manager.DatabaseExists(ctx, id, execCfg, effectiveDatabase)
	if err != nil {
		h.databaseDeleteError(w, r, err)
		return
	}
	if !exists {
		response.WriteError(w, http.StatusNotFound, "DATABASE_NOT_FOUND", "the database does not exist")
		return
	}

	// Retire the target pool before DROP (PostgreSQL refuses to drop a database
	// with active connections; other databases' pools are untouched).
	_ = h.manager.Close(id, effectiveDatabase)

	audit := []any{
		"connection_id", id,
		"engine", string(profile.Driver),
		"database", effectiveDatabase,
		"operation", "delete_database",
	}
	start := time.Now()
	if err := h.manager.DropDatabase(ctx, id, execCfg, effectiveDatabase); err != nil {
		h.logger.Warn("database_delete_failed",
			append(audit, "error", err.Error(), "duration_ms", time.Since(start).Milliseconds())...)
		h.databaseDeleteError(w, r, err)
		return
	}

	// Success only after independent verification.
	stillExists, err := h.manager.DatabaseExists(ctx, id, execCfg, effectiveDatabase)
	if err != nil {
		h.logger.Error("database_delete_outcome_unknown",
			append(audit, "error", err.Error())...)
		response.WriteError(w, http.StatusBadGateway, "DATABASE_DELETE_OUTCOME_UNKNOWN",
			"the database deletion outcome cannot be verified; check the server before retrying")
		return
	}
	if stillExists {
		h.logger.Warn("database_delete_conflict", audit...)
		response.WriteError(w, http.StatusConflict, "DATABASE_DELETE_CONFLICT",
			"the database still exists after the delete; no automatic retry is performed")
		return
	}

	h.logger.Info("database_deleted",
		append(audit,
			"outcome", "deleted",
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", chimiddleware.GetReqID(r.Context()))...)

	response.Success(w, DatabaseDeleteResult{Status: "deleted", Database: effectiveDatabase})
}

// resolveDeleteTarget fetches the profile and resolves the target database,
// rejecting engines that do not support database deletion before any database
// resolution. It writes the error and returns ok=false on failure.
func (h *ConnectionHandler) resolveDeleteTarget(w http.ResponseWriter, r *http.Request, id, databaseName string) (*model.ConnectionProfile, string, string, bool) {
	profile, err := h.connections.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "connection profile not found")
			return nil, "", "", false
		}
		h.internalError(w, r, "get_connection", err)
		return nil, "", "", false
	}
	if !database.DropSupported(profile.Driver) {
		response.WriteError(w, http.StatusNotImplemented, "DATABASE_DELETE_UNSUPPORTED",
			"delete database is not supported for this engine")
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

// deleteBlockingCondition returns a human reason when deletion must be blocked,
// or an error for infrastructure failures. It never force-terminates anything.
func (h *ConnectionHandler) deleteBlockingCondition(ctx context.Context, id string, profile *model.ConnectionProfile, password, target string) (string, error) {
	// A DataDeck-managed pool with in-use connections means an operation is
	// running against the target. Block; never cancel it.
	if db, ok := h.manager.Get(id, target); ok {
		if stats := db.Stats(); stats.InUse > 0 {
			return fmt.Sprintf("%d active operation(s) are using this database", stats.InUse), nil
		}
	}
	return "", nil
}

// deleteExecutionConfig builds the connection used to run DROP DATABASE. For
// PostgreSQL it must NOT be the target database (the server forbids dropping the
// current database); a maintenance database is used. MySQL permits dropping the
// current database, so its profile database is used.
func (h *ConnectionHandler) deleteExecutionConfig(profile *model.ConnectionProfile, password, target string) (database.Config, bool) {
	cfg := toDatabaseConfig(profile, password)
	if profile.Driver == model.DriverPostgres {
		maintenance := strings.TrimSpace(profile.DatabaseName)
		if maintenance == "" || maintenance == target {
			maintenance = "postgres"
		}
		if maintenance == target {
			return database.Config{}, false
		}
		cfg.Database = maintenance
		return cfg, true
	}
	cfg.Database = profile.DatabaseName
	return cfg, strings.TrimSpace(cfg.Database) != ""
}

// databaseDeleteError maps delete failures to the contract taxonomy (sanitized).
func (h *ConnectionHandler) databaseDeleteError(w http.ResponseWriter, r *http.Request, err error) {
	var sqlErr *database.SQLError
	switch {
	case errors.Is(err, database.ErrProtectedDatabase):
		response.WriteError(w, http.StatusBadRequest, "DATABASE_DELETE_PROTECTED", "this database is protected")
	case errors.Is(err, database.ErrDatabaseInUse):
		response.WriteError(w, http.StatusConflict, "DATABASE_IN_USE", "the database is in use")
	case errors.Is(err, database.ErrDatabaseNotFound):
		response.WriteError(w, http.StatusNotFound, "DATABASE_NOT_FOUND", "the database does not exist")
	case errors.Is(err, database.ErrNotImplemented):
		response.WriteError(w, http.StatusNotImplemented, "DATABASE_DELETE_UNSUPPORTED", "delete database is not supported for this engine")
	case errors.Is(err, database.ErrDatabaseDropFailed):
		message := "the database could not be deleted"
		if errors.As(err, &sqlErr) {
			message = sqlErr.Message
		}
		response.WriteError(w, http.StatusBadRequest, "DATABASE_DELETE_FAILED", message)
	case errors.Is(err, context.DeadlineExceeded):
		response.WriteError(w, http.StatusGatewayTimeout, "QUERY_TIMEOUT", "the operation exceeded its timeout")
	case errors.Is(err, context.Canceled):
		response.WriteError(w, 499, "QUERY_CANCELED", "the operation was canceled")
	case errors.Is(err, database.ErrConnection):
		response.WriteError(w, http.StatusBadGateway, "DATABASE_DELETE_PRECHECK_FAILED", "unable to reach the database server")
	default:
		h.logger.Error("database_delete_error",
			slog.String("request_id", chimiddleware.GetReqID(r.Context())),
			slog.String("error", err.Error()),
		)
		response.WriteError(w, http.StatusInternalServerError, "DATABASE_DELETE_FAILED", "the delete request failed")
	}
}

func (h *ConnectionHandler) sweepDeleteTokens() {
	h.deleteMu.Lock()
	for token, entry := range h.deleteTokens {
		if time.Since(entry.createdAt) > deleteTokenTTL {
			delete(h.deleteTokens, token)
		}
	}
	h.deleteMu.Unlock()
}

func pathUnescape(value string) string {
	if decoded, err := url.PathUnescape(value); err == nil {
		return decoded
	}
	return value
}
