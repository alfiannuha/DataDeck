package database

import (
	"strings"
	"testing"
)

// TestResultBufferStopsAccumulation verifies the guard halts accumulation while
// the running payload stays within the budget (plus at most one row), rather
// than collecting everything and trimming afterwards.
func TestResultBufferStopsAccumulation(t *testing.T) {
	const budget = 4096
	buffer := newResultBuffer(budget)

	added := 0
	for i := 0; i < 10_000; i++ {
		row := []any{strings.Repeat("x", 128)}
		if !buffer.add(row) {
			break
		}
		added++
		if buffer.size > budget {
			t.Fatalf("buffer size %d exceeded budget %d", buffer.size, budget)
		}
	}

	if added == 0 {
		t.Fatal("no rows were accepted")
	}
	if added >= 10_000 {
		t.Fatal("buffer accepted every row; the guard did not trigger")
	}
	// The tracked payload size never exceeds the budget.
	if buffer.size > budget {
		t.Fatalf("buffer size %d exceeded budget %d", buffer.size, budget)
	}
}

func TestResultBufferRejectsOversizedRow(t *testing.T) {
	buffer := newResultBuffer(64)
	if buffer.add([]any{strings.Repeat("y", 4096)}) {
		t.Fatal("oversized single row was accepted")
	}
	if len(buffer.rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(buffer.rows))
	}
}

func TestResultBufferHandlesUnmarshalableRow(t *testing.T) {
	buffer := newResultBuffer(1 << 20)
	// Channels cannot be marshaled; the buffer must fall back without panicking
	// and must not grow beyond the budget.
	if !buffer.add([]any{make(chan int)}) {
		t.Fatal("unmarshalable row was unexpectedly rejected under a large budget")
	}
	if buffer.size <= 0 {
		t.Fatalf("size = %d, want a positive fallback estimate", buffer.size)
	}
	if buffer.size > buffer.max {
		t.Fatalf("size = %d exceeded budget %d", buffer.size, buffer.max)
	}
}
