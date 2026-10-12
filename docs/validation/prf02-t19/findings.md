# PRF02-T19 — Findings, Severity & Disposition

Severity per the task rubric. No finding is downgraded to reach READY.

## BLOCKER
None. No wrong-database read/mutation/deletion, confirmation bypass, data
corruption, unsafe SQL execution, atomicity violation, credential exposure, or
unrecoverable data loss was observed across the executed suites.

## HIGH
None unresolved.

## MEDIUM
| ID | Finding | Evidence | Disposition |
|---|---|---|---|
| M-1 | Single-binary size ~32 MB vs M6 baseline ~15.8 MB (~2×) | `ls -lh` embedded binary; cross-builds ~30–32 MB | Documented. Explained by excelize (T14) + `modernc.org/sqlite` + pgx/mysql drivers + embedded UI. Not a functional regression; a release-trim task could reduce it. |
| M-2 | Startup-to-health ~430 ms vs baseline ~126 ms | `curl` polling at 50 ms | Different measurement methodology/environment; flagged for a dedicated re-measure. No functional impact observed. |

## LOW
| ID | Finding | Evidence | Disposition |
|---|---|---|---|
| L-1 | Root HTML lacks `Content-Security-Policy` / `X-Frame-Options` | `curl -D` on `/` | Pre-existing (M0–M6) posture, not introduced by PRF-02. Noted for release hardening outside T19 scope. |
| L-2 | E2E delete spec was flaky against a long-lived server (stale pool to a recreated DB) | full-suite run vs isolated run | Already fixed before T19 (T18 commit `7958007`): the delete e2e now uses run-unique database names. Re-run green under T19. |
| L-3 | `docs/release/security.md` is gitignored (`release/` pattern), so the T15 security amendment is not version-controlled | `.gitignore:46` | Observation only; the authoritative record lives in the tracked `docs/security/dependency-audit.md`. |

## NOT RUN (explicit; not counted as PASS)
| ID | Item | Why | Impact |
|---|---|---|---|
| N-1 | Live MySQL `DROP DATABASE` E2E | No disposable MySQL run scheduled this session | MySQL delete covered by unit + capability tests; engine matrix marks it NOT RUN |
| N-2 | Native runtime smoke on macOS amd64 / Linux / Windows | Only macOS arm64 native host available | Cross-compiles succeed; runtime on other OSes unverified |
| N-3 | 1,000,000-row table-data benchmark | Time/bounded-by-design | Paging is server-side and bounded |
| N-4 | 100k-row import/export throughput + cancellation latency | Time | Round-trips covered by integration tests |
| N-5 | Reconnect soak with goroutine/fd/RSS curves | Time | Per-key serialization + no pool-stacking proven by unit tests |
| N-6 | Fresh XLSX ZIP-bomb / symlink fixtures | Time | Reader limits + 64 MiB cap documented |
| N-7 | Remote CI (GitHub Actions) | No remote in environment | Not claimed green |

## In-scope fixes applied during T19
None. T19 is a pure validation pass: no product code was changed and no feature
was added. The only test-harness flake (**L-2**) had already been fixed in the
T18 commit; it was re-verified green in T19.
