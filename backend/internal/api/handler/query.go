package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/security"
)

const (
	defaultQueryTimeoutSeconds = 30  // PRD §11.3
	maxQueryTimeoutSeconds     = 300 // server-side ceiling
	historyWriteTimeout        = 5 * time.Second
)

// QueryHandler serves query execution and history endpoints.
type QueryHandler struct {
	connections *repository.ConnectionRepository
	history     *repository.QueryHistoryRepository
	manager     *database.Manager
	cipher      *security.Cipher
	logger      *slog.Logger
}

// NewQueryHandler constructs a handler for query endpoints.
func NewQueryHandler(
	connections *repository.ConnectionRepository,
	history *repository.QueryHistoryRepository,
	manager *database.Manager,
	cipher *security.Cipher,
	logger *slog.Logger,
) *QueryHandler {
	return &QueryHandler{
		connections: connections,
		history:     history,
		manager:     manager,
		cipher:      cipher,
		logger:      logger,
	}
}

// QueryRequest is the body for the query execution endpoint.
//
// PRF-01: `database` optionally selects the target database on a server-level
// PostgreSQL connection. It must not be used to redirect an existing tab: the
// client sends the tab's bound database. MySQL/SQLite ignore it unless it
// equals the profile's database; a mismatch is a validation error.
type QueryRequest struct {
	ConnectionID   string `json:"connection_id" binding:"required" example:"9f1c7d2e4a6b4e89b88ad5f356bf7312"`
	SQL            string `json:"sql" binding:"required" example:"SELECT 1"`
	Database       string `json:"database" example:"app"`
	TimeoutSeconds int    `json:"timeout_seconds" example:"30"`
}

// HistoryRecord is one persisted query execution audit entry.
type HistoryRecord struct {
	ID              string    `json:"id"`
	ConnectionID    string    `json:"connection_id"`
	DatabaseName    *string   `json:"database_name,omitempty"`
	SQLText         string    `json:"sql_text"`
	Status          string    `json:"status" enums:"SUCCESS,ERROR"`
	ExecutionTimeMS int64     `json:"execution_time_ms"`
	RowsAffected    int64     `json:"rows_affected"`
	ErrorMessage    *string   `json:"error_message,omitempty"`
	ExecutedAt      time.Time `json:"executed_at"`
}

// Execute godoc
// @Summary      Execute a SQL statement
// @Description  Runs a single SQL statement against a stored connection under a bounded timeout and records the attempt in history.
// @Tags         query
// @Accept       json
// @Produce      json
// @Param        request  body      handler.QueryRequest  true  "Query"
// @Success      200      {object}  response.Envelope{data=model.QueryResult}
// @Failure      400      {object}  response.ErrorEnvelope
// @Failure      404      {object}  response.ErrorEnvelope
// @Failure      499      {object}  response.ErrorEnvelope
// @Failure      502      {object}  response.ErrorEnvelope
// @Failure      504      {object}  response.ErrorEnvelope
// @Router       /query/execute [post]
func (h *QueryHandler) Execute(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	connectionID := strings.TrimSpace(req.ConnectionID)
	if connectionID == "" {
		response.ValidationError(w, "connection_id is required")
		return
	}
	if strings.TrimSpace(req.SQL) == "" {
		response.ValidationError(w, "sql is required")
		return
	}
	timeout := req.TimeoutSeconds
	if timeout < 0 {
		response.ValidationError(w, "timeout_seconds must not be negative")
		return
	}
	if timeout == 0 {
		timeout = defaultQueryTimeoutSeconds
	}
	if timeout > maxQueryTimeoutSeconds {
		timeout = maxQueryTimeoutSeconds
	}

	profile, err := h.connections.Get(r.Context(), connectionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "connection profile not found")
			return
		}
		writeInternalError(h.logger, w, r, "get_connection", err)
		return
	}

	password := ""
	if profile.EncryptedPassword != nil {
		plaintext, err := h.cipher.Decrypt(*profile.EncryptedPassword)
		if err != nil {
			writeInternalError(h.logger, w, r, "decrypt_password", err)
			return
		}
		password = string(plaintext)
	}

	effectiveDatabase, err := resolveTargetDatabase(profile, req.Database)
	if err != nil {
		writeResolveDatabaseError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeout)*time.Second)
	defer cancel()

	cfg := toDatabaseConfig(profile, password)
	cfg.Database = effectiveDatabase
	result, execErr := h.manager.Execute(ctx, connectionID, cfg, req.SQL)
	h.recordHistory(r, connectionID, effectiveDatabase, req.SQL, result, execErr)

	if execErr != nil {
		h.executionError(w, r, execErr)
		return
	}
	response.Success(w, result)
}

