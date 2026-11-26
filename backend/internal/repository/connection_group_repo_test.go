package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestConnectionGroupRepository(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	groups := NewConnectionGroupRepository(db)
	connections := NewConnectionRepository(db)

	g1 := &model.ConnectionGroup{ID: "g1", Name: "Development"}
	g2 := &model.ConnectionGroup{ID: "g2", Name: "Production"}
	if err := groups.Create(ctx, g1); err != nil {
		t.Fatalf("create g1: %v", err)
	}
	if err := groups.Create(ctx, g2); err != nil {
		t.Fatalf("create g2: %v", err)
	}
	if g1.IsExpanded || g2.IsExpanded {
		t.Error("new groups must default to collapsed")
	}
	if err := groups.Create(ctx, &model.ConnectionGroup{ID: "g3", Name: "Development"}); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate name error = %v, want ErrConflict", err)
	}

	if err := connections.Create(ctx, sampleProfile("c1", "A")); err != nil {
		t.Fatalf("create c1: %v", err)
	}
	if err := connections.Create(ctx, sampleProfile("c2", "B")); err != nil {
		t.Fatalf("create c2: %v", err)
	}

	// Move c1 into g1.
	if err := connections.SetGroup(ctx, "c1", &g1.ID); err != nil {
		t.Fatalf("set group: %v", err)
	}
	c1, _ := connections.Get(ctx, "c1")
	if c1.GroupID == nil || *c1.GroupID != "g1" {
		t.Errorf("c1 group = %v, want g1", c1.GroupID)
	}
	// Unknown group rejected.
	missing := "missing"
	if err := connections.SetGroup(ctx, "c1", &missing); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown group error = %v, want ErrNotFound", err)
	}

	// Rename + expand.
	g1.Name = "Dev"
	g1.IsExpanded = true
	if err := groups.Update(ctx, g1); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := groups.Get(ctx, "g1")
	if got.Name != "Dev" || !got.IsExpanded {
		t.Errorf("group after update = %+v", got)
	}

	// Reorder groups.
	if err := groups.Reorder(ctx, []string{"g2", "g1"}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	list, _ := groups.List(ctx)
	if list[0].ID != "g2" || list[1].ID != "g1" {
		t.Fatalf("group order = %s,%s", list[0].ID, list[1].ID)
	}
	if err := groups.Reorder(ctx, []string{"g1"}); !errors.Is(err, ErrInvalidOrder) {
		t.Errorf("partial reorder error = %v, want ErrInvalidOrder", err)
	}

	// Deleting a group must NOT delete its connections; they become ungrouped.
	if err := groups.Delete(ctx, "g1"); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	if _, err := connections.Get(ctx, "c1"); err != nil {
		t.Fatalf("connection deleted with group: %v", err)
	}
	c1, _ = connections.Get(ctx, "c1")
	if c1.GroupID != nil {
		t.Errorf("c1 group after group delete = %v, want nil", c1.GroupID)
	}
}
