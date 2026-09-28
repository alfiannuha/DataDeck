//go:build integration

package database_test

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/database"
)

// TestPostgresDatabasePoolsIntegration proves per-database pool identity against
// a real PostgreSQL server (PRF-01/T03).
func TestPostgresDatabasePoolsIntegration(t *testing.T) {
	base := pgConfig(t)
	ctx := context.Background()
	manager := database.NewManager(database.DefaultOptions(), database.Postgres{})
	defer func() { _ = manager.CloseAll() }()
	admin := adminDB(t, manager, base)

	const (
		dbA = "datadeck_pool_alpha"
		dbB = "datadeck_pool_beta"
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

	currentDatabase := func(id string, cfg database.Config) string {
		t.Helper()
		result, err := manager.Execute(ctx, id, cfg, "SELECT current_database()")
		if err != nil {
			t.Fatalf("Execute(%s) error = %v", cfg.Database, err)
		}
		return result.Rows[0][0].(string)
	}

	t.Run("same profile different databases use isolated pools", func(t *testing.T) {
		poolA, err := manager.Open(ctx, "ccm", cfgA)
		if err != nil {
			t.Fatalf("Open(A) error = %v", err)
		}
		poolB, err := manager.Open(ctx, "ccm", cfgB)
		if err != nil {
			t.Fatalf("Open(B) error = %v", err)
		}
		if poolA == poolB {
			t.Fatal("different databases shared one pool")
		}
		if got := currentDatabase("ccm", cfgA); got != dbA {
			t.Errorf("current_database = %q, want %q", got, dbA)
		}
		if got := currentDatabase("ccm", cfgB); got != dbB {
			t.Errorf("current_database = %q, want %q", got, dbB)
		}
		// Same profile + same database reuses the pool.
		again, _ := manager.Open(ctx, "ccm", cfgA)
		if again != poolA {
			t.Error("same (connection, database) did not reuse the pool")
		}
	})

	t.Run("two profiles with the same database name stay isolated", func(t *testing.T) {
		_, err := manager.Open(ctx, "ccm", cfgA)
		if err != nil {
			t.Fatalf("Open(ccm) error = %v", err)
		}
		other, err := manager.Open(ctx, "other", cfgA)
		if err != nil {
			t.Fatalf("Open(other) error = %v", err)
		}
		stored, _ := manager.Get("ccm", dbA)
		if stored == other {
			t.Fatal("two profiles with the same database name shared a pool")
		}
	})

	t.Run("deletion closes every database pool", func(t *testing.T) {
		if err := manager.CloseConnection("ccm"); err != nil {
			t.Fatalf("CloseConnection() error = %v", err)
		}
		for _, name := range []string{dbA, dbB} {
			if _, ok := manager.Get("ccm", name); ok {
				t.Errorf("pool for %s still registered after CloseConnection", name)
			}
		}
		if _, ok := manager.Get("other", dbA); !ok {
			t.Error("unrelated connection pool was closed")
		}
	})

	t.Run("credential change reconnects", func(t *testing.T) {
		before, err := manager.Open(ctx, "rot", cfgA)
		if err != nil {
			t.Fatalf("Open(rot) error = %v", err)
		}
		if err := manager.CloseConnection("rot"); err != nil {
			t.Fatalf("CloseConnection(rot) error = %v", err)
		}
		after, err := manager.Open(ctx, "rot", cfgA)
		if err != nil {
			t.Fatalf("Open(rot after rotation) error = %v", err)
		}
		if before == after {
			t.Error("pool reused after a credential/config change")
		}
	})

	t.Run("database outage evicts the pool and recovers", func(t *testing.T) {
		proxy := newTCPProxy(t, net.JoinHostPort(base.Host, strconv.Itoa(base.Port)))
		host, port := proxy.hostPort()
		proxied := cfgA
		proxied.Host, proxied.Port = host, port

		if _, err := manager.Open(ctx, "outage", proxied); err != nil {
			t.Fatalf("Open(proxied) error = %v", err)
		}
		if err := manager.CloseConnection("outage"); err != nil {
			t.Fatalf("reset outage pool: %v", err)
		}
		if _, err := manager.Open(ctx, "outage", proxied); err != nil {
			t.Fatalf("Open(proxied) error = %v", err)
		}
		proxy.cut()
		if _, err := manager.Execute(ctx, "outage", proxied, "SELECT 1"); !errors.Is(err, database.ErrConnection) {
			t.Fatalf("Execute() error = %v, want ErrConnection", err)
		}
		if _, ok := manager.Get("outage", dbA); ok {
			t.Error("unhealthy pool was not evicted")
		}
		// Direct connection recovers.
		if got := currentDatabase("outage", cfgA); got != dbA {
			t.Errorf("post-outage current_database = %q, want %q", got, dbA)
		}
	})
}
