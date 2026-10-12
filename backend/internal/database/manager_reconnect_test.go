package database

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func dbConfig(database string) Config {
	return Config{Driver: model.DriverPostgres, Host: "h", Port: 1, Database: database}
}

func TestManagerReconnectReplacesPool(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	old, err := m.Open(ctx, "c1", dbConfig("d"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := m.Reconnect(ctx, "c1", dbConfig("d")); err != nil {
		t.Fatalf("Reconnect() error = %v", err)
	}
	got, ok := m.Get("c1", "d")
	if !ok {
		t.Fatal("Get() after Reconnect returned no pool")
	}
	if got == old {
		t.Error("Reconnect() kept the previous pool pointer")
	}
	if fc.opens.Load() != 2 {
		t.Errorf("connector opens = %d, want 2 (one replacement)", fc.opens.Load())
	}
	if err := old.Ping(); err == nil {
		t.Error("previous pool is still usable after Reconnect")
	}
	if err := got.Ping(); err != nil {
		t.Errorf("replacement pool is not usable: %v", err)
	}
}

// Reconnect is database-scoped: reconnecting one database must not touch the
// pools of other databases on the same connection profile.
func TestManagerReconnectIsDatabaseScoped(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	alpha, err := m.Open(ctx, "c1", dbConfig("alpha"))
	if err != nil {
		t.Fatalf("Open(alpha) error = %v", err)
	}
	beta, err := m.Open(ctx, "c1", dbConfig("beta"))
	if err != nil {
		t.Fatalf("Open(beta) error = %v", err)
	}

	if err := m.Reconnect(ctx, "c1", dbConfig("alpha")); err != nil {
		t.Fatalf("Reconnect(alpha) error = %v", err)
	}

	alphaAfter, ok := m.Get("c1", "alpha")
	if !ok || alphaAfter == alpha {
		t.Error("alpha pool was not replaced")
	}
	betaAfter, ok := m.Get("c1", "beta")
	if !ok || betaAfter != beta {
		t.Error("Reconnect(alpha) replaced the beta pool")
	}
	if err := beta.Ping(); err != nil {
		t.Errorf("beta pool became unusable after reconnecting alpha: %v", err)
	}
}

// A failed reconnect fails closed: the existing pool is kept and stays usable.
func TestManagerReconnectFailureKeepsOldPool(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	old, err := m.Open(ctx, "c1", dbConfig("d"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	fc.pingErr = errors.New("connection refused")
	if err := m.Reconnect(ctx, "c1", dbConfig("d")); err == nil {
		t.Fatal("Reconnect() error = nil, want failure")
	}

	got, ok := m.Get("c1", "d")
	if !ok || got != old {
		t.Error("failed Reconnect() discarded the previous healthy pool")
	}
	if err := old.Ping(); err != nil {
		t.Errorf("previous pool unusable after failed Reconnect(): %v", err)
	}
}

func TestManagerReconnectWithoutExistingPoolOpensFresh(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	if err := m.Reconnect(ctx, "c1", dbConfig("d")); err != nil {
		t.Fatalf("Reconnect() error = %v", err)
	}
	if _, ok := m.Get("c1", "d"); !ok {
		t.Error("Reconnect() did not register a pool when none existed")
	}
}

// Concurrent reconnects for the same key must not stack duplicate pools.
func TestManagerReconnectConcurrentSharesOneAttempt(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	fc.pingDelay = 20 * time.Millisecond // widen the race window
	m := NewManager(DefaultOptions(), fc)

	if _, err := m.Open(ctx, "c1", dbConfig("d")); err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	const n = 12
	var wg sync.WaitGroup
	errs := make([]error, n)
	pools := make([]*sql.DB, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = m.Reconnect(ctx, "c1", dbConfig("d"))
			pools[i], _ = m.Get("c1", "d")
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("Reconnect[%d] error = %v", i, errs[i])
		}
	}
	// One initial open + one shared replacement.
	if fc.opens.Load() != 2 {
		t.Errorf("connector opens = %d, want 2 (single shared reconnect)", fc.opens.Load())
	}
}

func TestManagerReconnectUnsupportedDriver(t *testing.T) {
	ctx := context.Background()
	m := NewManager(DefaultOptions()) // no connectors registered

	err := m.Reconnect(ctx, "c1", Config{Driver: model.DriverMySQL, Database: "d"})
	if !errors.Is(err, ErrUnsupportedDriver) {
		t.Errorf("Reconnect(unsupported) error = %v, want ErrUnsupportedDriver", err)
	}
}
