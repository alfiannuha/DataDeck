# DataDeck v0.1.0 — R04 Findings Register

Merged/deduplicated from M0–M6, PRF01, PRF02-T19/T20, R01, and the R04 audit.
**Zero unresolved BLOCKER/HIGH.**

## BLOCKER
None.

## HIGH
None unresolved.

## MEDIUM
| ID | Finding | Component | Status | Evidence / disposition |
|---|---|---|---|---|
| R04-FIND-001 | **DNS rebinding residual**: the CSRF same-origin rule cannot distinguish a rebinding attacker whose Origin host equals the request Host. | API boundary | OPEN (accepted) | Mitigated by unguessable connection IDs (random 128-bit) and unreadable cross-origin responses (no ACAO). Full Host-allowlist hardening would risk breaking dev/Docker access and is deferred to a dedicated task. Low real-world exploitability. |
| R04-FIND-002 | npm advisories: `sharp` (librsvg CVE), `source-map-js` (DoS), `postcss`/`next` (path traversal, incomplete fix) | Frontend build tooling / optional image lib | OPEN (accepted) | Build-time only or optional image library; not in the shipped static export or Go binary. `sharp`/`source-map-js` have non-breaking fixes, but `npm audit fix` also downgraded `typescript` (out of scope) and was reverted; no dependency changed in R04. Dependency-audit.md. |
| PRF02-FIND-001 | Binary footprint ~32 MB vs M6 ~15.8 MB | Packaging | ACCEPTED | Carried from R01. |
| PRF02-FIND-002 | Startup measuring methodology differs | Runtime | ACCEPTED | Carried from R01. |

## LOW
| ID | Finding | Component | Status | Evidence / disposition |
|---|---|---|---|---|
| R04-FIND-003 | Static HTML lacks `CSP`/`X-Frame-Options` | Frontend/PWA | ACCEPTED | Pre-existing (M0–M6); API omits them deliberately (no HTML). Carried. |
| R04-FIND-004 | Fresh XLSX ZIP-bomb / symlink adversarial fixtures not run | Import | NOT RUN | Reader limits + 64 MiB cap; low risk. |
| R04-FIND-005 | E2E suite-order flake: `database-script.spec.ts` uses fixed DB names; one failure observed in a full run, green on re-run and in isolation | Test infra | ACCEPTED | Same class as the T18 delete-spec flake (fixed there with run-unique names). Recommend applying run-unique names to `database-script.spec.ts`. |

## Fixed during R04 (hardening)
| ID | Fix | Evidence |
|---|---|---|
| R04-FIX-001 | **CSRF/Origin middleware** rejecting cross-origin unsafe methods (incl. multipart simple requests) while allowing same-origin + allowlist + non-browser clients. | `middleware/csrf.go` (+ `csrf_test.go`); dynamic 403/400 checks; E2E 10/10 |

No dependency was changed (the `npm audit fix` trial was reverted; see
`dependency-audit.md`).

## Carried / not reproven
- PRF02-NR-001…005 (native non-macOS smoke, large-scale perf, reconnect soak, remote
  CI) remain **NOT VERIFIED**; unaffected by R04.

No finding is downgraded to pass a gate; no secret value appears in this register.
