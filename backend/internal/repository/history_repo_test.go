package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func historyFixture(id, connectionID string, at time.Time) *model.QueryHistory {
	return &model.QueryHistory{
		ID:              id,
		ConnectionID:    connectionID,
		SQLText:         "SELECT 1",
		Status:          model.QueryStatusSuccess,
		ExecutionTimeMS: 5,
		RowsAffected:    1,
		ExecutedAt:      at,
	}
}

func TestQueryHistoryRepositoryOrdering(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	seedConnection(t, db, "c1")
	repo := NewQueryHistoryRepository(db)

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i, id := range []string{"h1", "h2", "h3"} {
		if err := repo.Create(ctx, historyFixture(id, "c1", base.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatalf("Create(%s) error = %v", id, err)
		}
	}

	records, err := repo.List(ctx, HistoryFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("List() len = %d, want 3", len(records))
	}
	if records[0].ID != "h3" || records[1].ID != "h2" || records[2].ID != "h1" {
		t.Errorf("List() order = %s,%s,%s, want h3,h2,h1", records[0].ID, records[1].ID, records[2].ID)
	}
}

func TestQueryHistoryRepositoryFilterByConnection(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	seedConnection(t, db, "c1")
	seedConnection(t, db, "c2")
	repo := NewQueryHistoryRepository(db)

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if err := repo.Create(ctx, historyFixture("h1", "c1", base)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, historyFixture("h2", "c2", base.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}

	filtered, err := repo.List(ctx, HistoryFilter{ConnectionID: "c1"})
	if err != nil {
		t.Fatalf("List(filtered) error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != "h1" {
		t.Errorf("filtered = %+v, want only h1", filtered)
	}

	all, err := repo.List(ctx, HistoryFilter{})
	if err != nil {
		t.Fatalf("List(all) error = %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List(all) len = %d, want 2", len(all))
	}
}

func TestQueryHistoryRepositoryLimit(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	seedConnection(t, db, "c1")
	repo := NewQueryHistoryRepository(db)

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i, id := range []string{"h1", "h2", "h3"} {
		if err := repo.Create(ctx, historyFixture(id, "c1", base.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatal(err)
		}
	}

	records, err := repo.List(ctx, HistoryFilter{Limit: 1})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(records) != 1 || records[0].ID != "h3" {
		t.Errorf("List(Limit:1) = %+v, want newest h3 only", records)
	}
}

func TestQueryHistoryRepositoryForeignKey(t *testing.T) {
	ctx := context.Background()
	repo := NewQueryHistoryRepository(newTestDB(t))

	err := repo.Create(ctx, historyFixture("h1", "missing", time.Now().UTC()))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Create() with missing connection error = %v, want ErrNotFound", err)
	}
}

func TestQueryHistoryRepositoryCascadeOnConnectionDelete(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	seedConnection(t, db, "c1")
	history := NewQueryHistoryRepository(db)

	if err := history.Create(ctx, historyFixture("h1", "c1", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	if err := NewConnectionRepository(db).Delete(ctx, "c1"); err != nil {
		t.Fatal(err)
	}

	records, err := history.List(ctx, HistoryFilter{ConnectionID: "c1"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(records) != 0 {
		t.Errorf("history after connection delete = %d records, want 0 (cascade)", len(records))
	}
}

func TestQueryHistoryRepositoryStatusConstraint(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	seedConnection(t, db, "c1")
	repo := NewQueryHistoryRepository(db)

	record := historyFixture("h1", "c1", time.Now().UTC())
	record.Status = model.QueryStatus("PENDING")
	if err := repo.Create(ctx, record); err == nil {
		t.Fatal("Create() with invalid status error = nil, want constraint error")
	}
}
