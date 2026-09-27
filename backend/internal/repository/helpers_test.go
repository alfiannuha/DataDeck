package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
	"github.com/datadeck/datadeck/backend/internal/storage"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "datadeck.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store.DB()
}

func ptr[T any](v T) *T { return &v }

func seedConnection(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if err := NewConnectionRepository(db).Create(context.Background(), &model.ConnectionProfile{
		ID:           id,
		Name:         id,
		Driver:       model.DriverPostgres,
		DatabaseName: "app",
	}); err != nil {
		t.Fatalf("seed connection %q: %v", id, err)
	}
}
