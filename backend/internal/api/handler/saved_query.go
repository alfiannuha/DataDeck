package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
)

// SavedQueryHandler serves the saved-query endpoints.
type SavedQueryHandler struct {
	queries *repository.SavedQueryRepository
	logger  *slog.Logger
}

// NewSavedQueryHandler constructs a handler for saved queries.
func NewSavedQueryHandler(queries *repository.SavedQueryRepository, logger *slog.Logger) *SavedQueryHandler {
	return &SavedQueryHandler{queries: queries, logger: logger}
}

// SavedQueryRequest is the body for creating/updating a saved query. The
// connection is optional (a snippet may be unbound).
type SavedQueryRequest struct {
	ConnectionID *string `json:"connection_id,omitempty" example:"9f1c7d2e4a6b4e89b88ad5f356bf7312"`
	DatabaseName *string `json:"database_name,omitempty" example:"app"`
	Title        string  `json:"title" binding:"required" example:"Active users"`
	SQLText      string  `json:"sql_text" binding:"required" example:"SELECT * FROM users WHERE status = 'active'"`
	Tags         *string `json:"tags,omitempty" example:"users,report"`
}

// SavedQueryResponse is the API representation of a saved query.
type SavedQueryResponse struct {
	ID           string  `json:"id"`
	ConnectionID *string `json:"connection_id"`
	DatabaseName *string `json:"database_name,omitempty"`
	Title        string  `json:"title"`
	SQLText      string  `json:"sql_text"`
	Tags         *string `json:"tags,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func newSavedQueryResponse(query model.SavedQuery) SavedQueryResponse {
	return SavedQueryResponse{
		ID:           query.ID,
		ConnectionID: query.ConnectionID,
		DatabaseName: query.DatabaseName,
		Title:        query.Title,
		SQLText:      query.SQLText,
		Tags:         query.Tags,
		CreatedAt:    query.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    query.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ListSavedQueries godoc
// @Summary      List saved queries
// @Description  Returns saved queries newest-updated first, optionally filtered by connection. (M3 API extension.)
// @Tags         saved-queries
// @Produce      json
// @Param        connection_id  query     string  false  "Filter by connection id"
// @Param        page           query     int     false  "Page number (1-based)"
// @Param        page_size      query     int     false  "Page size (max 200)"
// @Success      200            {object}  response.Envelope{data=[]handler.SavedQueryResponse}
// @Failure      500            {object}  response.ErrorEnvelope
// @Router       /queries/saved [get]
func (h *SavedQueryHandler) List(w http.ResponseWriter, r *http.Request) {
	params, err := parsePageParams(r)
	if err != nil {
		response.ValidationError(w, err.Error())
		return
	}
	connectionID := strings.TrimSpace(r.URL.Query().Get("connection_id"))

	var (
		queries []model.SavedQuery
		total   int
	)
	if connectionID != "" {
		queries, err = h.queries.ListByConnection(r.Context(), connectionID, params.PageSize, params.Offset)
		if err == nil {
			total, err = h.queries.CountByConnection(r.Context(), connectionID)
		}
	} else {
		queries, err = h.queries.List(r.Context(), params.PageSize, params.Offset)
		if err == nil {
			total, err = h.queries.Count(r.Context())
		}
	}
	if err != nil {
		h.internalError(w, r, "list_saved_queries", err)
		return
	}

	out := make([]SavedQueryResponse, 0, len(queries))
	for _, query := range queries {
		out = append(out, newSavedQueryResponse(query))
	}
	response.SuccessWithMeta(w, out, pageMeta(params.Page, params.PageSize, total))
}

// GetSavedQuery godoc
// @Summary      Get a saved query
// @Tags         saved-queries
// @Produce      json
// @Param        id  path      string  true  "Saved query id"
// @Success      200  {object}  response.Envelope{data=handler.SavedQueryResponse}
// @Failure      404  {object}  response.ErrorEnvelope
// @Router       /queries/saved/{id} [get]
func (h *SavedQueryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	query, err := h.queries.Get(r.Context(), id)
	if err != nil {
		h.repositoryError(w, r, "get_saved_query", err)
		return
	}
	response.Success(w, newSavedQueryResponse(*query))
}

// CreateSavedQuery godoc
// @Summary      Create a saved query
// @Description  Persists a SQL snippet. The SQL is stored, never executed. (PRD endpoint.)
// @Tags         saved-queries
// @Accept       json
// @Produce      json
// @Param        request  body      handler.SavedQueryRequest  true  "Saved query"
// @Success      200      {object}  response.Envelope{data=handler.SavedQueryResponse}
// @Failure      400      {object}  response.ErrorEnvelope
// @Failure      404      {object}  response.ErrorEnvelope
// @Router       /queries/saved [post]
func (h *SavedQueryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req SavedQueryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	query, err := normalizeSavedQuery(req)
	if err != nil {
		response.ValidationError(w, err.Error())
		return
	}
	id, err := newID()
	if err != nil {
		h.internalError(w, r, "generate_id", err)
		return
	}
	query.ID = id

	if err := h.queries.Create(r.Context(), query); err != nil {
		h.repositoryError(w, r, "create_saved_query", err)
		return
	}
	response.Success(w, newSavedQueryResponse(*query))
}

// UpdateSavedQuery godoc
// @Summary      Update a saved query
// @Description  Replaces the mutable fields of a saved query. (M3 API extension.)
// @Tags         saved-queries
// @Accept       json
// @Produce      json
// @Param        id       path      string                     true  "Saved query id"
// @Param        request  body      handler.SavedQueryRequest  true  "Saved query"
// @Success      200      {object}  response.Envelope{data=handler.SavedQueryResponse}
// @Failure      400      {object}  response.ErrorEnvelope
// @Failure      404      {object}  response.ErrorEnvelope
// @Router       /queries/saved/{id} [put]
func (h *SavedQueryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	var req SavedQueryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	query, err := normalizeSavedQuery(req)
	if err != nil {
		response.ValidationError(w, err.Error())
		return
	}
	query.ID = id

	if err := h.queries.Update(r.Context(), query); err != nil {
		h.repositoryError(w, r, "update_saved_query", err)
		return
	}
	stored, err := h.queries.Get(r.Context(), id)
	if err != nil {
		h.repositoryError(w, r, "get_saved_query", err)
		return
	}
	response.Success(w, newSavedQueryResponse(*stored))
}

// DeleteSavedQuery godoc
// @Summary      Delete a saved query
// @Tags         saved-queries
// @Produce      json
// @Param        id  path      string  true  "Saved query id"
// @Success      200  {object}  response.Envelope{data=handler.DeleteConnectionResponse}
// @Failure      404  {object}  response.ErrorEnvelope
// @Router       /queries/saved/{id} [delete]
func (h *SavedQueryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if err := h.queries.Delete(r.Context(), id); err != nil {
		h.repositoryError(w, r, "delete_saved_query", err)
		return
	}
	response.Success(w, DeleteConnectionResponse{ID: id})
}

func (h *SavedQueryHandler) repositoryError(w http.ResponseWriter, r *http.Request, action string, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "saved query not found")
		return
	}
	h.internalError(w, r, action, err)
}

func (h *SavedQueryHandler) internalError(w http.ResponseWriter, r *http.Request, action string, err error) {
	writeInternalError(h.logger, w, r, action, err)
}

// normalizeSavedQuery validates input. It never executes or parses SQL beyond
// requiring non-empty content.
func normalizeSavedQuery(req SavedQueryRequest) (*model.SavedQuery, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, errors.New("title is required")
	}
	sqlText := strings.TrimSpace(req.SQLText)
	if sqlText == "" {
		return nil, errors.New("sql_text is required")
	}

	var tags *string
	if req.Tags != nil {
		trimmed := strings.TrimSpace(*req.Tags)
		if trimmed != "" {
			tags = &trimmed
		}
	}

	var connectionID *string
	if req.ConnectionID != nil {
		trimmed := strings.TrimSpace(*req.ConnectionID)
		if trimmed != "" {
			connectionID = &trimmed
		}
	}

	var databaseName *string
	if req.DatabaseName != nil {
		trimmed := strings.TrimSpace(*req.DatabaseName)
		if trimmed != "" {
			databaseName = &trimmed
		}
	}

	return &model.SavedQuery{
		ConnectionID: connectionID,
		DatabaseName: databaseName,
		Title:        title,
		SQLText:      sqlText,
		Tags:         tags,
	}, nil
}
