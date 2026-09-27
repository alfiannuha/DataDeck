package api

import (
	"net/http"

	"github.com/datadeck/datadeck/backend/internal/api/response"
)

// HealthResponse is the payload of the health endpoint.
type HealthResponse struct {
	Status string `json:"status" example:"healthy"`
}

// handleHealth godoc
// @Summary      Health check
// @Description  Reports whether the daemon is running.
// @Tags         system
// @Produce      json
// @Success      200  {object}  response.Envelope{data=api.HealthResponse}
// @Router       /health [get]
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	response.Success(w, HealthResponse{Status: "healthy"})
}
