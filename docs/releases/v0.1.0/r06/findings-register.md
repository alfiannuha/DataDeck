# DataDeck v0.1.0 — R06 Findings Register

**Zero unresolved BLOCKER/HIGH.**

## BLOCKER / HIGH
None.

## MEDIUM (carried, accepted)
| ID | Finding | Status |
|---|---|---|
| R04-FIND-001 | DNS rebinding residual (needs an unguessable connection id) | Accepted |
| R04-FIND-002 | npm build-time/optional advisories | Accepted |
| PRF02-FIND-001/002 | Binary footprint / startup methodology | Accepted |

## LOW / NOT RUN (carried to R07)
| ID | Finding | Status |
|---|---|---|
| R06-NR-001 | Native runtime smoke on mac-amd64 / Linux / Windows | NOT VERIFIED (no runners) |
| R06-NR-002 | Docker image build | NOT RUN (no Docker daemon) |
| R06-NR-003 | Bit-for-bit reproducibility (rebuild compare) | NOT VERIFIED |
| R06-NR-004 | No LICENSE/notice files packaged | OPEN (repo has none) |
| R05-FIND-004/005 | Failure injection / large-scale perf | NOT RUN |

## R06 notes
- No source change; no dependency change; one test-harness fix landed in R05.
- All five RC binaries built from the single approved source `f92ce49`, `channel rc`.
