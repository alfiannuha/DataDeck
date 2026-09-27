package handler

import (
	"log/slog"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/datadeck/datadeck/backend/internal/api/response"
)

// writeInternalError logs the underlying error server-side (with the request
// id) and returns a sanitized internal-error envelope to the client.
func writeInternalError(logger *slog.Logger, w http.ResponseWriter, r *http.Request, action string, err error) {
	logger.Error(action,
		slog.String("request_id", chimiddleware.GetReqID(r.Context())),
		slog.String("error", err.Error()),
	)
	response.InternalError(w, "internal server error")
}
