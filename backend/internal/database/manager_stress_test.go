package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// tagConnector is a controllable Connector whose Execute result identifies the
// connector, so routing across connection ids can be asserted.
type tagConnector struct {
	name    model.Driver
	tag     string
	openErr error
	opens   atomic.Int64
}

func (c *tagConnector) Name() model.Driver { return c.name }

func (c *tagConnector) Open(_ context.Context, _ Config, opts Options) (*sql.DB, error) {
	c.opens.Add(1)
	if c.openErr != nil {
		return nil, c.openErr
	}
	db := sql.OpenDB(fakeSQLConnector{d: fakeSQLDriver{}})
	applyPoolOptions(db, opts)
	return db, nil
}

func (c *tagConnector) Ping(ctx context.Context, db *sql.DB) error { return db.PingContext(ctx) }

func (c *tagConnector) Introspect(context.Context, *sql.DB) ([]model.Database, error) {
	return nil, nil
}

func (c *tagConnector) Execute(context.Context, *sql.DB, string) (model.QueryResult, error) {
	return model.QueryResult{Columns: []model.QueryColumn{{Name: c.tag}}}, nil
}

func (c *tagConnector) Capabilities() model.Capabilities {
	return model.Capabilities{Dialect: string(c.name)}
}

func stressConfig(driver model.Driver) Config {
	return Config{Driver: driver, Host: "h", Port: 1, Database: "d"}
}

// TestManagerConcurrentLifecycle hammers Open/Get/Test/Close/CloseAll from many
// goroutines. Run with -race; the invariant is "no data race, no panic, and no
// pool left registered after a final CloseAll".
func TestManagerConcurrentLifecycle(t *testing.T) {
	ctx := context.Background()
	pg := &tagConnector{name: model.DriverPostgres, tag: "pg"}
	my := &tagConnector{name: model.DriverMySQL, tag: "my"}
	m := NewManager(DefaultOptions(), pg, my)

	ids := []string{"a", "b", "c"}
	drivers := map[string]model.Driver{"a": model.DriverPostgres, "b": model.DriverMySQL, "c": model.DriverPostgres}

	const workers = 12
	const iterations = 60
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for i := 0; i < iterations; i++ {
				id := ids[rng.Intn(len(ids))]
				switch rng.Intn(5) {
				case 0:
					_, _ = m.Open(ctx, id, stressConfig(drivers[id]))
				case 1:
					_, _ = m.Get(id)
				case 2:
					_ = m.Test(ctx, stressConfig(drivers[id]))
				case 3:
					_ = m.Close(id)
				case 4:
					if rng.Intn(20) == 0 {
						_ = m.CloseAll()
					}
				}
			}
		}(int64(w))
	}
	wg.Wait()

	if err := m.CloseAll(); err != nil {
		t.Fatalf("CloseAll() error = %v", err)
	}
	for _, id := range ids {
		if _, ok := m.Get(id); ok {
			t.Errorf("pool %q still registered after CloseAll", id)
		}
	}
}

// TestConcurrentFailingOpensAllReturn ensures a failing Open wakes every waiter
// (no lost reference, no stuck goroutine) and registers nothing.
func TestConcurrentFailingOpensAllReturn(t *testing.T) {
	ctx := context.Background()
	fc := &tagConnector{name: model.DriverPostgres, tag: "pg", openErr: errors.New("dial failed")}
	m := NewManager(DefaultOptions(), fc)

	const n = 32
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = m.Open(ctx, "same", stressConfig(model.DriverPostgres))
		}(i)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent failing Opens did not all return")
	}

	for i, err := range errs {
		if err == nil {
			t.Fatalf("Open[%d] error = nil, want failure", i)
		}
	}
	if _, ok := m.Get("same"); ok {
		t.Error("failed Open left a pool registered")
	}
}

// TestConcurrentOpensShareSinglePool widens the open window and asserts exactly
// one pool is created for simultaneous callers.
func TestConcurrentOpensShareSinglePool(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	fc.pingDelay = 30 * time.Millisecond
	m := NewManager(DefaultOptions(), fc)

	const n = 24
	pools := make([]*sql.DB, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pools[i], _ = m.Open(ctx, "dup", stressConfig(model.DriverPostgres))
		}(i)
	}
	wg.Wait()

	for i := range pools {
		if pools[i] == nil {
			t.Fatalf("Open[%d] returned nil", i)
		}
		if pools[i] != pools[0] {
			t.Fatalf("Open[%d] returned a different pool", i)
		}
	}
	if opens := fc.opens.Load(); opens != 1 {
		t.Fatalf("connector opens = %d, want 1", opens)
	}
}

// TestCrossConnectionExecutionIsolation runs queries for two connection ids in
// parallel and asserts each is routed to its own connector/driver.
func TestCrossConnectionExecutionIsolation(t *testing.T) {
	ctx := context.Background()
	pg := &tagConnector{name: model.DriverPostgres, tag: "pg"}
	my := &tagConnector{name: model.DriverMySQL, tag: "my"}
	m := NewManager(DefaultOptions(), pg, my)

	for id, driver := range map[string]model.Driver{"pg": model.DriverPostgres, "my": model.DriverMySQL} {
		if _, err := m.Open(ctx, id, stressConfig(driver)); err != nil {
			t.Fatalf("Open(%s) error = %v", id, err)
		}
	}

	const n = 60
	var wg sync.WaitGroup
	failures := make(chan string, n)
	for i := 0; i < n; i++ {
		id, driver, want := "pg", model.DriverPostgres, "pg"
		if i%2 == 1 {
			id, driver, want = "my", model.DriverMySQL, "my"
		}
		wg.Add(1)
		go func(id string, driver model.Driver, want string) {
			defer wg.Done()
			result, err := m.Execute(ctx, id, stressConfig(driver), "SELECT 1")
			if err != nil {
				failures <- fmt.Sprintf("%s: %v", id, err)
				return
			}
			if len(result.Columns) != 1 || result.Columns[0].Name != want {
				failures <- fmt.Sprintf("%s routed to %q, want %q", id, result.Columns[0].Name, want)
			}
		}(id, driver, want)
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}

// TestPoolLimitsConstrainFakePool verifies the manager applies pool sizing to
// every opened pool (no accidental unlimited default).
func TestPoolLimitsConstrainFakePool(t *testing.T) {
	ctx := context.Background()
	fc := &tagConnector{name: model.DriverPostgres, tag: "pg"}
	m := NewManager(Options{MaxOpenConns: 3, MaxIdleConns: 1, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Minute, PingTimeout: time.Second}, fc)

	db, err := m.Open(ctx, "a", stressConfig(model.DriverPostgres))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got := db.Stats().MaxOpenConnections; got != 3 {
		t.Fatalf("MaxOpenConnections = %d, want 3", got)
	}
}
