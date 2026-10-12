# DataDeck v0.1.0 — R01 Findings Register

Merged from M0–M6, PRF01, PRF02-T01–T20, security reviews, and the R01 preflight.
Deduplicated; IDs preserved. **Zero unresolved BLOCKER/HIGH.**

## BLOCKER
None.

## HIGH
None.

## MEDIUM
| ID | Finding | Component | Status | Release disposition |
|---|---|---|---|---|
| PRF02-FIND-001 | Binary ~32 MB vs M6 ~15.8 MB (~2×) | Packaging | ACCEPTED | Explained (excelize + modernc/sqlite + drivers + embedded UI); optional future trim. |
| PRF02-FIND-002 | Startup measurement differs (~312–430 ms vs baseline ~126 ms) | Runtime | ACCEPTED | Methodology/environment differ; not a functional regression. |
| R01-FIND-006 | **No CI run exists for the baseline (T10–T20 unpushed; `origin/main`=T09)** | CI/Baseline | OPEN (mitigated) | Local equivalents pass (test/e2e/integration/build). **Required action:** push baseline / open PR and get CI green on the frozen commit before R06. |
| PRF02-NR-001…005 | Native non-macOS smoke, 1M-row/100k import-export perf, reconnect soak, fresh XLSX adversarial fixtures, live CI | Various | NOT VERIFIED | Documented; non-blocking for R01; R04/R05/R06 to close where relevant. |

## LOW
| ID | Finding | Component | Status | Release disposition |
|---|---|---|---|---|
| PRF02-FIND-003 | Root HTML lacks CSP / X-Frame-Options | Frontend/PWA | ACCEPTED | Pre-existing (M0–M6); capture for release hardening outside PRF02. |
| PRF02-FIND-004 | One transient MySQL integration failure (1/7 runs; not reproducible) | Test infra | ACCEPTED | Monitor in CI. |
| PRF02-FIND-005 | `docs/release/` gitignored (repo `release/` pattern) | Docs | ACCEPTED | R01 artifacts placed in tracked `docs/releases/v0.1.0/r01/`. |
| R01-FIND-007 | Unrelated untracked file `multi_connection_architectural_specification.md` at repo root | Repo hygiene | OPEN (low) | Not ours; never committed. **Resolve before R06** so it cannot enter artifacts. |

## Closed
- MySQL live Delete Database verification — closed in T20 (`database_delete_mysql_integration_test.go`).

No finding is hidden or downgraded to reach READY.
