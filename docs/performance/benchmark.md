# DataDeck Performance Benchmarks

Status: M5-T07 baseline. Measures only; no product behavior was changed to
improve any number.

## Environment

| Component | Value |
|---|---|
| OS / arch | macOS Darwin 25.6.0, arm64 (Apple Silicon) |
| Go | go1.26.5 (module `go 1.23.0`) |
| Backend build | `CGO_ENABLED=0`, `ENVIRONMENT=test`, loopback |
| Node | v24.14.0 |
| Frontend | Next.js 15.5.26 / React 19.3.0 (build not required for these benches) |
| SQLite | `modernc.org/sqlite v1.38.2` |
| Grid/UI timing | jsdom (Vitest) — relative, not a real-paint metric |

These numbers are from one machine and are indicative, not universal targets.

## Methodology & Commands

```bash
# Backend startup + idle RSS (release-like build)
scripts/bench-backend.sh

# Backend micro-benchmarks (SQLite fixtures; no external DB required)
cd backend && go test ./internal/database/ ./internal/repository/ \
  -run '^$' -bench 'BenchmarkExecuteResultSizes|BenchmarkIntrospectTableCounts|BenchmarkHistoryListPage|BenchmarkSavedQueryListPage' \
  -benchtime=3x -benchmem

# Frontend benchmarks (grid, schema, export)
scripts/bench-frontend.sh
```

Benchmark fixtures are generated at runtime (recursive CTE for rows; DDL loop
for tables; raw inserts for pagination), so runs are reproducible without
committing large datasets. Backend query benchmarks use a temporary SQLite
target so CI does not need PostgreSQL/MySQL.

**Idle RSS definition:** resident set size of the backend process, loopback
binding, no active queries, measured 2 s after health-ready and again after a
further 3 s. Reported as `ps -o rss` (KiB→MiB). This matches PRD memory intent;
it is not a peak-under-load figure.

## Backend Startup & Idle Memory

Three runs (`startup_ms` = launch → `/health` returns 200):

| Run | startup_ms | idle RSS @2s | RSS @5s | binary size |
|---|---|---|---|---|
| 1 | 705 | 20.89 MB | 11.06 MB | 22.0 MB |
| 2 | 461 | 21.14 MB | 14.56 MB | 22.0 MB |
| 3 | 940 | 20.90 MB | 20.89 MB | 22.0 MB |

RSS is non-monotonic because Go returns memory to the OS asynchronously; treat
the 2 s sample as the conservative idle figure.

### PRD goal: backend RAM < 25 MB — **PASS**

- **Metric:** idle RSS, release build (`CGO_ENABLED=0`), loopback, macOS arm64.
- **Observed:** 20.89–21.14 MB at 2 s; 11.06–20.89 MB settled at 5 s.
- **Result: PASS** (worst sampled idle RSS 21.14 MB < 25 MB). The margin is
  modest, so any future background goroutine/cache growth should be re-measured.

## Query Benchmarks (SQLite, 3 columns, `-benchtime=3x`)

| Rows | ns/op | ms/op | B/op | allocs/op | JSON payload (1 ref) |
|---|---|---|---|---|---|
| 1,000 | 952,556 | 0.95 | 224 KB | 9,285 | 20 KB |
| 10,000 | 10,982,167 | 10.98 | 2.65 MB | 99,295 | 232 KB |
| 50,000 | 48,463,069 | 48.46 | 15.4 MB | 499,312 | 1.27 MB |
| 100,000 | 92,474,417 | 92.47 | 31.2 MB | 999,316 | 2.62 MB |

Interpretation: execution + encoding is roughly linear; the 50 MB result cap is
the hard upper bound. `B/op` reflects transient row structures before they are
released, well under the cap for a 100k-row × 3-column result.

## Grid Benchmark (virtualization)

jsdom render, `[perf]` lines:

| Rows | DOM rows | mount ms |
|---|---|---|
| 1,000 | 23 | 231 |
| 10,000 | 23 | 70 |
| 50,000 | 23 | 71 |
| 100,000 | 23 | 91 |

**Bounded virtualization confirmed: 23 DOM rows at 100k**, matching the M4
reference. Timing is harness-dependent.

## 1k-table Schema Benchmark

Introspection (real SQLite with generated tables):

| Tables | ns/op | ms/op | B/op | JSON tree |
|---|---|---|---|---|
| 100 | 5,287,042 | 5.29 | 478 KB | 41.6 KB |
| 500 | 26,557,764 | 26.56 | 2.40 MB | 208 KB |
| 1,000 | 52,200,555 | 52.20 | 4.79 MB | 416 KB |

