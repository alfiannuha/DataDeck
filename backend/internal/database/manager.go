package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// PoolKey identifies one managed pool. PRF-01: a PostgreSQL connection profile
// represents a server, so pools are per (connection, database). For MySQL and
// SQLite the database component is the profile's database/file, so the key
// degenerates to the previous connection-id behaviour.
//
// A PoolKey never carries credentials or DSN material, so it is safe to log.
type PoolKey struct {
	ConnectionID string
	Database     string
}

// String renders a credential-free key for logs and error messages.
func (k PoolKey) String() string { return k.ConnectionID + "/" + k.Database }

// Manager owns the active target database pools, keyed by (connection,
// database).
//
// Concurrent Open calls for the same key share a single in-flight attempt
// instead of opening duplicate pools. The number of cached pools per connection
// is bounded (Options.MaxPoolsPerConnection); the least-recently-used pool is
// evicted when the cap is exceeded.
type Manager struct {
	mu         sync.Mutex
	pools      map[PoolKey]*sql.DB
	lastUsed   map[PoolKey]time.Time
	opening    map[PoolKey]*openCall
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
	if opts.MaxPoolsPerConnection <= 0 {
		opts.MaxPoolsPerConnection = DefaultMaxPoolsPerConnection
	}
	registry := make(map[model.Driver]Connector, len(connectors))
	for _, connector := range connectors {
		registry[connector.Name()] = connector
	}
	return &Manager{
		pools:      make(map[PoolKey]*sql.DB),
		lastUsed:   make(map[PoolKey]time.Time),
		opening:    make(map[PoolKey]*openCall),
		connectors: registry,
		opts:       opts,
	}
}

// poolKey derives the (connection, database) identity for a request. The
// database comes from the resolved connection config; it never contains
// credentials.
func poolKey(connectionID string, cfg Config) PoolKey {
	return PoolKey{ConnectionID: connectionID, Database: cfg.Database}
}

