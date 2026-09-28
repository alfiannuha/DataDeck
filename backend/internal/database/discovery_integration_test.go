//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

func adminDB(t *testing.T, manager *database.Manager, cfg database.Config) *sql.DB {
	t.Helper()
	adminCfg := cfg
	adminCfg.Database = envOr("DATADECK_TEST_PG_DATABASE", "postgres")
	db, err := manager.Open(context.Background(), "admin", adminCfg)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	return db
}

func mustExec(t *testing.T, db *sql.DB, sqlText string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), sqlText); err != nil {
		t.Fatalf("exec %q: %v", sqlText, err)
	}
}

func TestPostgresDatabaseDiscoveryIntegration(t *testing.T) {
	cfg := pgConfig(t)
	ctx := context.Background()
	manager := database.NewManager(database.DefaultOptions(), database.Postgres{})
	defer func() { _ = manager.CloseAll() }()
	admin := adminDB(t, manager, cfg)

	const (
		alpha  = "datadeck_alpha"
		beta   = "datadeck_beta"
		gamma  = "datadeck_gamma"
		noConn = "datadeck_noconn"
		role   = "dd_limited_discovery"
	)

	// Fixture: three normal databases, one that refuses connections, and a
	// limited login role.
	t.Cleanup(func() {
		// Databases first (drops the role's CONNECT grants), then clear any
		// remaining privileges/dev objects before dropping the role.
		for _, name := range []string{gamma, beta, alpha, noConn} {
			_ = dbExec(admin, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		}
		_ = dbExec(admin, "DROP OWNED BY "+role+" CASCADE")
		_ = dbExec(admin, "DROP ROLE IF EXISTS "+role)
	})
	for _, name := range []string{alpha, beta, gamma} {
		_ = dbExec(admin, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		mustExec(t, admin, "CREATE DATABASE "+name)
	}
	_ = dbExec(admin, "DROP DATABASE IF EXISTS "+noConn+" WITH (FORCE)")
	mustExec(t, admin, "CREATE DATABASE "+noConn)
	mustExec(t, admin, "ALTER DATABASE "+noConn+" ALLOW_CONNECTIONS false")

	// Clear any leftovers from an interrupted previous run.
	_ = dbExec(admin, "DROP OWNED BY "+role+" CASCADE")
	_ = dbExec(admin, "DROP ROLE IF EXISTS "+role)
	mustExec(t, admin, "CREATE ROLE "+role+" LOGIN PASSWORD 'discovery-secret'")
	mustExec(t, admin, "REVOKE CONNECT ON DATABASE "+gamma+" FROM PUBLIC")
	mustExec(t, admin, "GRANT CONNECT ON DATABASE "+alpha+", "+beta+", "+noConn+" TO "+role)
	mustExec(t, admin, "GRANT CONNECT ON DATABASE "+envOr("DATADECK_TEST_PG_DATABASE", "postgres")+" TO "+role)

	names := func(list []model.DatabaseInfo) map[string]bool {
		out := map[string]bool{}
		for _, d := range list {
			out[d.Name] = true
		}
		return out
	}

	t.Run("explicit database lists connectable databases", func(t *testing.T) {
		list, err := manager.ListDatabases(ctx, "c", cfg)
		if err != nil {
			t.Fatalf("ListDatabases() error = %v", err)
		}
		got := names(list)
		for _, want := range []string{alpha, beta, gamma} {
			if !got[want] {
				t.Errorf("discovery missing %q (got %v)", want, got)
			}
		}
		if got["template0"] || got["template1"] {
			t.Errorf("templates must be excluded: %v", got)
		}
		if got[noConn] {
			t.Errorf("database refusing connections must be excluded: %v", got)
		}

		var bootstrap model.DatabaseInfo
		for _, d := range list {
			if d.Name == "postgres" {
				bootstrap = d
			}
		}
		if !bootstrap.BootstrapCandidate {
			t.Error("postgres should be marked as the bootstrap candidate")
		}
	})

	t.Run("bootstrap resolution without a default database", func(t *testing.T) {
		serverOnly := cfg
		serverOnly.Database = ""
		list, err := manager.ListDatabases(ctx, "c", serverOnly)
		if err != nil {
			t.Fatalf("bootstrap ListDatabases() error = %v", err)
		}
		got := names(list)
		if !got[alpha] || !got[beta] {
			t.Errorf("bootstrap discovery incomplete: %v", got)
		}
	})

	t.Run("CONNECT permission filters the list", func(t *testing.T) {
		limited := cfg
		limited.Database = ""
		limited.Username = role
		limited.Password = "discovery-secret"

		list, err := manager.ListDatabases(ctx, "c", limited)
		if err != nil {
			t.Fatalf("limited ListDatabases() error = %v", err)
		}
		got := names(list)
		if !got[alpha] || !got[beta] {
			t.Errorf("limited role should see granted databases: %v", got)
		}
		if got[gamma] {
			t.Errorf("limited role must not see a database it lacks CONNECT on: %v", got)
		}
		if got[noConn] {
			t.Errorf("limited role must not see a no-connect database: %v", got)
		}
	})

	t.Run("single-database profiles still work", func(t *testing.T) {
		single := cfg
		single.Database = alpha
		if _, err := manager.ListDatabases(ctx, "c", single); err != nil {
			t.Fatalf("explicit single-database ListDatabases() error = %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		cancelCtx, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()
		if _, err := manager.ListDatabases(cancelCtx, "c", cfg); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("unavailable server", func(t *testing.T) {
		down := cfg
		down.Host, down.Port, down.Database = "127.0.0.1", 1, "postgres"
		if _, err := manager.ListDatabases(ctx, "c", down); !errors.Is(err, database.ErrConnection) {
			t.Errorf("error = %v, want ErrConnection", err)
		}
	})

	t.Run("bad credentials are sanitized", func(t *testing.T) {
		bad := cfg
		bad.Database = "postgres"
		bad.Username = "dd_no_such_role"
		bad.Password = "discovery-secret"
		_, err := manager.ListDatabases(ctx, "c", bad)
		if !errors.Is(err, database.ErrConnection) {
			t.Fatalf("error = %v, want ErrConnection", err)
		}
		message := err.Error()
		for _, leak := range []string{"discovery-secret", "password", "postgres://"} {
			if strings.Contains(strings.ToLower(message), strings.ToLower(leak)) {
				t.Errorf("error message leaked %q: %s", leak, message)
			}
		}
	})
}

func dbExec(db *sql.DB, sqlText string) error {
	_, err := db.ExecContext(context.Background(), sqlText)
	return err
}
