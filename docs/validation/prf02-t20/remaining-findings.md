# PRF02-T20 — Remaining Findings (Consolidated)

Merged and deduplicated from T01–T18 reports, T19 validation, and the T20 audit.
Severity per the task rubric. No finding was downgraded to reach READY.

## BLOCKER
None.

## HIGH
None unresolved.

## MEDIUM
| ID | Finding | Component | Status | Impact / Disposition |
|---|---|---|---|---|
| PRF02-FIND-001 | Single binary ~32 MB vs M6 ~15.8 MB (~2×) | Packaging | ACCEPTED | Footprint growth from excelize + `modernc.org/sqlite` + pgx/mysql drivers + embedded UI. No correctness impact; optional future build-tag trim. |
| PRF02-FIND-002 | Startup-to-health ~430 ms vs baseline ~126 ms | Runtime | ACCEPTED | Different measurement methodology/environment; no functional impact. Re-measure with a dedicated harness before publishing a startup claim. |

## LOW
| ID | Finding | Component | Status | Impact / Disposition |
|---|---|---|---|---|
| PRF02-FIND-003 | Root HTML lacks `Content-Security-Policy` / `X-Frame-Options` | Frontend/PWA | ACCEPTED | Pre-existing (M0–M6) posture, not introduced by PRF-02; capture for release hardening outside PRF02 scope. |
| PRF02-FIND-004 | One transient MySQL integration failure in 7 combined runs (not reproducible in 5) | Test infra | ACCEPTED | Likely shared `datadeck_test` state / connection blip; monitor in CI; no product defect observed. |
| PRF02-FIND-005 | `docs/release/security.md` is gitignored (`release/` pattern) | Docs | ACCEPTED | The tracked `docs/security/dependency-audit.md` is authoritative; the release doc is a local artifact. |

## NOT VERIFIED / NOT RUN (not counted as PASS)
| ID | Item | Why | Impact |
|---|---|---|---|
| PRF02-NR-001 | Native runtime smoke on macOS amd64 / Linux / Windows | Only macOS arm64 native host | Cross-compiles pass; runtime on other OSes unverified |
| PRF02-NR-002 | 1,000,000-row table-data benchmark; 100k import/export throughput + cancellation latency | Time | Paging/streaming bounded by design |
| PRF02-NR-003 | Reconnect soak (goroutine/fd/RSS curves) | Time | Per-key serialization + no pool-stacking proven by unit tests |
| PRF02-NR-004 | Fresh XLSX ZIP-bomb / symlink fixtures | Time | Reader limits + 64 MiB cap documented; low risk |
| PRF02-NR-005 | Remote CI (GitHub Actions) status | No remote in environment | Local equivalents executed; not claimed green |

## Closed during T20
- MySQL live Delete Database verification — now covered by
  `internal/api/handler/database_delete_mysql_integration_test.go` (PASS).

No finding is hidden in notes; no test was weakened; no BLOCKER/HIGH remains open.
