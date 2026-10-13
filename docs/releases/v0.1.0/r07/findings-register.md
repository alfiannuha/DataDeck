# DataDeck v0.1.0 — R07 Findings Register

Consolidated R04–R06 + R07. **Zero unresolved BLOCKER/HIGH.**

## BLOCKER / HIGH
None.

## MEDIUM
| ID | Finding | Artifact | Evidence | Disposition |
|---|---|---|---|---|
| R04-FIND-001 | DNS rebinding residual | binaries | re-verified; needs unguessable id; no browser path | Accepted (re-confirmed) |
| R04-FIND-002 | npm build-time/optional advisories | frontend SBOM | build-time/optional only | Accepted |
| R07-F-001 | Bit-for-bit reproducibility not achieved (frontend export nondeterminism; no `SOURCE_DATE_EPOCH`) | all binaries | rebuild identical-to-itself but ≠ R06 hash | Accepted (toolchain-reproducible); future task |
| PRF02-FIND-001/002 | Binary footprint / startup methodology | binaries | ~24 MB stripped; startup method differs | Accepted |

## LOW
| ID | Finding | Status |
|---|---|---|
| R07-F-002 | Non-mac/Windows native runtime NOT VERIFIED (no runners) | Carried to R08 |
| R07-F-003 | No LICENSE/notice files packaged (repo has none) | Open |
| R04-FIND-003 | Static HTML lacks CSP/X-Frame | Accepted (pre-existing) |
| R05-FIND-004/005 | Failure injection / large-scale perf | NOT RUN |

## R06 verification notes
- R06-NR-002 (Docker) → N/A for the binary RC set (no daemon; packaging option only).
- R06-NR-004 (license) carried.
No original artifact was modified; no checksum regenerated to hide a mismatch.
