package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/security"
	"github.com/datadeck/datadeck/backend/internal/storage"
)

func newQueryHandler(t *testing.T) (*QueryHandler, *sql.DB) {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "datadeck.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cipher, err := security.NewCipher(testKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	manager := database.NewManager(database.Options{
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Minute,
		PingTimeout:     300 * time.Millisecond,
	}, database.Postgres{}, database.MySQL{}, database.SQLite{})
	handler := NewQueryHandler(
		repository.NewConnectionRepository(store.DB()),
		repository.NewQueryHistoryRepository(store.DB()),
		manager,
		cipher,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return handler, store.DB()
}

func TestExecuteValidation(t *testing.T) {
	h, _ := newQueryHandler(t)

	cases := map[string]string{
		"missing connection_id": `{"sql":"SELECT 1"}`,
		"missing sql":           `{"connection_id":"c1"}`,
		"negative timeout":      `{"connection_id":"c1","sql":"SELECT 1","timeout_seconds":-5}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
			env := decodeEnvelope(t, rec)
			if env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
				t.Errorf("error = %+v, want VALIDATION_ERROR", env.Error)
			}
		})
	}
}

func TestExecuteUnknownConnection(t *testing.T) {
	h, _ := newQueryHandler(t)

	rec := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"missing","sql":"SELECT 1"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Errorf("error = %+v, want NOT_FOUND", env.Error)
	}
}

func TestExecuteUnreachableConnectionRecordsErrorHistory(t *testing.T) {
	h, db := newQueryHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "c1",
		Name:         "Unreachable",
		Driver:       model.DriverPostgres,
		Host:         strPtr("127.0.0.1"),
		Port:         intPtr(1),
		DatabaseName: "app",
		Username:     strPtr("appuser"),
	})

	rec := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"c1","sql":"SELECT 1"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "CONNECTION_ERROR" {
		t.Fatalf("error = %+v, want CONNECTION_ERROR", env.Error)
	}
	if string(env.Data) != "null" {
		t.Errorf("data = %s, want null", env.Data)
	}

	historyRec := doRequest(h.History, http.MethodGet, "/api/v1/query/history", "")
	if historyRec.Code != http.StatusOK {
		t.Fatalf("history status = %d, want 200", historyRec.Code)
	}
	var records []HistoryRecord
	if err := json.Unmarshal(decodeEnvelope(t, historyRec).Data, &records); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("history len = %d, want 1", len(records))
	}
	if records[0].Status != string(model.QueryStatusError) {
		t.Errorf("history status = %q, want ERROR", records[0].Status)
	}
	if records[0].SQLText != "SELECT 1" || records[0].ConnectionID != "c1" {
		t.Errorf("history record = %+v", records[0])
	}
	if records[0].ErrorMessage == nil {
		t.Error("history error_message is nil, want a sanitized message")
	}
}

func TestHistoryEmpty(t *testing.T) {
	h, _ := newQueryHandler(t)

	rec := doRequest(h.History, http.MethodGet, "/api/v1/query/history", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	env := decodeEnvelope(t, rec)
	if string(env.Data) != "[]" {
		t.Errorf("data = %s, want []", env.Data)
	}
}

type pagedEnvelope struct {
	Data []map[string]any `json:"data"`
	Meta map[string]any   `json:"meta"`
}

func TestHistoryPagination(t *testing.T) {
	h, db := newQueryHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{ID: "c1", Name: "PG", Driver: model.DriverPostgres, DatabaseName: "app"})
	repo := repository.NewQueryHistoryRepository(db)
	base := time.Now().UTC()
	for i := 0; i < 5; i++ {
		if err := repo.Create(context.Background(), &model.QueryHistory{
			ID: fmt.Sprintf("h%d", i), ConnectionID: "c1", SQLText: "SELECT 1",
			Status: model.QueryStatusSuccess, ExecutedAt: base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("seed history: %v", err)
		}
	}

	rec := doRequest(h.History, http.MethodGet, "/api/v1/query/history?page=2&page_size=2", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body=%s)", rec.Code, rec.Body.String())
	}
	var env pagedEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data) != 2 {
		t.Errorf("page 2 size 2 len = %d, want 2", len(env.Data))
	}
	if env.Meta["total"] != float64(5) || env.Meta["total_pages"] != float64(3) || env.Meta["page"] != float64(2) {
		t.Errorf("meta = %v, want total=5 total_pages=3 page=2", env.Meta)
	}

	bad := doRequest(h.History, http.MethodGet, "/api/v1/query/history?page=0", "")
	if bad.Code != http.StatusBadRequest {
		t.Errorf("invalid page status = %d, want 400", bad.Code)
	}
}
