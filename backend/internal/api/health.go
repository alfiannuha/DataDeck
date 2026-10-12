package api

import (
	"net/http"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/version"
)

// HealthResponse is the payload of the health endpoint. It also exposes
// non-sensitive build metadata so operators can confirm the running version.
type HealthResponse struct {
	Status  string `json:"status" example:"healthy"`
	Version string `json:"version" example:"0.1.0"`
	Channel string `json:"channel" example:"development"`
}

// handleHealth godoc
// @Summary      Health check
// @Description  Reports whether the daemon is running, with the application version and build channel.
// @Tags         system
// @Produce      json
// @Success      200  {object}  response.Envelope{data=api.HealthResponse}
// @Router       /health [get]
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	response.Success(w, HealthResponse{Status: "healthy", Version: version.Version, Channel: version.Channel})
}
