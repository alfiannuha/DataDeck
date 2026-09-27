package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// Manager owns the active target database pools, keyed by connection id.
//
// A connection id maps to at most one pool. Concurrent Open calls for the same
// id share a single in-flight attempt instead of opening duplicate pools.
type Manager struct {
	mu         sync.Mutex
	pools      map[string]*sql.DB
	opening    map[string]*openCall
	connectors map[model.Driver]Connector
	opts       Options
}

type openCall struct {
	done chan struct{}
	err  error
}

// NewManager builds a manager. A zero Options value falls back to
// DefaultOptions. Connectors are registered by their driver name.
func NewManager(opts Options, connectors ...Connector) *Manager {
	if opts == (Options{}) {
		opts = DefaultOptions()
	}
	registry := make(map[model.Driver]Connector, len(connectors))
	for _, connector := range connectors {
		registry[connector.Name()] = connector
	}
	return &Manager{
		pools:      make(map[string]*sql.DB),
		opening:    make(map[string]*openCall),
		connectors: registry,
		opts:       opts,
	}
}

// Open returns the active pool for id, opening and health-checking it on first
// use. Opening failures are not registered, so a later Open can retry.
func (m *Manager) Open(ctx context.Context, id string, cfg Config) (*sql.DB, error) {
	m.mu.Lock()
	if db, ok := m.pools[id]; ok {
		m.mu.Unlock()
		return db, nil
	}
	if call, ok := m.opening[id]; ok {
		m.mu.Unlock()
		select {
		case <-call.done:
			m.mu.Lock()
			db, err := m.pools[id], call.err
			m.mu.Unlock()
			return db, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &openCall{done: make(chan struct{})}
	m.opening[id] = call
	m.mu.Unlock()

	db, err := m.connect(ctx, cfg)

	m.mu.Lock()
	if err == nil {
		m.pools[id] = db
	}
	call.err = err
	delete(m.opening, id)
	close(call.done)
	m.mu.Unlock()

	return db, err
}

// Get returns an already-active pool without opening one.
func (m *Manager) Get(id string) (*sql.DB, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	db, ok := m.pools[id]
	return db, ok
}

// Test opens, health-checks and closes a temporary pool. It never registers the
// pool, so it is safe for validating parameters before saving a profile.
func (m *Manager) Test(ctx context.Context, cfg Config) error {
	db, err := m.connect(ctx, cfg)
	if err != nil {
		return err
	}
	return db.Close()
}

// Introspect activates (or reuses) the pool for id and reads the target
// database schema through the connector for cfg.Driver.
func (m *Manager) Introspect(ctx context.Context, id string, cfg Config) ([]model.Database, error) {
	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return nil, err
	}
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	return connector.Introspect(ctx, db)
}

// Capabilities returns the advertised capabilities for a driver.
func (m *Manager) Capabilities(driver model.Driver) (model.Capabilities, bool) {
	connector, ok := m.connectors[driver]
	if !ok {
		return model.Capabilities{}, false
	}
	return connector.Capabilities(), true
}

// Execute activates (or reuses) the pool for id and runs one statement through
// the connector for cfg.Driver.
func (m *Manager) Execute(ctx context.Context, id string, cfg Config, sqlText string) (model.QueryResult, error) {
	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return model.QueryResult{}, err
	}
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return model.QueryResult{}, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	return connector.Execute(ctx, db, sqlText)
}

// Close closes and evicts the pool for id, returning ErrNotFound if none is
// active.
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	db, ok := m.pools[id]
	if ok {
		delete(m.pools, id)
	}
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return db.Close()
}

// CloseAll closes every active pool. Safe to call on the shutdown path and when
// no pools are open.
func (m *Manager) CloseAll() error {
	m.mu.Lock()
	pools := m.pools
	m.pools = make(map[string]*sql.DB)
	m.mu.Unlock()

	var errs []error
	for id, db := range pools {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close connection %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) connect(ctx context.Context, cfg Config) (*sql.DB, error) {
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}

	db, err := connector.Open(ctx, cfg, m.opts)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnection, err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, m.opts.PingTimeout)
	defer cancel()
	if err := connector.Ping(pingCtx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%w: health check failed: %w", ErrConnection, err)
	}
	return db, nil
}