Explorer rendering (1000 tables):

| Mode | DOM nodes | mount ms |
|---|---|---|
| Schema-level (PostgreSQL-style, collapsed schema) | 2 | 57 |
| Table-level (SQLite/MySQL-style, database expanded) | 1,001 | 412 |

The schema-level explorer is lazy (no table rows until the schema expands). The
table-level path (engines without a schema level) mounts every table, so 1000
tables ⇒ ~1001 DOM nodes; this is the main schema-scale bottleneck (see
Recommendations).

## History / Saved-Query Pagination Scale

10,000 rows seeded; page size 50 (`-benchtime=3x`):

| Query | ns/op | ms/op | B/op |
|---|---|---|---|
| history page 1 | 6,463,806 | 6.46 | 50.7 KB |
| history page 100 (offset 9,900) | 14,303,083 | 14.30 | 50.7 KB |
| history page beyond data | 14,172,447 | 14.17 | 0.9 KB |
| saved page 1 | 8,516,125 | 8.52 | 80.0 KB |
| saved page 100 (offset 9,900) | 16,793,972 | 16.79 | 80.0 KB |

Memory per page is constant regardless of depth, confirming the page-based UI
does not load the whole dataset. Deep `OFFSET` costs more time (SQLite scans),
but not memory.

## Export Benchmark (serialization only, outside React)

| Rows | CSV ms | JSON ms | CSV bytes | JSON bytes |
|---|---|---|---|---|
| 1,000 | 1.3 | 0.5 | 48 KB | 65 KB |
| 10,000 | 10.5 | 2.9 | 525 KB | 695 KB |
| 50,000 | 43.9 | 13.6 | 2.81 MB | 3.66 MB |
| 100,000 | 131.0 | 34.9 | 5.66 MB | 7.36 MB |

Export is a click-time, single-pass operation independent of rendering; memory
impact is one serialized string (CSV) or one row-object array + string (JSON),
bounded by the 50 MB backend cap.

## Concurrency (from M5-T05)

Concurrent query matrix 1/5/10/25/50 against PostgreSQL, MySQL and SQLite:
all operations succeed; peak goroutines stayed **flat at 14–16** across levels;
a pool configured with `MaxOpenConns=2` held `OpenConnections ≤ 2` under 8
concurrent long queries.

## Bottlenecks & Interpretation

1. **Table-level schema rendering** is the only clearly unbounded UI path:
   1000 tables ⇒ 1001 DOM nodes (412 ms in jsdom).
2. Introspection grows linearly (≈52 ms and 416 KB JSON at 1000 tables); fine
   for typical schemas, noticeable at the high end.
3. Deep pagination offset increases latency (SQLite scan) but not memory.
4. Backend query `B/op` at 100k (~31 MB transient) approaches half the 50 MB
   cap; results are truncated at the cap by design.
5. Idle RSS (~21 MB) is comfortably under 25 MB but not far from it.

## Limitations

- jsdom timings are not real-browser paint/render metrics; use them for
  regression direction, not absolute FPS.
- RSS depends on OS/GC scheduling; only the idle, no-load figure was measured.
- Backend query benchmarks use SQLite; PostgreSQL/MySQL timing differs. M4/M5
  integration runs validate correctness on all three, not throughput.
- Single-machine, single-run-per-size with `-benchtime=3x`/`1x`; variance is
  expected. Re-run on the target machine before drawing conclusions.
- No memory profiler heap snapshot was taken; `B/op` is allocator throughput,
  not peak RSS.

## Recommendations

1. **Virtualize or lazy-expand table-level schema nodes** (SQLite/MySQL) so a
   1000-table database does not mount 1000 rows — highest-value UI fix.
2. Consider capping/paginating very large introspection payloads if 1000+ table
   schemas become common.
3. Re-evaluate idle RSS after any new background goroutine/cache; the <25 MB
   margin is ~4 MB.
4. Keep the export benchmark as a regression guard (CSV remains the heavier
   path).
5. Add a real-browser (Playwright) first-render measurement during the M5
   review to complement jsdom numbers.

## Raw Artifacts

Repeatable via `scripts/bench-backend.sh`, `scripts/bench-frontend.sh`, and the
Go benchmarks in `backend/internal/database/benchmark_test.go` and
`backend/internal/repository/benchmark_test.go`.
