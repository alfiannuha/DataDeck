package handler

import (
	"net/http"

	"github.com/datadeck/datadeck/backend/internal/api/response"
	"github.com/datadeck/datadeck/backend/internal/database"
)

// PoolStatusResponse is the payload of the connection-manager status endpoint
// (v0.2.0). It reports initialized database pools and how many connection
// profiles are registered. It never contains credentials, DSNs, encryption keys,
// or raw driver errors.
type PoolStatusResponse struct {
	RegisteredConnections int                   `json:"registered_connections"`
	Pools                 []database.PoolStatus `json:"pools"`
}

// PoolStatus godoc
// @Summary      Connection-manager pool status
// @Description  Reports initialized database pools (identity, driver, state, database/sql stats, last-used) and the number of registered connection profiles. Credential-free; does not open pools as a side effect. Registered connections are distinct from initialized pools.
// @Tags         connections
// @Produce      json
// @Success      200  {object} response.Envelope{data=handler.PoolStatusResponse}
// @Failure      500  {object} response.ErrorEnvelope
// @Router       /connections/status [get]
func (h *ConnectionHandler) PoolStatus(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.connections.List(r.Context())
	if err != nil {
		h.repositoryError(w, r, "list_connections", err)
		return
	}
	response.Success(w, PoolStatusResponse{
		RegisteredConnections: len(profiles),
		Pools:                 h.manager.PoolStatuses(),
	})
}