// History godoc
// @Summary      List query history
// @Description  Returns recent executions newest-first, optionally filtered by connection.
// @Tags         query
// @Produce      json
// @Param        connection_id  query     string  false  "Filter by connection id"
// @Param        page           query     int     false  "Page number (1-based)"
// @Param        page_size      query     int     false  "Page size (max 200)"
// @Success      200            {object}  response.Envelope{data=[]handler.HistoryRecord}
// @Failure      500            {object}  response.ErrorEnvelope
// @Router       /query/history [get]
func (h *QueryHandler) History(w http.ResponseWriter, r *http.Request) {
	params, err := parsePageParams(r)
	if err != nil {
		response.ValidationError(w, err.Error())
		return
	}

	filter := repository.HistoryFilter{
		ConnectionID: strings.TrimSpace(r.URL.Query().Get("connection_id")),
		Limit:        params.PageSize,
		Offset:       params.Offset,
	}
	records, err := h.history.List(r.Context(), filter)
	if err != nil {
		writeInternalError(h.logger, w, r, "list_history", err)
		return
	}
	total, err := h.history.Count(r.Context(), repository.HistoryFilter{ConnectionID: filter.ConnectionID})
	if err != nil {
		writeInternalError(h.logger, w, r, "count_history", err)
		return
	}

	out := make([]HistoryRecord, 0, len(records))
	for _, record := range records {
		out = append(out, HistoryRecord{
			ID:              record.ID,
			ConnectionID:    record.ConnectionID,
			DatabaseName:    record.DatabaseName,
			SQLText:         record.SQLText,
			Status:          string(record.Status),
			ExecutionTimeMS: record.ExecutionTimeMS,
			RowsAffected:    record.RowsAffected,
			ErrorMessage:    record.ErrorMessage,
			ExecutedAt:      record.ExecutedAt,
		})
	}
	response.SuccessWithMeta(w, out, pageMeta(params.Page, params.PageSize, total))
}

// recordHistory persists the audit record. A history write failure is logged
// but never changes the query response the client receives.
func (h *QueryHandler) recordHistory(r *http.Request, connectionID, databaseName, sqlText string, result model.QueryResult, execErr error) {
	status := model.QueryStatusSuccess
	var errorMessage *string
	if execErr != nil {
		status = model.QueryStatusError
		message := sanitizeExecutionError(execErr)
		errorMessage = &message
	}

	id, err := newID()
	if err != nil {
		h.logger.Error("query_history_id_failed",
			"request_id", chimiddleware.GetReqID(r.Context()),
			"error", err.Error(),
		)
		return
	}

	// The request context may already be expired (timeouts); history writes use
	// an independent bounded context so the audit record is still persisted.
	ctx, cancel := context.WithTimeout(context.Background(), historyWriteTimeout)
	defer cancel()

	var database *string
	if databaseName != "" {
		database = &databaseName
	}
	record := &model.QueryHistory{
		ID:              id,
		ConnectionID:    connectionID,
		DatabaseName:    database,
		SQLText:         sqlText,
		Status:          status,
		ExecutionTimeMS: result.ExecutionTimeMS,
		RowsAffected:    result.RowsAffected,
		ErrorMessage:    errorMessage,
	}
	if err := h.history.Create(ctx, record); err != nil {
		h.logger.Error("query_history_write_failed",
			"request_id", chimiddleware.GetReqID(r.Context()),
			"error", err.Error(),
		)
	}
}

func (h *QueryHandler) executionError(w http.ResponseWriter, r *http.Request, err error) {
	var sqlErr *database.SQLError
	switch {
	case errors.As(err, &sqlErr):
		code := "SQL_ERROR"
		if sqlErr.Syntax {
			code = "SQL_SYNTAX_ERROR"
		}
		apiErr := response.APIError{Code: code, Message: sqlErr.Message}
		if sqlErr.Position > 0 {
			position := sqlErr.Position
			apiErr.Position = &position
		}
		response.WriteErrorObject(w, http.StatusBadRequest, apiErr)
	case errors.Is(err, context.DeadlineExceeded):
		response.WriteError(w, http.StatusGatewayTimeout, "QUERY_TIMEOUT", "query exceeded the timeout")
	case errors.Is(err, context.Canceled):
		response.WriteError(w, 499, "QUERY_CANCELED", "query was canceled")
	case errors.Is(err, database.ErrDatabaseNotFound):
		response.WriteError(w, http.StatusBadRequest, "DATABASE_NOT_FOUND", "the requested database does not exist")
	case errors.Is(err, database.ErrDatabaseConnectDenied):
		response.WriteError(w, http.StatusBadRequest, "DATABASE_CONNECT_DENIED", "the connection has no CONNECT permission on that database")
	case errors.Is(err, database.ErrNoBootstrapDatabase):
		response.WriteError(w, http.StatusBadGateway, "BOOTSTRAP_DATABASE_UNAVAILABLE", "no usable database: select one explicitly")
	case errors.Is(err, database.ErrUnsupportedDriver):
		response.ValidationError(w, "the connection driver is not supported yet")
	case errors.Is(err, database.ErrNotImplemented):
		response.WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
			"query execution is not implemented for this driver yet")
	case errors.Is(err, database.ErrConnection):
		response.WriteError(w, http.StatusBadGateway, "CONNECTION_ERROR", "unable to connect to the database")
	default:
		writeInternalError(h.logger, w, r, "query_execution_failed", err)
	}
}

// sanitizeExecutionError produces a history-safe message: SQL errors keep their
// message and position, everything else becomes generic (never a DSN).
func sanitizeExecutionError(err error) string {
	var sqlErr *database.SQLError
	switch {
	case errors.As(err, &sqlErr):
		if sqlErr.Position > 0 {
			return sqlErr.Message + " (position " + strconv.Itoa(sqlErr.Position) + ")"
		}
		return sqlErr.Message
	case errors.Is(err, context.DeadlineExceeded):
		return "query timeout"
	case errors.Is(err, context.Canceled):
		return "query canceled"
	default:
		return "query execution failed"
	}
}
