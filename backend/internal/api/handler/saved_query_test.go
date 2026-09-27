package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/storage"
)

func newSavedQueryHandler(t *testing.T) (*SavedQueryHandler, *sql.DB) {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "datadeck.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	handler := NewSavedQueryHandler(
		repository.NewSavedQueryRepository(store.DB()),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return handler, store.DB()
}

func withSavedQueryID(method, id, body string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/queries/saved/"+id, strings.NewReader(body))
	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func createSavedQuery(t *testing.T, h *SavedQueryHandler, body string) SavedQueryResponse {
	t.Helper()
	rec := doRequest(h.Create, http.MethodPost, "/api/v1/queries/saved", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d (body=%s)", rec.Code, rec.Body.String())
	}
	var query SavedQueryResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &query); err != nil {
		t.Fatalf("decode saved query: %v", err)
	}
	return query
}

func TestSavedQueryCRUD(t *testing.T) {
	h, _ := newSavedQueryHandler(t)

	created := createSavedQuery(t, h,
		`{"title":"Active users","sql_text":"SELECT 1","tags":"users,report"}`)
	if created.ID == "" || created.Title != "Active users" || created.SQLText != "SELECT 1" {
		t.Fatalf("unexpected created query: %+v", created)
	}
	if created.ConnectionID != nil {
		t.Errorf("ConnectionID = %v, want nil (unbound)", created.ConnectionID)
	}
	if created.Tags == nil || *created.Tags != "users,report" {
		t.Errorf("Tags = %v", created.Tags)
	}

	getRec := httptest.NewRecorder()
	h.Get(getRec, withSavedQueryID(http.MethodGet, created.ID, ""))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d", getRec.Code)
	}

	listRec := doRequest(h.List, http.MethodGet, "/api/v1/queries/saved", "")
	var listed []SavedQueryResponse
	if err := json.Unmarshal(decodeEnvelope(t, listRec).Data, &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list len = %d, want 1", len(listed))
	}

	updateRec := httptest.NewRecorder()
	h.Update(updateRec, withSavedQueryID(http.MethodPut, created.ID,
		`{"title":"Active users v2","sql_text":"SELECT 2"}`))
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d (body=%s)", updateRec.Code, updateRec.Body.String())
	}
	var updated SavedQueryResponse
	if err := json.Unmarshal(decodeEnvelope(t, updateRec).Data, &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.Title != "Active users v2" || updated.SQLText != "SELECT 2" {
		t.Errorf("updated = %+v", updated)
	}
	if updated.Tags != nil {
		t.Errorf("Tags = %v, want nil after update", updated.Tags)
	}

	deleteRec := httptest.NewRecorder()
	h.Delete(deleteRec, withSavedQueryID(http.MethodDelete, created.ID, ""))
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d", deleteRec.Code)
	}
	missingRec := httptest.NewRecorder()
	h.Get(missingRec, withSavedQueryID(http.MethodGet, created.ID, ""))
	if missingRec.Code != http.StatusNotFound {
		t.Errorf("get after delete status = %d, want 404", missingRec.Code)
	}
}

func TestSavedQueryValidation(t *testing.T) {
	h, _ := newSavedQueryHandler(t)

	cases := map[string]string{
		"missing title": `{"sql_text":"SELECT 1"}`,
		"missing sql":   `{"title":"x"}`,
		"blank title":   `{"title":"   ","sql_text":"SELECT 1"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := doRequest(h.Create, http.MethodPost, "/api/v1/queries/saved", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
			if env := decodeEnvelope(t, rec); env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
				t.Errorf("error = %+v, want VALIDATION_ERROR", env.Error)
			}
		})
	}
}

func TestSavedQueryConnectionRelationship(t *testing.T) {
	h, db := newSavedQueryHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "c1", Name: "PG", Driver: model.DriverPostgres, DatabaseName: "app",
	})

	created := createSavedQuery(t, h,
		`{"connection_id":"c1","title":"Bound","sql_text":"SELECT 1"}`)
	if created.ConnectionID == nil || *created.ConnectionID != "c1" {
		t.Fatalf("ConnectionID = %v, want c1", created.ConnectionID)
	}

	// Filter by connection.
	filterRec := doRequest(h.List, http.MethodGet, "/api/v1/queries/saved?connection_id=c1", "")
	var filtered []SavedQueryResponse
	if err := json.Unmarshal(decodeEnvelope(t, filterRec).Data, &filtered); err != nil {
		t.Fatalf("decode filtered: %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("filtered len = %d, want 1", len(filtered))
	}

	// Deleting the connection nulls the saved-query reference (ON DELETE SET NULL).
	if err := repository.NewConnectionRepository(db).Delete(context.Background(), "c1"); err != nil {
		t.Fatalf("delete connection: %v", err)
	}
	getRec := httptest.NewRecorder()
	h.Get(getRec, withSavedQueryID(http.MethodGet, created.ID, ""))
	var afterDelete SavedQueryResponse
	if err := json.Unmarshal(decodeEnvelope(t, getRec).Data, &afterDelete); err != nil {
		t.Fatalf("decode after delete: %v", err)
	}
	if afterDelete.ConnectionID != nil {
		t.Errorf("ConnectionID = %v, want nil after connection delete", afterDelete.ConnectionID)
	}
}

func TestSavedQueryUnknownConnection(t *testing.T) {
	h, _ := newSavedQueryHandler(t)
	rec := doRequest(h.Create, http.MethodPost, "/api/v1/queries/saved",
		`{"connection_id":"missing","title":"x","sql_text":"SELECT 1"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestSavedQueryNotFound(t *testing.T) {
	h, _ := newSavedQueryHandler(t)

	getRec := httptest.NewRecorder()
	h.Get(getRec, withSavedQueryID(http.MethodGet, "missing", ""))
	if getRec.Code != http.StatusNotFound {
		t.Errorf("get status = %d, want 404", getRec.Code)
	}

	updateRec := httptest.NewRecorder()
	h.Update(updateRec, withSavedQueryID(http.MethodPut, "missing",
		`{"title":"x","sql_text":"SELECT 1"}`))
	if updateRec.Code != http.StatusNotFound {
		t.Errorf("update status = %d, want 404", updateRec.Code)
	}

	deleteRec := httptest.NewRecorder()
	h.Delete(deleteRec, withSavedQueryID(http.MethodDelete, "missing", ""))
	if deleteRec.Code != http.StatusNotFound {
		t.Errorf("delete status = %d, want 404", deleteRec.Code)
	}
}

type savedPagedEnvelope struct {
	Data []SavedQueryResponse `json:"data"`
	Meta map[string]any       `json:"meta"`
}

func TestSavedQueryPagination(t *testing.T) {
	h, _ := newSavedQueryHandler(t)
	for i := 0; i < 5; i++ {
		createSavedQuery(t, h, fmt.Sprintf(`{"title":"q%d","sql_text":"SELECT %d"}`, i, i))
	}

	rec := doRequest(h.List, http.MethodGet, "/api/v1/queries/saved?page=2&page_size=2", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body=%s)", rec.Code, rec.Body.String())
	}
	var env savedPagedEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data) != 2 {
		t.Errorf("page 2 size 2 len = %d, want 2", len(env.Data))
	}
	if env.Meta["total"] != float64(5) || env.Meta["total_pages"] != float64(3) {
		t.Errorf("meta = %v, want total=5 total_pages=3", env.Meta)
	}

	bad := doRequest(h.List, http.MethodGet, "/api/v1/queries/saved?page_size=0", "")
	if bad.Code != http.StatusBadRequest {
		t.Errorf("invalid page_size status = %d, want 400", bad.Code)
	}
}
