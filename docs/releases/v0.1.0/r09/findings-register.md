# DataDeck v0.1.0 — R09 Findings Register

Published-artifact smoke test. **No BLOCKER/HIGH.**

## BLOCKER / HIGH
None. (Tag target matches `6d99856`; all 5 packages present; 9/9 checksums match;
formats/arch correct; no public listener; no credential leak; no wrong-db op.)

## MEDIUM
| ID | Finding | Evidence | Disposition |
|---|---|---|---|
| R04-FIND-001 | DNS rebinding residual | R04/R05/R07/R09 | Accepted (MEDIUM) — no practical browser exploit path |
| R04-FIND-002 | npm build-time advisories | R04/R07 | Accepted (MEDIUM) |
| R07-F-001 | Bit-for-bit reproducibility not achieved | R06/R07/R08 | Accepted (MEDIUM) |

## LOW
| ID | Finding | Evidence | Disposition |
|---|---|---|---|
| R09-F-001 | Published **frontend** build-info channel/commit is stale (`development`/`unknown`) while backend reports `release`. | UI title `development · unknown`; `/health` `release` | Fix: pass `NEXT_PUBLIC_DATADECK_VERSION/_CHANNEL/_COMMIT` to `build:static` in `release.yml` (future patch release). Version itself is correct `v0.1.0`. No security/data impact. |
| R09-F-002 | `SHA256SUMS` uses `sbom/`/`licenses/` prefixes vs flat assets | downloaded `SHA256SUMS` | Verification by basename; recommend flattening (future). |
| R09-F-003 | Archive binary named `datadeck`/`datadeck.exe` vs docs `datadeck_0.1.0_*` | extracted archives | Align docs or packaging script (future). |
| R09-F-004 | Full E2E `table-menu` cleanup timeout (1/10) | full run; passes alone | Test-harness flake (R04-FIND-005 class); no product defect. |
| R09-F-005 | Playwright browser revision missing after `npm ci` | launch error | Environment; fixed with `npx playwright install chromium`. Not a product issue. |
| R04-FIND-003 | Static HTML lacks CSP/X-Frame | R04 | Accepted (LOW) |
| R05-FIND-004/005 | Failure injection / large-scale perf | R05 | NOT RUN |

## BLOCKED / NOT VERIFIED
| ID | Item | Reason |
|---|---|---|
| R09-NR-001 | MySQL runtime smoke | Local MySQL server not running (`ECONNREFUSED`); app correctly returned `CONNECTION_ERROR`. MySQL feature coverage from R05 integration. |
| R09-NR-002 | Native runtime on mac-amd64 / Linux / Windows | No native runners; download+hash+format verified only. |

No published asset was modified or replaced.
