# DataDeck v0.4.1 — Release Notes

- **Product:** DataDeck (local-first SQL database GUI)
- **Version:** 0.4.1
- **Channel:** release
- **Release date:** 2026-11-17
- **Build commit:** see tag `v0.4.1` (the hash is embedded at build time)
- **Build tool:** Go 1.26.x (CGO-free), Next.js 15 static export embedded

## New features

- **Connection reordering** — reorder connections by dragging the row handle (with a
  drop indicator) or via the three-dot menu: **Move to Top / Up / Down / Bottom**.
  The order is persisted to the embedded SQLite store (`sort_order`, migration 004,
  backfilled to the previous name order), written in one transaction via
  `PATCH /connections/order`. New connections append at the bottom; editing a
  connection never changes its position; the order survives refresh and restart.
- **Connection color & edit connection** — per-connection `#RRGGBB` color, an Edit
  action that preserves the stored password and unrelated metadata.
- **Schema management** — rename table, add/alter/drop column, create/drop index,
  add/drop constraint and foreign keys, and **delete table** (two-step
  type-to-confirm), all dialect-quoted and gated by per-engine capabilities.
- **Backup scopes** — table-scope and connection-scope backups (pure-Go, no external
  `pg_dump`/`mysqldump`) with a self-describing manifest.
- **Table data UX** — inline row editing, page-scoped row selection (select-all +
  indeterminate, bulk copy/delete), a page-number box with First/Prev/Next/Last and
  a real total page count, and query execution duration.
- **SQL export modes** — INSERT-only, `CREATE TABLE IF NOT EXISTS` + data, and
  `CREATE TABLE` + data (dialect-aware, inline constraints + secondary indexes).
- **Import drag-and-drop** — dropzone (drag/drop, browse, remove) over the existing
  upload → validate → execute flow.
- **View Data options** — Show All / First 100 / Last 100 (the last ordered by the
  primary key, with a documented fallback when no PK exists).
- **Database-aware Add Column type selector** with Length / Precision / Scale.
- **SQL execution safety guard** — high-risk statements require confirmation; the
  most destructive require typing `EXECUTE`.

## Improvements

- **Refresh Schema** reliably reloads every cached schema subtree (all databases)
  for server-level PostgreSQL connections, with a loading state, duplicate-request
  guard, and success/error feedback.
- **Explorer UX** — always-visible three-dot table menu (replaces hover shortcuts);
  object-type icons replace redundant type labels.
- **Global typography** — larger `text-xs` scale, readable labels, 18px dialog
  titles.
- **Destructive actions** are consistently rendered in the destructive red color.

## Bug fixes

- Refresh Schema no longer leaves stale schema trees for server-level connections.
- Pagination: compact icon controls, working Last page and total page count, and
  clamped direct page navigation.

## Security

- Credentials remain AES-256-GCM encrypted at rest and are never returned by the
  API; connection color/sort metadata carries no credentials.
- Loopback-only binding by default; destructive operations require confirmation and
  never use implicit CASCADE.
- No encryption key or credential format changes: existing user data (and the
  Keychain-held key on macOS) remains readable after upgrade.

## Known limitations

- MySQL SQL restore and MySQL/SQLite database deletion remain disabled by policy.
- The SQL risk guard is frontend-only (backend executes exactly the submitted SQL but
  does not re-classify); detection is conservative, not a full parser.
- Add Row uses an improved panel (not inline); the Add Column type list is curated
  per engine.
- Connection/table-scope **restore** of connection-scope zips is deferred; table
  artifacts restore via the database-restore path.
- SQLite cannot alter columns or add/drop constraints; these report as unsupported.
- Last 100 requires a primary key (falls back to First 100 with a notice otherwise).
- The macOS bundle is ad-hoc signed but **not notarized** (no Developer ID); first
  launch requires Gatekeeper "Open Anyway".

## Supported platforms

| Platform | Architecture | Artifact |
| --- | --- | --- |
| macOS | arm64 | `DataDeck-0.4.1-macos-arm64.dmg` / `.zip` + `datadeck_0.4.1_darwin_arm64` |
| macOS | amd64 | `datadeck_0.4.1_darwin_amd64` |
| Linux | amd64 | `datadeck_0.4.1_linux_amd64` |
| Linux | arm64 | `datadeck_0.4.1_linux_arm64` |
| Windows | amd64 | `datadeck_0.4.1_windows_amd64.exe` |

Databases: PostgreSQL, MySQL, SQLite.

## Upgrade notes

- The embedded SQLite schema gains migration 004 (`sort_order`); it is additive and
  backfills the previous visible order. Existing connection profiles, credentials,
  history and saved queries are preserved.
- The macOS app keeps using the Keychain-held encryption key; do not delete
  `~/.datadeck/` when upgrading or encrypted credentials will be lost.
- The default loopback port is **3200**.
