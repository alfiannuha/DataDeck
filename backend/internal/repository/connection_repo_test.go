package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func sampleProfile(id, name string) *model.ConnectionProfile {
	return &model.ConnectionProfile{
		ID:                id,
		Name:              name,
		Driver:            model.DriverPostgres,
		Host:              ptr("db.example.com"),
		Port:              ptr(5432),
		DatabaseName:      "app",
		Username:          ptr("readonly"),
		EncryptedPassword: ptr("ciphertext-value"),
		SSLMode:           "require",
	}
}

func TestConnectionRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewConnectionRepository(newTestDB(t))

	created := sampleProfile("c1", "Alpha")
	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("Create() did not assign timestamps")
	}

	got, err := repo.Get(ctx, "c1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "Alpha" || got.Driver != model.DriverPostgres || got.DatabaseName != "app" {
		t.Errorf("Get() = %+v", got)
	}
	if got.Host == nil || *got.Host != "db.example.com" {
		t.Errorf("Host = %v", got.Host)
	}
	if got.Port == nil || *got.Port != 5432 {
		t.Errorf("Port = %v", got.Port)
	}
	if got.EncryptedPassword == nil || *got.EncryptedPassword != "ciphertext-value" {
		t.Errorf("EncryptedPassword = %v", got.EncryptedPassword)
	}
	if got.SSLMode != "require" {
		t.Errorf("SSLMode = %q", got.SSLMode)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt not persisted")
	}

	if err := repo.Create(ctx, sampleProfile("c2", "Beta")); err != nil {
		t.Fatalf("Create() second error = %v", err)
	}
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
	if list[0].ID != "c1" || list[1].ID != "c2" {
		t.Errorf("List() order = %q,%q, want c1,c2", list[0].ID, list[1].ID)
	}

	got.Name = "Alpha Updated"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := repo.Get(ctx, "c1")
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if updated.Name != "Alpha Updated" {
		t.Errorf("Name after update = %q", updated.Name)
	}

	if err := repo.Delete(ctx, "c1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Get(ctx, "c1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after delete error = %v, want ErrNotFound", err)
	}
}

func TestConnectionRepositoryNullableFields(t *testing.T) {
	ctx := context.Background()
	repo := NewConnectionRepository(newTestDB(t))

	profile := &model.ConnectionProfile{
		ID:           "n1",
		Name:         "Local SQLite",
		Driver:       model.DriverSQLite,
		DatabaseName: "local.db",
	}
	if err := repo.Create(ctx, profile); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.Get(ctx, "n1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Host != nil || got.Port != nil || got.Username != nil || got.EncryptedPassword != nil {
		t.Errorf("expected nil optional fields, got %+v", got)
	}
	if got.SSHHost != nil || got.SSHPort != nil || got.SSHUsername != nil || got.SSHKeyPath != nil {
		t.Errorf("expected nil SSH fields, got %+v", got)
	}
	if got.SSLMode != "disable" {
		t.Errorf("SSLMode = %q, want disable", got.SSLMode)
	}
	if got.SSHEnabled {
		t.Error("SSHEnabled = true, want false")
	}
}

func TestConnectionRepositoryCreateConflict(t *testing.T) {
	ctx := context.Background()
	repo := NewConnectionRepository(newTestDB(t))

	if err := repo.Create(ctx, sampleProfile("dup", "One")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	err := repo.Create(ctx, sampleProfile("dup", "Two"))
	if !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate Create() error = %v, want ErrConflict", err)
	}
}

func TestConnectionRepositoryNotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewConnectionRepository(newTestDB(t))

	if _, err := repo.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
	if err := repo.Update(ctx, sampleProfile("missing", "X")); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update() error = %v, want ErrNotFound", err)
	}
}

func TestConnectionRepositoryDriverConstraint(t *testing.T) {
	ctx := context.Background()
	repo := NewConnectionRepository(newTestDB(t))

	profile := sampleProfile("bad", "Bad")
	profile.Driver = model.Driver("oracle")
	if err := repo.Create(ctx, profile); err == nil {
		t.Fatal("Create() with unsupported driver error = nil, want constraint error")
	}
}
