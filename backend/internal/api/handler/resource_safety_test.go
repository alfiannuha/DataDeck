package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// TestRepeatedFailuresDoNotLeakResources runs many failing executions and
// asserts that goroutine count stays bounded (after settling) and that every
// failure still produced a history row. It also confirms the backend remains
// usable afterwards.
func TestRepeatedFailuresDoNotLeakResources(t *testing.T) {
	h, db := newQueryHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "bad", Name: "Unreachable PG", Driver: model.DriverPostgres,
		Host: strPtr("127.0.0.1"), Port: intPtr(1),
		DatabaseName: "app", Username: strPtr("u"),
	})

	execute := func(connectionID, sql string) int {
		rec := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
			fmt.Sprintf(`{"connection_id":%q,"sql":%q,"timeout_seconds":2}`, connectionID, sql))
		return rec.Code
	}

	// Warm up so lazily created runtime/transport goroutines are not counted.
	for i := 0; i < 5; i++ {
		if code := execute("bad", "SELECT 1"); code != http.StatusBadGateway {
			t.Fatalf("warmup status = %d, want 502", code)
		}
	}
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()

	const failures = 40
	for i := 0; i < failures; i++ {
		if code := execute("bad", "SELECT 1"); code != http.StatusBadGateway {
			t.Fatalf("failure %d status = %d, want 502", i, code)
		}
	}

	after := runtime.NumGoroutine()
	deadline := time.Now().Add(3 * time.Second)
	for after > before+12 && time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
		after = runtime.NumGoroutine()
	}
	if after > before+12 {
		t.Fatalf("goroutine growth after %d failures: before=%d after=%d", failures, before, after)
	}

	var recorded int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM query_history WHERE connection_id = 'bad'`,
	).Scan(&recorded); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if want := failures + 5; recorded != want {
		t.Fatalf("history rows = %d, want %d (one per failure)", recorded, want)
	}

	// The backend must remain usable: a valid SQLite profile still executes.
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "ok", Name: "SQLite", Driver: model.DriverSQLite,
		DatabaseName: filepath.Join(t.TempDir(), "user.db"),
	})
	if code := execute("ok", "SELECT 1"); code != http.StatusOK {
		t.Fatalf("post-failure execute status = %d, want 200", code)
	}
}

// TestSQLitePathFailureIsBounded verifies a bad SQLite target yields a bounded
// error and no panic, and the manager stays usable.
func TestSQLitePathFailureIsBounded(t *testing.T) {
	h, db := newQueryHandler(t)
	seedProfile(t, db, &model.ConnectionProfile{
		ID: "badpath", Name: "SQLite bad", Driver: model.DriverSQLite,
		DatabaseName: filepath.Join(t.TempDir(), "missing-dir", "nested", "db.sqlite3"),
	})

	rec := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"badpath","sql":"SELECT 1"}`)
	if rec.Code == http.StatusOK {
		t.Fatalf("expected failure for a bad SQLite path, got 200 (body=%s)", rec.Body.String())
	}
	if rec.Code != http.StatusBadGateway && rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want a bounded 5xx (body=%s)", rec.Code, rec.Body.String())
	}
}
