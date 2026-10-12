# PRF02-T19 — Performance & Resource Results

Environment: macOS 26.6.2 arm64, Go 1.26.9 toolchain, single embedded binary,
PostgreSQL 17.11. Measurements are from this host only; they are **not** directly
comparable to the M6 baseline environment.

## 1. Binary & startup vs M6 baseline
| Metric | M6 baseline (historical) | PRF-02 (this run) | Note |
|---|---|---|---|
| Single binary size | ~15.8 MB | **~32 MB** | +excelize (T14), +`modernc.org/sqlite`, pgx + mysql drivers |
| Startup (−>health) | ~126 ms | **~430 ms** | measured coarsely with 50 ms polling; different methodology |
| Idle RSS | ~20.98 MB | **~4.5 MB** (`ps rss`) | lower; baseline metric likely measured differently |
| RSS after full e2e load | — | **~4.3 MB** | stable, no growth after the 10-spec run |

Observations: the binary grew ~2×. This is explained by T14 (excelize, ~a few MB),
`modernc.org/sqlite` (CGO-free SQLite), and the pgx/mysql drivers — not a PRF-02
functional regression. Flagged **MEDIUM (footprint)**, no action required for
correctness; a future release task could trim build tags.

## 2. Backend benchmarks (`go test -bench -benchmem`)
| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| Execute result 1,000 rows | 3.77 ms | 223 KB | 9,281 |
| Execute result 10,000 rows | 41.7 ms | 2.65 MB | 99,293 |
| Execute result 50,000 rows | 145 ms | 15.4 MB | 499,312 |
| Execute result 100,000 rows | 257 ms | 31.2 MB | 999,318 |
| Introspect 100 tables | 17.5 ms | 492 KB | 11,453 |
| Introspect 500 tables | 90.7 ms | 2.46 MB | 57,061 |
| Introspect 1,000 tables | 193 ms | 4.93 MB | 114,068 |
| Concurrent queries | 264 µs/op | 14 KB | 338 |

(The Execute-result benchmarks exercise the **raw query API** result buffer, which
is bounded by the existing 50 MB cap; structured Table Data browsing is
server-side paginated and does not materialize the full table.)

## 3. TableData paging
Server-side pagination is enforced (`page_size` bounds); the browser only holds
one page. Frontend has mount-time perf assertions in the vitest suite
(`schema-tree.perf`, `result-grid` perf). 1,000,000-row benchmark **NOT RUN**
(paging is bounded by design; no full dataset enters frontend memory).

## 4. Import/Export
- T14 export streams via a DB cursor and spools to a temp file; the 1,000-row
  full-table export is asserted by `table-export.spec.ts` (1000 rows despite a
  100-row page).
- 100k-row import/export throughput and cancellation latency **NOT RUN** as a
  dedicated benchmark; integration tests cover round-trips.

## 5. Reconnect
`internal/database/manager_reconnect_test.go` proves per-key serialization
(connector opens == 2 for 12 concurrent reconnects → no pool stacking) and
db-scoped isolation. A long-run reconnect soak with goroutine/fd sampling was
**NOT RUN**.

## 6. Destructive operations
Delete Database runs only against disposable databases; e2e completes in ~2.5 s
including preflight + DROP + verification. No destructive perf test on real data.

## Not run (explicit)
- 1M-row table-data page/sort/filter benchmark.
- Import/export 100k throughput + cancellation latency.
- Reconnect soak with RSS/goroutine/fd curves.
