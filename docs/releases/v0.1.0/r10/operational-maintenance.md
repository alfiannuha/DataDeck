# DataDeck v0.1.0 — R10 Operational Readiness, Support & Maintenance

## Operational readiness
| Item | Status | Reference |
|---|---|---|
| Fresh installation (download → extract → run) | VERIFIED (R09, macOS arm64) | R03 installation + R09 |
| `ENCRYPTION_KEY` setup + fail-closed | VERIFIED | R04/R09 |
| Startup / localhost access | VERIFIED | R09 |
| Connection setup | VERIFIED | R09 e2e |
| Persistent metadata (`STORAGE_PATH`) | VERIFIED | design + R09 |
| Graceful shutdown / restart | VERIFIED | R09 |
| Backup / recovery guidance | DOCUMENTED | `docs/security/security-guide.md` |
| Troubleshooting | DOCUMENTED | `docs/reference/troubleshooting.md` |
| Upgrade considerations | DOCUMENTED (first release; key must persist) | `docs/releases/v0.1.0/release-notes.md` |
| Known limitations | DOCUMENTED | `docs/reference/limitations.md` |

**Not suitable for public multi-user hosting** — documented; loopback default.

## Support & maintenance readiness (v0.1.x plan)
- **Build reproduction:** `scripts/build-embedded.sh` / `build-release.sh`; version
  validator `scripts/check-version.sh`; R06 `docs/releases/v0.1.0/r06/`.
- **Checksum verification:** published `SHA256SUMS` + `published-provenance.json`.
- **Diagnosis:**
  - Startup failures → `ENCRYPTION_KEY` / port / storage path (troubleshooting.md).
  - Connection failures → `CONNECTION_ERROR`/`DATABASE_*` taxonomy.
  - Encryption-key issues → no rotation; restore key or recreate profiles.
  - Database-isolation defects → α/β e2e + pool-key tests.
  - Import/export failures → `IMPORT_*`/`RESTORE_*` codes.
- **Security reports:** local-first; advisories tracked in `docs/security/dependency-audit.md`.
- **Patch release policy:** version bump (R02 style) + full R05/R06/R07/R08/R09 rerun;
  never force-move `v0.1.0`.
- **Incident response:** preserve artifacts, do not silently replace public assets,
  follow R08 §35 recovery; obtain explicit approval for public changes.

## v0.1.x maintenance checklist
1. Monitor Go/npm advisories (`govulncheck`, `npm audit`).
2. Patch dependency/stdslib issues via a controlled release-blocking change.
3. Re-run gofmt/vet/unit/race/integration, frontend lint/typecheck/test/build,
   e2e, and re-verify artifacts before any patch tag.
4. Regression: α/β isolation, CRUD concurrency, import atomicity, restore, delete.
5. Update CHANGELOG + release notes; publish a new tag (never reuse `v0.1.0`).
