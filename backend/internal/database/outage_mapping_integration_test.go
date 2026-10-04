//go:build integration

package database_test

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/database"
)

// tcpProxy forwards TCP to a real database so connection loss can be simulated
// deterministically (accepts, then cuts all connections).
type tcpProxy struct {
	ln    net.Listener
	mu    sync.Mutex
	conns []net.Conn
}

func newTCPProxy(t *testing.T, target string) *tcpProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("proxy listen: %v", err)
	}
	p := &tcpProxy{ln: ln}
	go func() {
		for {
			down, err := ln.Accept()
			if err != nil {
				return
			}
			up, err := net.Dial("tcp", target)
			if err != nil {
				_ = down.Close()
				continue
			}
			p.mu.Lock()
			p.conns = append(p.conns, down, up)
			p.mu.Unlock()
			go func() { _, _ = io.Copy(up, down) }()
			go func() { _, _ = io.Copy(down, up) }()
		}
	}()
	return p
}

func (p *tcpProxy) hostPort() (string, int) {
	host, portStr, _ := net.SplitHostPort(p.ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return host, port
}

func (p *tcpProxy) cut() {
	_ = p.ln.Close()
	p.mu.Lock()
	for _, c := range p.conns {
		_ = c.Close()
	}
	p.conns = nil
	p.mu.Unlock()
}

func assertSQLStillSQLError(t *testing.T, manager *database.Manager, id string, cfg database.Config) {
	t.Helper()
	ctx := context.Background()
	_, err := manager.Execute(ctx, id, cfg, `SELECT WHERRE`)
	if err == nil {
		t.Fatal("expected a syntax error")
	}
	var sqlErr *database.SQLError
	if !errors.As(err, &sqlErr) {
		t.Fatalf("syntax error = %T (%v), want *database.SQLError", err, err)
	}
	if errors.Is(err, database.ErrConnection) {
		t.Fatalf("syntax error was misclassified as a connection error: %v", err)
	}
}

func assertConnectionLossMapped(t *testing.T, manager *database.Manager, id string, cfg database.Config) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, err := manager.Execute(ctx, id, cfg, `SELECT 1`)
		if err == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if errors.Is(err, database.ErrConnection) {
			return
		}
		t.Fatalf("connection loss error = %v (%T), want ErrConnection", err, err)
	}
	t.Fatal("connection loss was never mapped to ErrConnection")
}

func TestPostgresConnectionLossMapped(t *testing.T) {
	base := pgConfig(t)
	proxy := newTCPProxy(t, net.JoinHostPort(base.Host, strconv.Itoa(base.Port)))
	host, port := proxy.hostPort()
	cfg := base
	cfg.Host, cfg.Port = host, port

	manager := database.NewManager(database.DefaultOptions(), database.Postgres{})
	if _, err := manager.Open(context.Background(), "pg", cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = manager.CloseAll() }()
	if _, err := manager.Execute(context.Background(), "pg", cfg, `SELECT 1`); err != nil {
		t.Fatalf("baseline execute: %v", err)
	}
	assertSQLStillSQLError(t, manager, "pg", cfg)

	proxy.cut()
	assertConnectionLossMapped(t, manager, "pg", cfg)
}

func TestMySQLConnectionLossMapped(t *testing.T) {
	base := mysqlConfig(t)
	proxy := newTCPProxy(t, net.JoinHostPort(base.Host, strconv.Itoa(base.Port)))
	host, port := proxy.hostPort()
	cfg := base
	cfg.Host, cfg.Port = host, port

	manager := database.NewManager(database.DefaultOptions(), database.MySQL{})
	if _, err := manager.Open(context.Background(), "my", cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = manager.CloseAll() }()
	if _, err := manager.Execute(context.Background(), "my", cfg, `SELECT 1`); err != nil {
		t.Fatalf("baseline execute: %v", err)
	}
	assertSQLStillSQLError(t, manager, "my", cfg)

	proxy.cut()
	assertConnectionLossMapped(t, manager, "my", cfg)
}

// TestPostgresDroppedDatabaseAfterPoolClassified reproduces the stale-pool case:
// a pool is opened for a database that is then dropped. The next execution must
// still be classified as ErrDatabaseNotFound (not an internal error).
func TestPostgresDroppedDatabaseAfterPoolClassified(t *testing.T) {
	cfg := pgConfig(t)
	const dbName = "datadeck_drop_race"
	ctx := context.Background()

	manager := database.NewManager(database.DefaultOptions(), database.Postgres{})
	defer func() { _ = manager.CloseAll() }()
	admin := adminDB(t, manager, cfg)
	t.Cleanup(func() { _ = dbExec(admin, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)") })

	_ = dbExec(admin, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
	mustExec(t, admin, "CREATE DATABASE "+dbName)

	target := cfg
	target.Database = dbName
	if _, err := manager.Execute(ctx, "drop-race", target, `SELECT 1`); err != nil {
		t.Fatalf("baseline execute: %v", err)
	}

	mustExec(t, admin, "DROP DATABASE "+dbName+" WITH (FORCE)")
	if _, err := manager.Execute(ctx, "drop-race", target, `SELECT 1`); !errors.Is(err, database.ErrDatabaseNotFound) {
		t.Fatalf("error = %v (%T), want ErrDatabaseNotFound", err, err)
	}
}
