# DataDeck v0.1.0 — R10 Quality & Performance Assessment

## Quality evidence (current)
| Layer | Result |
|---|---|
| Backend unit | PASS — 15 packages (`go test -count=1 ./...`) |
| Backend race | PASS — 15 packages (`-race`) |
| Backend integration | PASS — PG 17.11 + MySQL 8.0.46 (`-tags=integration -p 1`) |
| Backend format/vet/build | gofmt clean, `go vet` clean, `CGO_ENABLED=0` build |
| Frontend unit | PASS — 51 files / 472 tests |
| Frontend lint/typecheck | PASS (0 / clean) |
| Frontend build/static | PASS |
| Browser E2E | PASS — 10/10 (R05; R09 against published binary 9/10 + 1 cleanup flake passing alone) |
| OpenAPI contract | PASS (swagger ↔ router ↔ envelope) |
| govulncheck | 0 reachable |

Coverage percentage: **not measured** (not claimed). Failure injection (disk-full,
network-loss) and 1M-row / 100k import-export benchmarks: **NOT RUN** (documented).
Known flaky test: `table-menu` e2e cleanup (suite-order); passes alone (R04/R05/R09).

## Performance (reference; not PRD-acceptance thresholds)
| Metric | M6 ref | v0.1.0 |
|---|---|---|
| Binary size | ~15.8 MB | ~24–32 MB (stripped/embedded varies) |
| Startup → health | ~126 ms | ~300–430 ms (methodology differs) |
| Idle RSS | ~20.98 MB | ~4.5 MB settled (`ps rss`) |
| Raw 100k-row query | — | ~257 ms |
| Introspect 1000 tables | — | ~193 ms |

Table-data paging is server-side/bounded; export/import stream with size caps.
Bit-for-bit reproducibility: NOT achieved (frontend export nondeterminism; no
`SOURCE_DATE_EPOCH`) — MEDIUM accepted; toolchain-reproducible (locked deps).
