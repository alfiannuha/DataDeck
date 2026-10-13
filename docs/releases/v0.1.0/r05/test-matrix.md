# DataDeck v0.1.0 — R05 Test Matrix

Commit `e81ca8e` (+ R05 test fix). Environment: macOS 26.6.2 arm64; Go 1.26.9
toolchain; Node 24.14.0; PostgreSQL 17.11; MySQL 8.0.46; Playwright 1.63/Chromium.
Disposable DBs only (`datadeck_e2e_*`, temp SQLite files).

Status legend: PASS / FAIL / BLOCKED / NOT RUN / N/A.

| ID | Scenario | Engine | Expected | Actual | Status |
|---|---|---|---|---|---|
| S-01 | `gofmt -l .` clean | Go | empty | empty | PASS |
| S-02 | `go vet ./...` | Go | no findings | clean | PASS |
| S-03 | `go mod verify` | Go | verified | verified | PASS |
| S-04 | Unit tests | Go | pass | 15 pkgs ok | PASS |
| S-05 | Race tests | Go | pass | 15 pkgs ok | PASS |
| S-06 | Integration | PG+MySQL | pass | all ok | PASS |
| S-07 | Frontend lint/typecheck | TS | clean | clean | PASS |
| S-08 | Frontend unit | React | pass | 472 tests | PASS |
| S-09 | Production + static build | Next | pass | ok | PASS |
| S-10 | Version validator | — | consistent 0.1.0 | OK | PASS |
| S-11 | Single-binary build + `--version` | — | 0.1.0 | ok | PASS |
| S-12 | Cross-compile ×5 | — | compile | ok | PASS |
| S-13 | α/β isolation (browse/sort/filter) | PG | no crossover | ok | PASS |
| S-14 | α/β CRUD isolation | PG | α only | ok | PASS |
| S-15 | Table export α/β isolation (1000 rows) | PG | α data only | ok | PASS |
| S-16 | Table import target binding | PG | bound | ok | PASS |
| S-17 | Reconnect α while Explorer on β | PG | targets α | ok | PASS |
| S-18 | DB export α / restore; non-empty → 409 | PG | isolated | ok | PASS |
| S-19 | Delete α; β intact; protected/token rejected | PG | safe | ok | PASS |
| S-20 | MySQL live delete | MySQL | verified gone | ok (integration) | PASS |
| S-21 | CSRF: cross-origin POST/multipart | — | 403 | 403 | PASS |
| S-22 | CSRF: same-origin / no-origin | — | allowed | 400 (validation) | PASS |
| S-23 | CSRF: null origin | — | 403 | 403 | PASS |
| S-24 | DNS-rebinding residual characterization | — | documented MEDIUM | reproduced | PASS |
| S-25 | Missing/invalid key aborts startup | — | refuse | refuse | PASS |
| S-26 | Non-loopback bind without ALLOW_REMOTE | — | refuse | refuse | PASS |
| S-27 | Loopback-only listener | — | 127.0.0.1 | 127.0.0.1 | PASS |
| S-28 | Graceful shutdown (SIGTERM) | — | exit | exit | PASS |
| S-29 | Browser E2E full suite ×2 | PG | 10/10 | 10/10 | PASS |
| S-30 | govulncheck | Go | 0 reachable | 0 | PASS |
| S-31 | npm audit triage | npm | documented | 4 (build/optional) | PASS |
| S-32 | CI on current revision | GitHub | green | see ci-verification.md | (pending) |
| S-33 | DNS-rebinding full browser exploit | — | n/a (needs leaked id) | NOT RUN | NOT RUN |
| S-34 | Disk-full / network-loss injection | — | documented | NOT RUN | NOT RUN |
| S-35 | 1M-row / 100k import-export perf | — | bounded | NOT RUN | NOT RUN |
| S-36 | Native runtime on non-macOS | — | — | compile-only | N/A |

**R05 fix:** `database-script.spec.ts` now uses run-unique DB names, eliminating the
suite-order flake (R04-FIND-005); full suite green twice.