// Open returns the active pool for (connection id, cfg.Database), opening and
// health-checking it on first use. Opening failures are not registered, so a
// later Open can retry.
func (m *Manager) Open(ctx context.Context, id string, cfg Config) (*sql.DB, error) {
	key := poolKey(id, cfg)

	m.mu.Lock()
	if db, ok := m.pools[key]; ok {
		m.lastUsed[key] = time.Now()
		m.mu.Unlock()
		return db, nil
	}
	if call, ok := m.opening[key]; ok {
		m.mu.Unlock()
		select {
		case <-call.done:
			m.mu.Lock()
			db, err := m.pools[key], call.err
			if ok := db != nil; ok {
				m.lastUsed[key] = time.Now()
			}
			m.mu.Unlock()
			return db, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &openCall{done: make(chan struct{})}
	m.opening[key] = call
	m.mu.Unlock()

	db, err := m.connect(ctx, cfg)

	m.mu.Lock()
	if err == nil {
		m.pools[key] = db
		m.lastUsed[key] = time.Now()
		m.evictOverCapLocked(id)
	}
	call.err = err
	delete(m.opening, key)
	close(call.done)
	m.mu.Unlock()

	return db, err
}

// evictOverCapLocked closes the least-recently-used pools of a connection until
// the per-connection cap is respected. Callers must hold m.mu.
func (m *Manager) evictOverCapLocked(connectionID string) {
	for {
		var (
			oldestKey PoolKey
			oldest    time.Time
			found     bool
			count     int
		)
		for key := range m.pools {
			if key.ConnectionID != connectionID {
				continue
			}
			count++
			used := m.lastUsed[key]
			if !found || used.Before(oldest) {
				oldestKey, oldest, found = key, used, true
			}
		}
		if count <= m.opts.MaxPoolsPerConnection || !found {
			return
		}
		db := m.pools[oldestKey]
		delete(m.pools, oldestKey)
		delete(m.lastUsed, oldestKey)
		if db != nil {
			_ = db.Close()
		}
	}
}

// Get returns an already-active pool for (id, database) without opening one.
func (m *Manager) Get(id, database string) (*sql.DB, bool) {
	key := PoolKey{ConnectionID: id, Database: database}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, ok := m.pools[key]
	if ok {
		m.lastUsed[key] = time.Now()
	}
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

// Introspect activates (or reuses) the pool for (id, cfg.Database) and reads
// the target database schema through the connector for cfg.Driver. A lost
// connection evicts the pool so the next call opens a fresh one.
func (m *Manager) Introspect(ctx context.Context, id string, cfg Config) ([]model.Database, error) {
	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return nil, err
	}
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	databases, err := connector.Introspect(ctx, db)
	if err != nil && errors.Is(err, ErrConnection) {
		m.evict(id, cfg)
	}
	return databases, err
}

// Capabilities returns the advertised capabilities for a driver.
func (m *Manager) Capabilities(driver model.Driver) (model.Capabilities, bool) {
	connector, ok := m.connectors[driver]
	if !ok {
		return model.Capabilities{}, false
	}
	return connector.Capabilities(), true
}

// ListDatabases discovers the databases selectable on a server-level
// connection (PRF-01). Only connectors implementing DatabaseLister support it
// (PostgreSQL); other drivers return ErrNotImplemented.
//
// Bootstrap resolution (ADR-009 §3): an explicit/default database wins; when
// the profile has none, the candidates are tried in order — "postgres", then a
// database named after the login user. Each candidate probe uses a temporary,
// unregistered pool that is closed immediately, so discovery never leaves a
// pool (or one pool per database) behind.
func (m *Manager) ListDatabases(ctx context.Context, id string, cfg Config) ([]model.DatabaseInfo, error) {
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	lister, ok := connector.(DatabaseLister)
	if !ok {
		return nil, fmt.Errorf("%w: database discovery is not implemented for %s", ErrNotImplemented, cfg.Driver)
	}

	var lastErr error
	for _, candidate := range bootstrapCandidates(cfg) {
		probe := cfg
		probe.Database = candidate
		db, err := m.connect(ctx, probe)
		if err != nil {
			lastErr = err
			continue
		}
		databases, err := lister.ListDatabases(ctx, db)
		_ = db.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return databases, nil
	}

	if lastErr == nil {
		lastErr = errors.New("no bootstrap database candidate")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("%w: %w", ErrNoBootstrapDatabase, lastErr)
	}
	return nil, lastErr
}

// bootstrapCandidates returns the databases to try for discovery. An explicit
// database is used as-is (no fallback). Otherwise the conventional maintenance
// database is tried first, followed by a database named after the login user.
func bootstrapCandidates(cfg Config) []string {
	if cfg.Database != "" {
		return []string{cfg.Database}
	}
	candidates := []string{"postgres"}
	if cfg.Username != "" && cfg.Username != "postgres" {
		candidates = append(candidates, cfg.Username)
	}
	return candidates
}

// Execute activates (or reuses) the pool for (id, cfg.Database) and runs one
// statement through the connector for cfg.Driver. A lost connection evicts the
// pool so the next call opens a fresh one.
func (m *Manager) Execute(ctx context.Context, id string, cfg Config, sqlText string) (model.QueryResult, error) {
	db, err := m.Open(ctx, id, cfg)
	if err != nil {
		return model.QueryResult{}, err
	}
	connector, ok := m.connectors[cfg.Driver]
	if !ok {
		return model.QueryResult{}, fmt.Errorf("%w: %s", ErrUnsupportedDriver, cfg.Driver)
	}
	result, err := connector.Execute(ctx, db, sqlText)
	if err != nil && errors.Is(err, ErrConnection) {
		m.evict(id, cfg)
	}
	return result, err
}

// evict removes and closes the pool for the given connection/database. It is
// used when a pool is known to be unhealthy (connection lost). Concurrent Open
// waiters may still hold the pointer; their next operation fails and retries via
// a fresh Open, which is the documented behaviour.
func (m *Manager) evict(id string, cfg Config) {
	key := poolKey(id, cfg)
	m.mu.Lock()
	db, ok := m.pools[key]
	if ok {
		delete(m.pools, key)
		delete(m.lastUsed, key)
	}
	m.mu.Unlock()
	if ok && db != nil {
		_ = db.Close()
	}
}

// Close closes and evicts the pool for (id, database), returning ErrNotFound if
// none is active.
func (m *Manager) Close(id, database string) error {
	key := PoolKey{ConnectionID: id, Database: database}
	m.mu.Lock()
	db, ok := m.pools[key]
	if ok {
		delete(m.pools, key)
		delete(m.lastUsed, key)
	}
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, key.String())
	}
	return db.Close()
}

// CloseConnection closes and evicts every pool belonging to a connection
// profile. It is used when a profile is deleted or its credentials or
// configuration change, so the next operation reconnects with fresh settings.
// It returns nil when the connection has no active pools.
func (m *Manager) CloseConnection(id string) error {
	m.mu.Lock()
	var closing []*sql.DB
	for key, db := range m.pools {
		if key.ConnectionID != id {
			continue
		}
		closing = append(closing, db)
		delete(m.pools, key)
		delete(m.lastUsed, key)
	}
	m.mu.Unlock()

	var errs []error
	for _, db := range closing {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// CloseAll closes every active pool. Safe to call on the shutdown path and when
// no pools are open.
func (m *Manager) CloseAll() error {
	m.mu.Lock()
	pools := m.pools
	m.pools = make(map[PoolKey]*sql.DB)
	m.lastUsed = make(map[PoolKey]time.Time)
	m.mu.Unlock()

	var errs []error
	for key, db := range pools {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", key.String(), err))
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
