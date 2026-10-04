package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestSavedQueryRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewSavedQueryRepository(newTestDB(t))

	created := &model.SavedQuery{
		ID:      "s1",
		Title:   "Active users",
		SQLText: "SELECT * FROM users WHERE status = 'active'",
		Tags:    ptr("users,report"),
	}
	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("Create() did not assign timestamps")
	}

	got, err := repo.Get(ctx, "s1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Title != "Active users" || got.ConnectionID != nil {
		t.Errorf("Get() = %+v", got)
	}
	if got.Tags == nil || *got.Tags != "users,report" {
		t.Errorf("Tags = %v", got.Tags)
	}

	got.Title = "Active users v2"
	got.Tags = nil
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := repo.Get(ctx, "s1")
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if updated.Title != "Active users v2" {
		t.Errorf("Title = %q", updated.Title)
	}
	if updated.Tags != nil {
		t.Errorf("Tags = %v, want nil", updated.Tags)
	}

	list, err := repo.List(ctx, 0, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].ID != "s1" {
		t.Errorf("List() = %+v", list)
	}

	if err := repo.Delete(ctx, "s1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Get(ctx, "s1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after delete error = %v, want ErrNotFound", err)
	}
}

func TestSavedQueryRepositoryListByConnection(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	seedConnection(t, db, "c1")
	seedConnection(t, db, "c2")
	repo := NewSavedQueryRepository(db)

	queries := []*model.SavedQuery{
		{ID: "s1", ConnectionID: ptr("c1"), Title: "One", SQLText: "SELECT 1"},
		{ID: "s2", ConnectionID: ptr("c2"), Title: "Two", SQLText: "SELECT 2"},
		{ID: "s3", Title: "Unbound", SQLText: "SELECT 3"},
	}
	for _, q := range queries {
		if err := repo.Create(ctx, q); err != nil {
			t.Fatalf("Create(%s) error = %v", q.ID, err)
		}
	}

	forC1, err := repo.ListByConnection(ctx, "c1", 0, 0)
	if err != nil {
		t.Fatalf("ListByConnection() error = %v", err)
	}
	if len(forC1) != 1 || forC1[0].ID != "s1" {
		t.Errorf("ListByConnection(c1) = %+v", forC1)
	}

	all, err := repo.List(ctx, 0, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 3 {
		t.Errorf("List() len = %d, want 3", len(all))
	}
}

func TestSavedQueryRepositoryConnectionSetNullOnDelete(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	seedConnection(t, db, "c1")
	repo := NewSavedQueryRepository(db)

	if err := repo.Create(ctx, &model.SavedQuery{
		ID: "s1", ConnectionID: ptr("c1"), Title: "Bound", SQLText: "SELECT 1",
	}); err != nil {
		t.Fatal(err)
	}

	if err := NewConnectionRepository(db).Delete(ctx, "c1"); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, "s1")
	if err != nil {
		t.Fatalf("Get() after connection delete error = %v", err)
	}
	if got.ConnectionID != nil {
		t.Errorf("ConnectionID = %v, want nil (ON DELETE SET NULL)", got.ConnectionID)
	}
}

func TestSavedQueryRepositoryForeignKey(t *testing.T) {
	ctx := context.Background()
	repo := NewSavedQueryRepository(newTestDB(t))

	err := repo.Create(ctx, &model.SavedQuery{
		ID: "s1", ConnectionID: ptr("missing"), Title: "Bound", SQLText: "SELECT 1",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Create() with missing connection error = %v, want ErrNotFound", err)
	}
}

func TestSavedQueryRepositoryNotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewSavedQueryRepository(newTestDB(t))

	if _, err := repo.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
	if err := repo.Update(ctx, &model.SavedQuery{ID: "missing", Title: "x", SQLText: "y"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update() error = %v, want ErrNotFound", err)
	}
}

func TestSavedQueryRepositoryDatabaseContext(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	seedConnection(t, db, "c1")
	repo := NewSavedQueryRepository(db)

	database := "reporting"
	bound := &model.SavedQuery{
		ID: "s-db", ConnectionID: ptr("c1"), DatabaseName: &database,
		Title: "Bound", SQLText: "SELECT 1",
	}
	if err := repo.Create(ctx, bound); err != nil {
		t.Fatalf("Create(bound) error = %v", err)
	}
	legacy := &model.SavedQuery{ID: "s-legacy", ConnectionID: ptr("c1"), Title: "Legacy", SQLText: "SELECT 2"}
	if err := repo.Create(ctx, legacy); err != nil {
		t.Fatalf("Create(legacy) error = %v", err)
	}

	got, err := repo.Get(ctx, "s-db")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.DatabaseName == nil || *got.DatabaseName != "reporting" {
		t.Errorf("database context = %v, want reporting", got.DatabaseName)
	}

	gotLegacy, err := repo.Get(ctx, "s-legacy")
	if err != nil {
		t.Fatalf("Get(legacy) error = %v", err)
	}
	if gotLegacy.DatabaseName != nil {
		t.Errorf("legacy database context = %v, want nil", gotLegacy.DatabaseName)
	}

	// Update preserves/changes the context.
	*got.DatabaseName = "analytics"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, _ := repo.Get(ctx, "s-db")
	if updated.DatabaseName == nil || *updated.DatabaseName != "analytics" {
		t.Errorf("updated database context = %v, want analytics", updated.DatabaseName)
	}
}
