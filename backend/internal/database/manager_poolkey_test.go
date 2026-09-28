package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func cfgFor(database string) Config {
	cfg := testConfig()
	cfg.Database = database
	return cfg
}

// TestPoolKeyIsolatesConnectionAndDatabase covers the core PRF-01 identity:
// the same profile/DB is reused, different DBs and different profiles never
// share a pool.
func TestPoolKeyIsolatesConnectionAndDatabase(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	// same connection + same database -> reuse
	a1, err := m.Open(ctx, "ccm", cfgFor("CCM"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	a2, _ := m.Open(ctx, "ccm", cfgFor("CCM"))
	if a1 != a2 {
		t.Error("same (connection, database) did not reuse the pool")
	}
	if fc.opens.Load() != 1 {
		t.Errorf("opens = %d, want 1", fc.opens.Load())
	}

	// same connection + different database -> different pools
	reporting, err := m.Open(ctx, "ccm", cfgFor("reporting"))
	if err != nil {
		t.Fatalf("Open(reporting) error = %v", err)
	}
	if reporting == a1 {
		t.Fatal("different database reused the CCM pool")
	}
	if fc.opens.Load() != 2 {
		t.Errorf("opens = %d, want 2", fc.opens.Load())
	}

	// different connection + same database name -> isolated pools
	other, err := m.Open(ctx, "other", cfgFor("CCM"))
	if err != nil {
		t.Fatalf("Open(other) error = %v", err)
	}
	if other == a1 {
		t.Fatal("two profiles with the same database name shared a pool")
	}
	if fc.opens.Load() != 3 {
		t.Errorf("opens = %d, want 3", fc.opens.Load())
	}
}

// TestPoolKeyConcurrentActivation ensures concurrent activations of different
// databases on one connection stay controlled (one pool per key).
func TestPoolKeyConcurrentActivation(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	fc.pingDelay = 20 * 1e6 // widen the window, 20ms
	m := NewManager(DefaultOptions(), fc)

	databases := []string{"alpha", "beta", "gamma"}
	type opened struct {
		database string
		db       *sql.DB
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]opened, 0, len(databases)*4)
	for _, database := range databases {
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func(database string) {
				defer wg.Done()
				db, err := m.Open(ctx, "ccm", cfgFor(database))
				if err != nil {
					t.Errorf("Open(%s) error = %v", database, err)
					return
				}
				mu.Lock()
				results = append(results, opened{database, db})
				mu.Unlock()
			}(database)
		}
	}
	wg.Wait()

	if got := fc.opens.Load(); got != int64(len(databases)) {
		t.Errorf("connector opens = %d, want %d (one per database)", got, len(databases))
	}
	byDatabase := map[string]*sql.DB{}
	for _, r := range results {
		if existing, ok := byDatabase[r.database]; ok && existing != r.db {
			t.Errorf("database %s returned multiple pools", r.database)
		}
		byDatabase[r.database] = r.db
	}
}

// TestCloseConnectionClosesAllDatabasePools covers profile deletion.
func TestCloseConnectionClosesAllDatabasePools(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	a, _ := m.Open(ctx, "ccm", cfgFor("alpha"))
	b, _ := m.Open(ctx, "ccm", cfgFor("beta"))
	other, _ := m.Open(ctx, "other", cfgFor("alpha"))

	if err := m.CloseConnection("ccm"); err != nil {
		t.Fatalf("CloseConnection() error = %v", err)
	}
	if _, ok := m.Get("ccm", "alpha"); ok {
		t.Error("alpha pool still registered after CloseConnection")
	}
	if _, ok := m.Get("ccm", "beta"); ok {
		t.Error("beta pool still registered after CloseConnection")
	}
	if _, ok := m.Get("other", "alpha"); !ok {
		t.Error("unrelated connection pool was closed")
	}
	if err := a.Ping(); err == nil {
		t.Error("alpha pool still usable after CloseConnection")
	}
	if err := b.Ping(); err == nil {
		t.Error("beta pool still usable after CloseConnection")
	}
	if err := other.Ping(); err != nil {
		t.Errorf("unrelated pool was closed: %v", err)
	}
	if err := m.CloseConnection("missing"); err != nil {
		t.Errorf("CloseConnection(no pools) error = %v, want nil", err)
	}
}

// TestCredentialChangeReconnects covers credential/config changes: after closing
// the connection, the next use opens a pool with the new configuration.
func TestCredentialChangeReconnects(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	before, _ := m.Open(ctx, "ccm", cfgFor("CCM"))
	if err := m.CloseConnection("ccm"); err != nil {
		t.Fatalf("CloseConnection() error = %v", err)
	}

	rotated := cfgFor("CCM")
	rotated.Username = "other-user"
	rotated.Password = "rotated-secret"
	after, err := m.Open(ctx, "ccm", rotated)
	if err != nil {
		t.Fatalf("Open(after rotation) error = %v", err)
	}
	if before == after {
		t.Error("pool was reused after a credential/config change")
	}
	if fc.opens.Load() != 2 {
		t.Errorf("opens = %d, want 2", fc.opens.Load())
	}
}

// TestPoolCacheIsBounded asserts the per-connection LRU cap.
func TestPoolCacheIsBounded(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	opts := DefaultOptions()
	opts.MaxPoolsPerConnection = 2
	m := NewManager(opts, fc)

	_, _ = m.Open(ctx, "ccm", cfgFor("alpha"))
	_, _ = m.Open(ctx, "ccm", cfgFor("beta"))
	_, _ = m.Open(ctx, "ccm", cfgFor("gamma"))

	active := 0
	for _, database := range []string{"alpha", "beta", "gamma"} {
		if _, ok := m.Get("ccm", database); ok {
			active++
		}
	}
	if active != 2 {
		t.Errorf("active pools = %d, want 2 (cap)", active)
	}
	if _, ok := m.Get("ccm", "alpha"); ok {
		t.Error("oldest pool (alpha) should have been evicted")
	}
	if _, ok := m.Get("ccm", "gamma"); !ok {
		t.Error("newest pool (gamma) should be cached")
	}
}

// TestUnhealthyPoolIsEvicted ensures a connection-loss error drops the pool so
// the next use reconnects.
func TestUnhealthyPoolIsEvicted(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	fc.executeErr = fmt.Errorf("%w: connection reset", ErrConnection)
	m := NewManager(DefaultOptions(), fc)

	if _, err := m.Open(ctx, "ccm", cfgFor("CCM")); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := m.Execute(ctx, "ccm", cfgFor("CCM"), "SELECT 1"); !errors.Is(err, ErrConnection) {
		t.Fatalf("Execute() error = %v, want ErrConnection", err)
	}
	if _, ok := m.Get("ccm", "CCM"); ok {
		t.Error("unhealthy pool was not evicted")
	}
}

// TestPoolKeyNeverCarriesCredentials guards against secret material entering
// keys (and therefore logs/errors).
func TestPoolKeyNeverCarriesCredentials(t *testing.T) {
	key := poolKey("ccm", Config{Database: "CCM", Username: "u", Password: "super-secret"})
	if got := key.String(); got != "ccm/CCM" {
		t.Errorf("PoolKey.String() = %q, want %q", got, "ccm/CCM")
	}
}
