# PRF-01 Migration & Backward Compatibility Review

Status: PRF01-T10. Proves that PRF-01 (PostgreSQL server-level connections,
database-bound tabs, database context in history/saved queries) upgrades an
existing M6 installation without destroying or invalidating user data.

## Fixture

Built with the **M6 release binary** at commit `d22d950` (PRF-01 absent) against
a fresh store, including real PostgreSQL/MySQL/SQLite targets.

| Item | Recorded value (identifiers only; no credentials dumped) |
|---|---|
| Profiles | `m6 admin`, `M6 PG` (database_name=`ccm`), `M6 MySQL` (datadeck_test), `M6 SQLite` (file) + their IDs |
| Schema | `schema_migrations = [(1, '001_initial.sql')]` |
| History | 9 entries |
| Saved queries | `M6 bound` (PG), `M6 unbound` |
| PG credential | encrypted; verified usable via an application query (`SELECT current_database()` → `ccm`) |
| PG databases on server | `ccm`, `datadeck_extra`, `postgres` |

No decrypted password was logged or written to any artifact.

## Migration Result

The **PRF-01 build** (current HEAD) started against the existing store:

- Application started; `GET /api/v1/health` → `200`.
- Migrator applied the additive migration:
  `schema_migrations = [(1, …), (2, '002_prf01_database_context.sql')]`.
- No destructive step; no rows rewritten.

## Credential Preservation

The M6 profile’s encrypted password was still decryptable after the upgrade:
the application executed PostgreSQL queries using it (`SELECT current_database()`
→ `ccm`, and an explicit `database=ccm` query succeeded). Credentials remain
AES-256-GCM ciphertext in the store and are never returned or logged.

## PostgreSQL Result

- Profiles and IDs preserved exactly.
- The legacy profile’s `database_name = ccm` is **unchanged** in storage.
- Executing without a `database` uses the legacy default (`ccm`) — original
  context is not silently destroyed.
- An explicit `database=ccm` request also succeeds.
- Discovery exposes additional server databases (`ccm` **and**
  `datadeck_extra`) without modifying the profile, matching the approved model.

## MySQL Result

`M6 MySQL` connected and executed successfully with unchanged single-database
semantics (no database discovery for MySQL).

## SQLite Result

`M6 SQLite` kept its target file and data: `SELECT id, note FROM t` returned the
original row, including the exact BIGINT string `9007199254740993`.

## History Result

All M6 history entries were preserved (count carried forward). Legacy rows have
`database_name = NULL` (no entry in the API payload), never inferred from the
current selection; new executions after the upgrade record their database.

## Saved-Query Result

Both M6 saved queries were preserved with identical IDs and titles; legacy
snippets have `database_name = NULL`. They remain openable and are never bound
to an unrelated database.

## Rollback Assessment

**Tested.** The M6 binary was run against the PRF-01-upgraded store:

- Application started; profiles, history and saved queries readable; PostgreSQL
  credential decrypted and `SELECT current_database()` → `ccm`; SQLite data
  intact.
- Migrations were left at `(1, 2)`; the M6 migrator ignored the newer version.
- The M6 code selects explicit column lists, so the added `database_name`
  columns are simply ignored.

Conclusion: because migration 002 is strictly additive (nullable columns, no
data rewrite, no type/key changes), an immediate rollback to the M6 binary is
**safe in this specific upgrade**. This is not a general downgrade guarantee:
any future migration that changes or removes data would require restoring a
pre-upgrade backup.

Backup guidance (unchanged): stop DataDeck, copy `datadeck.db` with
`-wal`/`-shm`, keep the encryption key separately, never beside the backup.

## Automated Migration Test

`TestMigration002IsAdditiveAndPreservesData` (storage package) simulates an M6
store (only migration 001, without the columns), reopens it with the PRF-01
migrator, and asserts the columns are added while profile/history/saved rows are
preserved (legacy `database_name` stays `NULL`).

## Findings

- No data loss, no ID churn, no credential breakage; the legacy
  `database_name` context is preserved and used as the default.
- Migration 002 is idempotent/additive and rollback was exercised for the M6
  binary, not assumed.
- Additional databases become *discoverable* without altering the existing
  profile, satisfying the compatibility model.

## Verdict

**PRF01-T10 READY**

Not proceeding to PRF01-T11.
