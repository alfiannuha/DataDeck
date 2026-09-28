package database

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// fakeConn is a minimal database/sql driver connection whose Ping behavior can
// be controlled. It lets manager behavior be tested without a real database.
type fakeConn struct {
	pingErr   error
	pingDelay time.Duration
}

func (c *fakeConn) Prepare(string) (sqldriver.Stmt, error) {
	return nil, errors.New("fake driver: prepare unsupported")
}
func (c *fakeConn) Close() error { return nil }
func (c *fakeConn) Begin() (sqldriver.Tx, error) {
	return nil, errors.New("fake driver: begin unsupported")
}
func (c *fakeConn) Ping(ctx context.Context) error {
	if c.pingDelay > 0 {
		select {
		case <-time.After(c.pingDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return c.pingErr
}

type fakeSQLDriver struct {
	pingErr   error
	pingDelay time.Duration
}

func (d fakeSQLDriver) Open(string) (sqldriver.Conn, error) {
	return &fakeConn{pingErr: d.pingErr, pingDelay: d.pingDelay}, nil
}

type fakeSQLConnector struct{ d fakeSQLDriver }

func (c fakeSQLConnector) Connect(context.Context) (sqldriver.Conn, error) { return c.d.Open("") }
func (c fakeSQLConnector) Driver() sqldriver.Driver                        { return c.d }

// fakeConnector implements Connector for manager tests.
type fakeConnector struct {
	name       model.Driver
	openErr    error
	pingErr    error
	pingDelay  time.Duration
	executeErr error
	opens      atomic.Int64
}

func newFakeConnector() *fakeConnector {
	return &fakeConnector{name: model.DriverPostgres}
}

func (c *fakeConnector) Name() model.Driver { return c.name }

func (c *fakeConnector) Open(_ context.Context, _ Config, _ Options) (*sql.DB, error) {
	c.opens.Add(1)
	if c.openErr != nil {
		return nil, c.openErr
	}
	return sql.OpenDB(fakeSQLConnector{d: fakeSQLDriver{pingErr: c.pingErr, pingDelay: c.pingDelay}}), nil
}

func (c *fakeConnector) Ping(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
}

func (c *fakeConnector) Introspect(context.Context, *sql.DB) ([]model.Database, error) {
	return nil, nil
}

func (c *fakeConnector) Execute(context.Context, *sql.DB, string) (model.QueryResult, error) {
	if c.executeErr != nil {
		return model.QueryResult{}, c.executeErr
	}
	return model.QueryResult{}, nil
}

func (c *fakeConnector) Capabilities() model.Capabilities {
	return model.Capabilities{Dialect: string(c.name)}
}

func testConfig() Config {
	return Config{Driver: model.DriverPostgres, Host: "h", Port: 1, Database: "d"}
}

func TestManagerOpenAndGet(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	db, err := m.Open(ctx, "c1", testConfig())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if db == nil {
		t.Fatal("Open() returned nil pool")
	}
	got, ok := m.Get("c1", "d")
	if !ok || got != db {
		t.Errorf("Get(c1) = %v, %v; want the opened pool", got, ok)
	}
	if fc.opens.Load() != 1 {
		t.Errorf("connector opens = %d, want 1", fc.opens.Load())
	}
}

func TestManagerAvoidsDuplicatePools(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	first, err := m.Open(ctx, "c1", testConfig())
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	second, err := m.Open(ctx, "c1", testConfig())
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	if first != second {
		t.Error("duplicate Open returned a different pool")
	}
	if fc.opens.Load() != 1 {
		t.Errorf("connector opens = %d, want 1 (no duplicate pool)", fc.opens.Load())
	}
}

func TestManagerConcurrentOpenSharesOnePool(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	fc.pingDelay = 20 * time.Millisecond // widen the race window
	m := NewManager(DefaultOptions(), fc)

	const n = 16
	var wg sync.WaitGroup
	pools := make([]*sql.DB, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pools[i], errs[i] = m.Open(ctx, "c1", testConfig())
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("Open[%d] error = %v", i, errs[i])
		}
		if pools[i] != pools[0] {
			t.Fatalf("Open[%d] returned a different pool", i)
		}
	}
	if fc.opens.Load() != 1 {
		t.Errorf("connector opens = %d, want 1 (single shared open)", fc.opens.Load())
	}
}

func TestManagerFailedOpenIsNotRegistered(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	fc.openErr = errors.New("dial failed")
	m := NewManager(DefaultOptions(), fc)

	if _, err := m.Open(ctx, "c1", testConfig()); err == nil {
		t.Fatal("Open() error = nil, want failure")
	}
	if _, ok := m.Get("c1", "d"); ok {
		t.Error("failed open left a pool registered")
	}
}

func TestManagerPingFailureIsNotRegistered(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	fc.pingErr = errors.New("connection refused")
	m := NewManager(DefaultOptions(), fc)

	if _, err := m.Open(ctx, "c1", testConfig()); err == nil {
		t.Fatal("Open() error = nil, want ping failure")
	}
	if _, ok := m.Get("c1", "d"); ok {
		t.Error("failed health check left a pool registered")
	}
}

func TestManagerPingTimeout(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	fc.pingDelay = time.Second
	m := NewManager(Options{PingTimeout: 50 * time.Millisecond}, fc)

	start := time.Now()
	_, err := m.Open(ctx, "c1", testConfig())
	if err == nil {
		t.Fatal("Open() error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Open() error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("Open() took %s, health check did not honor the timeout", elapsed)
	}
	if _, ok := m.Get("c1", "d"); ok {
		t.Error("timed-out connection left a pool registered")
	}
}

func TestManagerTestDoesNotRegister(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	if err := m.Test(ctx, testConfig()); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if _, ok := m.Get("c1", "d"); ok {
		t.Error("Test() registered a pool")
	}

	fc.pingErr = errors.New("refused")
	if err := m.Test(ctx, testConfig()); err == nil {
		t.Error("Test() error = nil, want failure")
	}
}

func TestManagerClose(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	db, err := m.Open(ctx, "c1", testConfig())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := m.Close("c1", "d"); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, ok := m.Get("c1", "d"); ok {
		t.Error("Get() after Close returned a pool")
	}
	if err := db.Ping(); err == nil {
		t.Error("pool still usable after Close")
	}
	if err := m.Close("unknown", "d"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Close(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestManagerCloseAll(t *testing.T) {
	ctx := context.Background()
	fc := newFakeConnector()
	m := NewManager(DefaultOptions(), fc)

	if _, err := m.Open(ctx, "c1", testConfig()); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Open(ctx, "c2", testConfig()); err != nil {
		t.Fatal(err)
	}
	if err := m.CloseAll(); err != nil {
		t.Fatalf("CloseAll() error = %v", err)
	}
	for _, id := range []string{"c1", "c2"} {
		if _, ok := m.Get(id, "d"); ok {
			t.Errorf("Get(%s) after CloseAll returned a pool", id)
		}
	}
	if err := m.CloseAll(); err != nil {
		t.Errorf("second CloseAll() error = %v, want nil", err)
	}
}

func TestManagerUnsupportedDriver(t *testing.T) {
	ctx := context.Background()
	m := NewManager(DefaultOptions()) // no connectors registered

	_, err := m.Open(ctx, "c1", Config{Driver: model.DriverMySQL})
	if !errors.Is(err, ErrUnsupportedDriver) {
		t.Errorf("Open(unsupported) error = %v, want ErrUnsupportedDriver", err)
	}
}

func TestManagerGetUnknown(t *testing.T) {
	m := NewManager(DefaultOptions())
	if _, ok := m.Get("nope", "d"); ok {
		t.Error("Get(unknown) = true, want false")
	}
}
