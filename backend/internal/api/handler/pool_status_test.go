package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestPoolStatusEndpoint(t *testing.T) {
	h, store, _ := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "p.db")
	seedProfile(t, store, &model.ConnectionProfile{
		ID: "c1", Name: "SQLite", Driver: model.DriverSQLite, DatabaseName: path,
	})

	// No pools initialized yet: registered connection is reported, pools empty.
	rec := httptest.NewRecorder()
	h.PoolStatus(rec, httptest.NewRequest(http.MethodGet, "/api/v1/connections/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp PoolStatusResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.RegisteredConnections != 1 {
		t.Errorf("registered_connections = %d, want 1", resp.RegisteredConnections)
	}
	if len(resp.Pools) != 0 {
		t.Errorf("pools = %+v, want empty (status must not open pools)", resp.Pools)
	}

	// Initialize a pool; status reports it with driver/state and no secrets.
	if _, err := h.manager.Open(context.Background(), "c1", database.Config{Driver: model.DriverSQLite, Database: path}); err != nil {
		t.Fatalf("open pool: %v", err)
	}
	rec = httptest.NewRecorder()
	h.PoolStatus(rec, httptest.NewRequest(http.MethodGet, "/api/v1/connections/status", nil))
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Pools) != 1 {
		t.Fatalf("pools = %+v, want 1", resp.Pools)
	}
	p := resp.Pools[0]
	if p.ConnectionID != "c1" || p.Database != path || p.Driver != "sqlite" || p.State != "ready" {
		t.Errorf("unexpected pool status: %+v", p)
	}
	// Never leak credentials/keys/DSNs.
	for _, forbidden := range []string{"password", "encryption_key", "ENCRYPTION_KEY", "postgres://", "mysql://"} {
		if strings.Contains(rec.Body.String(), forbidden) {
			t.Errorf("status body contains forbidden token %q", forbidden)
		}
	}
}
