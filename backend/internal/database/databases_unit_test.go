package database

import (
	"context"
	"database/sql"
	"errors"
	"sync"
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

// candidateConnector records which bootstrap candidates are probed and can fail
// selected databases. It implements DatabaseLister so Test() uses bootstrap.
type candidateConnector struct {
	name  model.Driver
	fail  map[string]bool
	tried []string
	mu    sync.Mutex
	opens int
}

func newCandidateConnector() *candidateConnector {
	return &candidateConnector{name: model.DriverPostgres, fail: map[string]bool{}}
}

func (c *candidateConnector) Name() model.Driver { return c.name }

func (c *candidateConnector) Open(_ context.Context, cfg Config, _ Options) (*sql.DB, error) {
	c.mu.Lock()
	c.tried = append(c.tried, cfg.Database)
	c.opens++
	fail := c.fail[cfg.Database]
	c.mu.Unlock()
	if fail {
		return nil, errors.New("connection refused")
	}
	return sql.OpenDB(fakeSQLConnector{d: fakeSQLDriver{}}), nil
}

func (c *candidateConnector) Ping(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
}
func (c *candidateConnector) Introspect(context.Context, *sql.DB) ([]model.Database, error) {
	return nil, nil
}
func (c *candidateConnector) Execute(context.Context, *sql.DB, string) (model.QueryResult, error) {
	return model.QueryResult{}, nil
}
func (c *candidateConnector) Capabilities() model.Capabilities { return model.Capabilities{} }
func (c *candidateConnector) ListDatabases(context.Context, *sql.DB) ([]model.DatabaseInfo, error) {
	return nil, nil
}

func TestTestConnectionUsesBootstrapWhenDatabaseEmpty(t *testing.T) {
	ctx := context.Background()

	t.Run("first candidate succeeds", func(t *testing.T) {
		fc := newCandidateConnector()
		m := NewManager(DefaultOptions(), fc)
		if err := m.Test(ctx, Config{Driver: model.DriverPostgres, Host: "h", Port: 5432, Username: "u"}); err != nil {
			t.Fatalf("Test() error = %v", err)
		}
		if len(fc.tried) != 1 || fc.tried[0] != "postgres" {
			t.Errorf("candidates tried = %v, want [postgres]", fc.tried)
		}
	})

	t.Run("falls through to login database", func(t *testing.T) {
		fc := newCandidateConnector()
		fc.fail["postgres"] = true
		m := NewManager(DefaultOptions(), fc)
		if err := m.Test(ctx, Config{Driver: model.DriverPostgres, Host: "h", Port: 5432, Username: "u"}); err != nil {
			t.Fatalf("Test() error = %v", err)
		}
		if len(fc.tried) != 2 || fc.tried[1] != "u" {
			t.Errorf("candidates tried = %v, want [postgres u]", fc.tried)
		}
	})

	t.Run("all candidates fail", func(t *testing.T) {
		fc := newCandidateConnector()
		fc.fail["postgres"] = true
		fc.fail["u"] = true
		m := NewManager(DefaultOptions(), fc)
		if err := m.Test(ctx, Config{Driver: model.DriverPostgres, Host: "h", Port: 5432, Username: "u"}); !errors.Is(err, ErrNoBootstrapDatabase) {
			t.Errorf("error = %v, want ErrNoBootstrapDatabase", err)
		}
	})

	t.Run("explicit database is not bootstrapped", func(t *testing.T) {
		fc := newCandidateConnector()
		fc.fail["app"] = true
		m := NewManager(DefaultOptions(), fc)
		err := m.Test(ctx, Config{Driver: model.DriverPostgres, Host: "h", Port: 5432, Username: "u", Database: "app"})
		if err == nil || errors.Is(err, ErrNoBootstrapDatabase) {
			t.Errorf("error = %v, want a normal connection failure", err)
		}
		if len(fc.tried) != 1 || fc.tried[0] != "app" {
			t.Errorf("candidates tried = %v, want [app]", fc.tried)
		}
	})
}
