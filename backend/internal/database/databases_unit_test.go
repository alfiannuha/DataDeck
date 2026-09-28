package database

import (
	"context"
	"errors"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestPostgresAdvertisesMultipleDatabases(t *testing.T) {
	caps := (Postgres{}).Capabilities()
	if !caps.MultipleDatabases {
		t.Error("Postgres.MultipleDatabases = false, want true")
	}
	if (MySQL{}).Capabilities().MultipleDatabases {
		t.Error("MySQL.MultipleDatabases = true, want false")
	}
	if (SQLite{}).Capabilities().MultipleDatabases {
		t.Error("SQLite.MultipleDatabases = true, want false")
	}
}

func TestListDatabasesUnsupportedDrivers(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(DefaultOptions(), Postgres{}, MySQL{}, SQLite{})

	for _, driver := range []model.Driver{model.DriverMySQL, model.DriverSQLite} {
		_, err := manager.ListDatabases(ctx, "any", Config{Driver: driver, Database: "d"})
		if !errors.Is(err, ErrNotImplemented) {
			t.Errorf("ListDatabases(%s) error = %v, want ErrNotImplemented", driver, err)
		}
	}

	// A connector that does not implement DatabaseLister is also unsupported.
	fake := newFakeConnector() // name: postgres, but no ListDatabases method
	manager = NewManager(DefaultOptions(), fake)
	if _, err := manager.ListDatabases(ctx, "c1", testConfig()); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("ListDatabases(fake) error = %v, want ErrNotImplemented", err)
	}
}

func TestBootstrapCandidates(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want []string
	}{
		{"explicit database wins", Config{Database: "app", Username: "u"}, []string{"app"}},
		{"default candidate then user", Config{Username: "u"}, []string{"postgres", "u"}},
		{"postgres user no duplicate", Config{Username: "postgres"}, []string{"postgres"}},
		{"no username", Config{}, []string{"postgres"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bootstrapCandidates(tc.cfg)
			if len(got) != len(tc.want) {
				t.Fatalf("candidates = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("candidates = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestNoBootstrapDatabaseSentinel verifies the distinct bootstrap failure when a
// profile has no database and no candidate is reachable (unreachable host).
func TestNoBootstrapDatabaseSentinel(t *testing.T) {
	manager := NewManager(Options{PingTimeout: 200 * 1e6}, Postgres{})
	cfg := Config{Driver: model.DriverPostgres, Host: "127.0.0.1", Port: 1, Username: "u"}
	if _, err := manager.ListDatabases(context.Background(), "c1", cfg); !errors.Is(err, ErrNoBootstrapDatabase) {
		t.Errorf("error = %v, want ErrNoBootstrapDatabase", err)
	}
	if _, err := manager.ListDatabases(context.Background(), "c1", Config{Driver: model.DriverPostgres, Host: "127.0.0.1", Port: 1, Database: "app"}); errors.Is(err, ErrNoBootstrapDatabase) {
		t.Error("explicit database failure must not be reported as ErrNoBootstrapDatabase")
	}
}
