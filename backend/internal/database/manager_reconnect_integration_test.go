//go:build integration

package database_test

import (
	"context"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/database"
)

// TestPostgresReconnectIsDatabaseScopedIntegration proves that a database-scoped
// reconnect replaces only the targeted pool and leaves every other database pool
// on the same server-level profile intact and usable (PRF-02/T16).
func TestPostgresReconnectIsDatabaseScopedIntegration(t *testing.T) {
	base := pgConfig(t)
	ctx := context.Background()
	manager := database.NewManager(database.DefaultOptions(), database.Postgres{})
	defer func() { _ = manager.CloseAll() }()
	admin := adminDB(t, manager, base)

	const (
		dbA = "datadeck_rc_alpha"
		dbB = "datadeck_rc_beta"
	)
	t.Cleanup(func() {
		for _, name := range []string{dbA, dbB} {
			_ = dbExec(admin, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		}
	})
	for _, name := range []string{dbA, dbB} {
		_ = dbExec(admin, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		mustExec(t, admin, "CREATE DATABASE "+name)
	}

	cfgA := base
	cfgA.Database = dbA
	cfgB := base
	cfgB.Database = dbB

	poolA, err := manager.Open(ctx, "ccm", cfgA)
	if err != nil {
		t.Fatalf("Open(alpha) error = %v", err)
	}
	poolB, err := manager.Open(ctx, "ccm", cfgB)
	if err != nil {
		t.Fatalf("Open(beta) error = %v", err)
	}

	if err := manager.Reconnect(ctx, "ccm", cfgA); err != nil {
		t.Fatalf("Reconnect(alpha) error = %v", err)
	}

	afterA, ok := manager.Get("ccm", dbA)
	if !ok {
		t.Fatal("alpha pool missing after reconnect")
	}
	if afterA == poolA {
		t.Error("Reconnect(alpha) kept the previous alpha pool pointer")
	}
	afterB, ok := manager.Get("ccm", dbB)
	if !ok {
		t.Fatal("beta pool was closed by reconnecting alpha")
	}
	if afterB != poolB {
		t.Error("Reconnect(alpha) replaced the beta pool")
	}

	// Beta must remain usable (no cross-database disturbance).
	result, err := manager.Execute(ctx, "ccm", cfgB, "SELECT current_database()")
	if err != nil {
		t.Fatalf("beta query after reconnect(alpha) error = %v", err)
	}
	if got := result.Rows[0][0].(string); got != dbB {
		t.Errorf("beta current_database = %q, want %q", got, dbB)
	}

	// Alpha must use the replacement pool and reach the same database.
	result, err = manager.Execute(ctx, "ccm", cfgA, "SELECT current_database()")
	if err != nil {
		t.Fatalf("alpha query after reconnect error = %v", err)
	}
	if got := result.Rows[0][0].(string); got != dbA {
		t.Errorf("alpha current_database = %q, want %q", got, dbA)
	}
}
