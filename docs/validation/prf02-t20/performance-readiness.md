# PRF02-T20 — Performance & Packaging Readiness

Environment: macOS 26.6.2 arm64, Go 1.26.9 toolchain, PostgreSQL 17.11,
`CGO_ENABLED=0` single binary with embedded static frontend. Measurements are
from this host; the M6 baseline is a historical reference, not a threshold.

## Metrics
| Metric | M6 baseline | PRF-02 (T19/T20) | Assessment |
|---|---|---|---|
| Single binary size | ~15.8 MB | ~32 MB | +excelize (T14) + `modernc.org/sqlite` + pgx/mysql drivers + embedded UI. Documented MEDIUM (footprint), no correctness impact. |
| Startup → health | ~126 ms | ~430 ms (50 ms poll granularity) | Methodology differs; no functional impact; flagged for a dedicated re-measure. |
| Idle RSS | ~20.98 MB | ~4.5 MB (`ps rss`) | Lower; baseline measured differently. |
| RSS after full e2e | — | ~4.3 MB | Stable; no growth across the 10-spec run. |

## Benchmarks (`go test -bench -benchmem`)
| Case | ns/op | B/op |
|---|---|---|
| Execute result 10k rows | 41.7 ms | 2.65 MB |
| Execute result 100k rows | 257 ms | 31.2 MB |
| Introspect 1000 tables | 193 ms | 4.93 MB |
| Concurrent queries | 264 µs/op | 14 KB |

Raw-query results are bounded by the existing 50 MB cap; structured Table Data
browsing is server-side paginated and never materializes the whole table.

## Bounded resources
- Table export streams via a DB cursor and spools to a temp file.
- Table import spools to a temp file with a 64 MiB body cap; DB restore 256 MiB.
- Paging caps page size; the browser holds one page.

## Packaging (verified)
- Static export (`build:static`) + PWA (`manifest.json`, `sw.js` served 200) + Go embed → single binary.
- `CGO_ENABLED=0 go build ./...` PASS.
- Cross-compiles PASS (compile-only): darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64 (~30–32 MB).
- Native runtime smoke: macOS arm64 only; other targets **NOT RUN**.

## Not run (explicit, non-blocking)
1M-row table-data benchmark; 100k import/export throughput + cancellation latency;
reconnect soak (goroutine/fd/RSS curves).
