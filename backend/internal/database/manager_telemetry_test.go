package database

import (
	"context"
	"testing"
	"time"
)

// TestPoolStatusesReportsWithoutOpening verifies telemetry is side-effect free
// (never opens a pool), reports identity/driver, and is sorted.
func TestPoolStatusesReportsWithoutOpening(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	// No pools yet: status is empty and must not trigger an open.
	if got := m.PoolStatuses(); len(got) != 0 {
		t.Fatalf("PoolStatuses() before open = %+v, want empty", got)
	}
	if fc.opens.Load() != 0 {
		t.Fatalf("PoolStatuses opened %d pools, want 0", fc.opens.Load())
	}

	if _, err := m.Open(ctx, "c1", dbConfig("alpha")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Open(ctx, "c1", dbConfig("beta")); err != nil {
		t.Fatal(err)
	}
	statuses := m.PoolStatuses()
	if len(statuses) != 2 {
		t.Fatalf("PoolStatuses len = %d, want 2", len(statuses))
	}
	// Sorted by (connectionID, database).
	if statuses[0].Database != "alpha" || statuses[1].Database != "beta" {
		t.Errorf("statuses not sorted: %q, %q", statuses[0].Database, statuses[1].Database)
	}
	for _, s := range statuses {
		if s.ConnectionID != "c1" || s.Driver != "postgres" || s.State != "ready" {
			t.Errorf("unexpected status: %+v", s)
		}
	}
	if fc.opens.Load() != 2 {
		t.Errorf("connector opens = %d, want 2 (telemetry must not open pools)", fc.opens.Load())
	}
}

func TestPoolStatusesIncludesInitializing(t *testing.T) {
	m := NewManager(DefaultOptions())
	key := PoolKey{ConnectionID: "c1", Database: "alpha"}
	m.mu.Lock()
	m.opening[key] = &openCall{done: make(chan struct{})}
	m.mu.Unlock()

	statuses := m.PoolStatuses()
	if len(statuses) != 1 || statuses[0].State != "initializing" {
		t.Fatalf("statuses = %+v, want one initializing entry", statuses)
	}
}

// TestSweepIdleClosesIdlePoolAndIsIdempotent verifies idle pools are closed and a
// second sweep closes nothing.
func TestSweepIdleClosesIdlePoolAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	db, err := m.Open(ctx, "c1", dbConfig("d"))
	if err != nil {
		t.Fatal(err)
	}
	// Force it to look long-idle.
	m.mu.Lock()
	m.lastUsed[PoolKey{ConnectionID: "c1", Database: "d"}] = time.Now().Add(-time.Hour)
	m.mu.Unlock()

	if n := m.SweepIdle(30 * time.Minute); n != 1 {
		t.Fatalf("first SweepIdle = %d, want 1", n)
	}
	if _, ok := m.Get("c1", "d"); ok {
		t.Error("idle pool still registered after sweep")
	}
	if err := db.Ping(); err == nil {
		t.Error("pool still usable after sweep")
	}
	if n := m.SweepIdle(30 * time.Minute); n != 0 {
		t.Errorf("second SweepIdle = %d, want 0 (idempotent)", n)
	}
}

// TestSweepIdleSkipsActivePool verifies a pool with an active connection (InUse>0)
// is never closed, and is closeable once released.
func TestSweepIdleSkipsActivePool(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	db, err := m.Open(ctx, "c1", dbConfig("d"))
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.lastUsed[PoolKey{ConnectionID: "c1", Database: "d"}] = time.Now().Add(-time.Hour)
	m.mu.Unlock()

	conn, err := db.Conn(ctx) // holds an in-use connection
	if err != nil {
		t.Fatal(err)
	}
	if n := m.SweepIdle(30 * time.Minute); n != 0 {
		t.Fatalf("SweepIdle closed %d pool(s) while a connection was active, want 0", n)
	}
	if len(m.PoolStatuses()) != 1 {
		t.Fatal("active pool was removed")
	}
	_ = conn.Close() // release the lease; connection returns to the pool (idle)
	deadline := time.Now().Add(2 * time.Second)
	for db.Stats().InUse != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	// Keep the pool long-idle (PoolStatuses/Get must not refresh lastUsed here).
	m.mu.Lock()
	m.lastUsed[PoolKey{ConnectionID: "c1", Database: "d"}] = time.Now().Add(-time.Hour)
	m.mu.Unlock()
	if n := m.SweepIdle(30 * time.Minute); n != 1 {
		t.Fatalf("SweepIdle after release = %d, want 1 (InUse=%d)", n, db.Stats().InUse)
	}
}

// TestSweepIdleSkipsInitializing verifies a pool that is initializing is not swept.
func TestSweepIdleSkipsInitializing(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	db, err := m.Open(ctx, "c1", dbConfig("d"))
	if err != nil {
		t.Fatal(err)
	}
	key := PoolKey{ConnectionID: "c1", Database: "d"}
	m.mu.Lock()
	m.lastUsed[key] = time.Now().Add(-time.Hour)
	m.opening[key] = &openCall{done: make(chan struct{})} // pretend a reconnect is in flight
	m.mu.Unlock()

	if n := m.SweepIdle(30 * time.Minute); n != 0 {
		t.Fatalf("SweepIdle closed %d while initializing, want 0", n)
	}
	_ = db.Ping()
}

func TestSweepIdleZeroThresholdDisables(t *testing.T) {
	m := NewManager(DefaultOptions())
	if n := m.SweepIdle(0); n != 0 {
		t.Errorf("SweepIdle(0) = %d, want 0", n)
	}
}
