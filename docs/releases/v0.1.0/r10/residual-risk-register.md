# DataDeck v0.1.0 — R10 Residual Risk Register & Incident Review

**No unresolved BLOCKER/HIGH.** All residuals below are MEDIUM/LOW with explicit
disposition (accepted for v0.1.0; follow-up in a patch or v0.2.0).

| ID | Sev | Description | Impact | Mitigation | Disposition |
|---|---|---|---|---|---|
| R04-FIND-001 | MEDIUM | DNS rebinding residual | Low (needs leaked unguessable id; unreadable cross-origin; JSON mutations preflight-blocked) | Local-first model; CORS/CSRF; unguessable ids | Accepted |
| R04-FIND-002 | MEDIUM | npm build-time/optional advisories (sharp/source-map-js/postcss) | None in shipped static export/binary | Build-time only; no dep change | Accepted |
| R07-F-001 | MEDIUM | Bit-for-bit reproducibility not achieved | Reproducibility assurance only | Toolchain-reproducible; locked deps | Accepted; future task |
| R09-F-001 | LOW/MED | Published **frontend** UI channel label `development` (backend `release`) | Cosmetic metadata; version correct | Fix `release.yml` `build:static` env | Accepted; v0.1.1 candidate |
| R09-F-002 | LOW | `SHA256SUMS` path prefixes vs flat assets | Verification ergonomics | Verify by basename | Accepted |
| R09-F-003 | LOW | Archive binary named `datadeck` vs docs `datadeck_0.1.0_*` | Doc/name mismatch | Follow R09 doc note | Accepted |
| R04-FIND-003 | LOW | Static HTML lacks CSP/X-Frame | Pre-existing hardening | — | Accepted |
| R04-FIND-004 | LOW | Fresh XLSX adversarial fixtures not run | Unknown (low) | Library limits + 64 MiB cap | Accepted |
| R05-FIND-004/005 | LOW | Failure injection / large-scale perf not run | Unknown (low) | Bounded by design | Accepted |
| R07-F-002 | LOW | Non-mac native runtime NOT VERIFIED | Platform runtime risk | Cross-compile + format verified | Accepted (carry to CI runners) |
| R06-NR-004 | LOW | No repository LICENSE; THIRD_PARTY_NOTICES published | Legal clarity | Notices published | Accepted |

## Release incident review
- **R08:** the release workflow did not publish provenance (R08-F-001) — contained by
  uploading an accurate `published-provenance.json`; the release was never
  incomplete from the user's perspective (tag + packages + checksums + SBOM present).
- **R08:** a docs-only push accidentally advanced `main` instead of staying on the PR
  branch; no tag was moved, no asset replaced; disclosed in `r08/ci-verification.md`.
- **R09:** Playwright browser revision missing after `npm ci` (environment) — fixed
  with `npx playwright install chromium`; **MySQL server down** (environment) — smoke
  BLOCKED, app returned correct `CONNECTION_ERROR`.
- No checksum mismatch, wrong tag, wrong artifact, or public exposure incident.

No closure-blocking incident. All incident state is documented; nothing was hidden.
