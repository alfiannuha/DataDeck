# DataDeck v0.1.0 — R05 Findings Register

Consolidated from R01–R04 + R05. **Zero unresolved BLOCKER/HIGH.**

## BLOCKER
None.

## HIGH
None.

## MEDIUM
| ID | Finding | Component | Status | Disposition |
|---|---|---|---|---|
| R04-FIND-001 | DNS rebinding residual (Origin==Host under a rebinding attacker) | API boundary | OPEN (accepted) | Reproduced in R05: multipart reaches the handler only with a valid **unguessable** 128-bit connection id; responses are unreadable cross-origin; browser JSON mutations are preflight-blocked. Full-browser exploit NOT RUN. Accept for v0.1.0. |
| R04-FIND-002 | npm build-time/optional advisories (`sharp`, `source-map-js`, `postcss`/`next`) | Frontend tooling | OPEN (accepted) | Not reachable in the shipped static export/binaries; major-only fix. `dependency-reassessment.md`. |
| PRF02-FIND-001 | Binary ~32 MB vs M6 ~15.8 MB | Packaging | ACCEPTED | Explained (deps + embedded UI). |
| PRF02-FIND-002 | Startup measurement methodology differs | Runtime | ACCEPTED | No functional impact. |

## LOW
| ID | Finding | Component | Status | Disposition |
|---|---|---|---|---|
| R04-FIND-003 | Static HTML lacks CSP/X-Frame | Frontend | ACCEPTED | Pre-existing; API deliberately omits them. |
| R04-FIND-004 | Fresh XLSX ZIP-bomb/symlink fixtures not run | Import | NOT RUN | Reader limits + 64 MiB cap; low risk. |
| **R05-FIX-001** (was R04-FIND-005) | `database-script.spec.ts` suite-order flake | Test infra | **FIXED** | Run-unique DB names added; full suite green twice. |
| R05-FIND-002 | Native runtime smoke on macOS-amd64/Linux/Windows not run | Packaging | OPEN (low) | Cross-compiles pass; carry to R06. |
| R05-FIND-003 | Docker image build not executed | Packaging | OPEN (low) | No Docker daemon; carry to R06/R07. |
| R05-FIND-004 | Failure injection (disk-full, network-loss-after-DROP) not simulated | Runtime | NOT RUN | Design-documented; carry as residual. |
| R05-FIND-005 | Large-scale perf (1M-row, 100k import/export) not benchmarked | Performance | NOT RUN | Paging/streaming bounded by design. |

## R05 changes
- `e2e/database-script.spec.ts`: run-unique DB names (R05-FIX-001).
- R05 evidence docs. **No product code change**; no dependency change; no scope change.

No finding is downgraded to pass a gate; no secret appears in this register.
