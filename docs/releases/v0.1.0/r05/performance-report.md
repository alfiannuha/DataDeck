# DataDeck v0.1.0 — R05 Performance Report

Environment: macOS 26.6.2 arm64, Go 1.26.9 toolchain, `CGO_ENABLED=0` embedded
single binary, PostgreSQL 17.11. Method: single host; startup measured by polling
`/health`; RSS via `ps -o rss`. Not directly comparable to the M6 baseline host.

| Metric | M6 baseline (ref) | PRF-02/R05 | Note |
|---|---|---|---|
| Binary size | ~15.8 MB | **~32 MB** | +excelize, `modernc.org/sqlite`, pgx/mysql drivers, embedded UI |
| Startup → health | ~126 ms | **~312 ms** (10 ms poll) / ~430 ms (50 ms poll) | methodology differs; no functional impact |
| Idle RSS (settled) | ~20.98 MB | **~4.5 MB** (`ps rss`) | lower; baseline metric differs |
| RSS just after start | — | ~24.8 MB | pre-GC peak; settles as above |

## Backend benchmarks (`go test -bench -benchmem`)
| Case | ns/op | B/op |
|---|---|---|
| Execute result 1,000 rows | 3.77 ms | 223 KB |
| Execute result 100,000 rows | 257 ms | 31.2 MB |
| Introspect 1,000 tables | 193 ms | 4.93 MB |
| Concurrent queries | 264 µs/op | 14 KB |

Raw-query results are bounded by the existing 50 MB cap; structured Table Data
browsing is server-side paginated (one page in memory).

## Boundaries verified
- Table export streams via a DB cursor to a temp file; the 1,000-row full-table
  export succeeds despite a 100-row page (E2E).
- Table import spools to a temp file (64 MiB body cap); DB restore artifact cap 256 MiB.
- Delete/reconnect run against disposable datasets only; no resource growth observed
  across the E2E suite.

## NOT RUN
1,000,000-row table-data benchmark; 100k-row import/export throughput + cancellation
latency; reconnect soak (goroutine/fd/RSS curves). Documented residual — paging and
streaming are bounded by design.
