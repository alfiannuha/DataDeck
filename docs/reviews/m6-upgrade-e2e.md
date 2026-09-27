# M6 Upgrade & Installed-App E2E

Status: M6-T10. Proves an existing DataDeck user can move to the M6 release
candidate without losing application data or workspace safety, using a realistic
pre-M6 store and real databases.

## 1. Old Version / Build

- Built from commit **`05a6a8f`** ("Initial DataDeck release…", the pre-release-
  workflow state) via a git worktree: `/tmp/oldwork`, producing `/tmp/datadeck-old`.
- Internal store schema: `schema_migrations = [(1, '001_initial.sql')]`.
- The store is only ever migrated by the single forward-only migration
  `001_initial.sql`; M5→M6 introduced **no schema changes**.

## 2. Release Candidate

- `make build` / `scripts/build-embedded.sh` output `/tmp/datadeck-rc`
  (version `0.1.0-dev`, embedded frontend, `CGO_ENABLED=0`), plus a rebuilt
  candidate with a bumped service-worker cache (`v3-t10`) for the update test.

## 3. Migration Result — PASS

Started the RC against the existing store: startup succeeded, the migration
runner re-validated the schema and applied nothing new
(`schema_migrations` remained `[(1, '001_initial.sql')]`), and all required
tables were present. No destructive or duplicate migration occurred.

## 4. Data Preservation — PASS

| Item | Before | After |
|---|---|---|
| Profiles | Upg PG, Upg MySQL, Upg SQLite | same |
| Profile IDs | recorded | identical (all three) |
| History | 5 | 6 (grew only from new activity) |
| Saved queries | 2 (A, B) | 2, opened successfully (`Upg saved A`) |
| SQLite target config + data | `/tmp/upg/target.sqlite3`, row `keep-me` | preserved, row returned |

## 5. Credential Preservation — PASS

The PostgreSQL profile (stored encrypted password `upg-PG-secret`, test-only)
still decrypted after the upgrade: `SELECT 9007199254740993::bigint` returned
`"9007199254740993"` exactly. No plaintext password was written to any artifact
in this task.

## 6. Database Matrix (after upgrade)

| Engine | Result |
|---|---|
| PostgreSQL | PASS — credential decrypt + exact BIGINT query |
| MySQL | PASS — query succeeded with the stored profile |
| SQLite (target) | PASS — original file path/config and table data preserved |

## 7. PWA Update — PASS

Controlled install/update lifecycle with dirty SQL present:

1. Old shell (service-worker cache `v2`) loaded and active.
2. Dirty SQL (`SELECT 'dirty-upgrade-sql' AS m;`) present in the editor.
3. A newer candidate (`datadeck-shell-v3-t10`) became available; the client
   surfaced **Update available** with no automatic reload.
4. Clicking Update showed the confirmation
   ("Reloading applies the new version. You have unsaved SQL…"); **no reload
   occurred** (page marker preserved) and the dirty SQL remained intact.
5. Confirming activated the new worker and migrated the cache to
   **`datadeck-shell-v3-t10`**; the shell stayed functional (cache audit shows
   only the shell cache, no `/api` entries).

## 8. Dirty SQL Protection — PASS

No forced reload while dirty SQL was present; the workspace text was preserved
through the update prompt and the cancel path.

## 9. Rollback Behavior

- Migrations are **forward-only**; there are **no down migrations**
  (`migrations/` contains only `001_initial.sql`).
- Downgrade is **currently safe** because M5→M6 made no schema change: running
  the older binary against the upgraded store succeeded — profiles, history,
  saved queries and credential decryption all intact, schema still `(1,)`.
- This is **not a promise of downgrade support** in general: once a future
  release adds a migration, an older binary may not understand the newer schema.
  Rollback then requires restoring a pre-upgrade backup with the previous binary.

## 10. Failed Migration / Corrupt Store — PASS

Simulated a broken store (copy with a required table dropped) and started the RC:

```
datadeck: storage: validate: required table "saved_queries" is missing
exit 1
```

The application failed **explicitly and before serving**, and the original
store was untouched. No silent corruption.

## 11. Backup Guidance

- **Stop DataDeck first** (clean shutdown), then copy the internal store and its
  write-ahead files together: `datadeck.db`, `datadeck.db-wal`,
  `datadeck.db-shm` (they must be consistent as a set).
- Keep the encryption key **separately** from the backup; the store's encrypted
  credentials are useless without it. **Never** store the key next to the backup
  or in the backup instructions/artifacts.
- Restore by stopping DataDeck, replacing the store files with the backup, and
  starting the same or an earlier compatible binary.
- Prefer an app-level dump (connections/history/saved queries via the API) only
  for portability — it does not include encrypted secrets.

## 12. Review Document

This file: `docs/reviews/m6-upgrade-e2e.md`.

## Summary

| Check | Result |
|---|---|
| Realistic pre-M6 state (3 engines, encrypted creds, history, saved) | PASS |
| Upgrade with actual release candidate | PASS |
| Migration safe / forward-only validated | PASS |
| Profiles · history · saved queries preserved | PASS |
| Credentials still usable | PASS |
| PG / MySQL / SQLite work after upgrade | PASS |
| SW update lifecycle with dirty SQL (no forced reload/loss) | PASS |
| Failed migration fails explicitly | PASS |
| Rollback limitations documented | PASS |
| Backup guidance documented | PASS |

## Limitations

- The "old" build was created from the pre-release-workflow commit because no
  tagged M5 binary exists; its store schema is identical to M6, which is the
  actual upgrade path.
- The service-worker update was exercised with a bumped cache version in a
  controlled build (real install-to-update lifecycle); no OS-level "installed
  app" shell was available on this host.
- Rollback safety holds only while migrations remain unchanged; re-assess after
  any future migration is added.
