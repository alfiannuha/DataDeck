# Changelog

All notable changes to DataDeck are documented here. This project follows
[Semantic Versioning](https://semver.org/).

## [Unreleased] — v0.4.x

### Fixed
- **Refresh Schema** now refetches every cached schema subtree (all databases)
  for server-level PostgreSQL connections, not just the database list; it shows a
  spinning/disabled state, prevents duplicate requests, and reports success or
  failure.
- **Pagination**: compact icon controls (ChevronsLeft/ChevronLeft/ChevronRight/
  ChevronsRight) with larger relative icons, a compact page input, and a working
  **Last** page. The table-data API now supports an opt-in `include_total=true`
  (`COUNT(*)` with the same filters) so the UI can display a real total page count
  and enable Last/direct navigation; when the total is unknown it is never
  invented.

### Added
- **Delete Table** from the schema explorer table menu, with two-step
  confirmation (warning → type the exact table name). Backend `drop_table` DDL
  (quoted, metadata-validated, never CASCADE; dependency errors surfaced).
- **SQL execution safety guard**: high-risk statements (unfiltered UPDATE/DELETE,
  TRUNCATE, DROP, destructive ALTER, multiple destructive statements) require a
  confirmation dialog; the most destructive require typing `EXECUTE`, and the
  exact confirmed SQL is the SQL executed.
- **SQL export modes**: SQL export offers INSERT-only (default), `CREATE TABLE
  IF NOT EXISTS` + data, and `CREATE TABLE` + data (inline constraints +
  secondary indexes), generated in the active dialect.
- **Import drag-and-drop**: a dropzone (drag/drop, browse, remove, replace) wraps
  the existing upload → validate → execute flow.
- **View Data submenu options**: "View First 100" and "View Last 100" generate
  the SQL, open a query tab and auto-execute it; Last 100 orders by the primary
  key descending (falls back to First 100 with a notice when there is no PK).

### Changed
- **Add Row panel** redesigned: grouped "Fields" / "Database-managed" sections,
  visible column types, type-aware controls (date/boolean/numeric/JSON),
  DEFAULT/NULL mode hints, a fixed footer and field-level validation. Inline add
  was evaluated and deferred in favour of the panel to reuse the proven insert
  path (DEFAULT/identity/generated handling) without destabilizing the
  virtualized grid.
- **Add Column** now uses a **database-aware grouped type selector** (PostgreSQL
  / MySQL / SQLite catalogues) with Length / Precision / Scale fields; the
  dialog receives the connection's engine.
- **Global typography**: the shared `text-xs` scale is now 13px (was 12px, via
  the theme token), the smallest 10/11px labels across modules now use that token
  for consistency, and dialog titles are 18px. Code-editor typography is
  unchanged.
- Delete Database uses an explicit two-step flow (warning → type the database
  name) and shows the connection name.
- Destructive actions (Delete Row / Selected Rows / Table / Database, structure
  drops) are rendered in the destructive (red) treatment.

## [0.4.0] — 2026-11-11

### Added
- Connection color: a per-connection `#RRGGBB` hint with a create/edit color picker,
  a color indicator in the connection list, and a backward-compatible default for
  records saved before colors existed (PRF-03/A).
- Edit connection: `PUT /api/v1/connections/{id}` plus an Edit action in the sidebar.
  Leaving the password blank keeps the stored credential; unrelated metadata is
  preserved and live pools are dropped so the next use reconnects (PRF-03/B).
- Sortable table-data column headers now show a sort-direction icon and a pointer
  cursor so sortability is discoverable.
- Schema management (PRF-04/Phase 2): rename table, add/alter/drop column,
  create/drop index, and add/drop constraint (PRIMARY KEY / UNIQUE / CHECK /
  FOREIGN KEY) through `POST /connections/{id}/table-structure`, driven by the
  Structure tab and the table context menu ("Rename Table…"). Identifiers are
  dialect-quoted and raw fragments validated; PostgreSQL/SQLite run in a
  transaction, MySQL rejects multi-statement changes. Per-engine
  `structure_capabilities` hide unsupported controls (SQLite cannot alter columns
  or add/drop constraints).
- Backup scopes (PRF-04/Phase 3): table-scope backup
  (`POST /connections/{id}/table-backup`) producing a self-contained DataDeck SQL
  artifact (structure + indexes + constraints/FK + data) for one table, and
  connection-scope backup (`POST /connections/{id}/connection-backup`) producing
  a zip of per-database artifacts plus `manifest.json`. Both are pure-Go (no
  external `pg_dump`/`mysqldump`) and record `scope` + `datadeck_version` in the
  manifest. Database-scope export/restore (PRF-02/T17) is unchanged.
- Table-data UX (PRF-04/Phase 4): a direct page-number box with First/Previous/
  Next controls (icon buttons with tooltips) that preserves sort and filters;
  the query execution time now appears as `Query complete HH:MM:SS.mmm` on the
  pagination row, measured by the database (not render time). Sortable headers
  already show their direction icon and pointer cursor.
- Schema explorer UX (PRF-04/Phase 5): the hover-only table shortcuts are gone —
  an always-visible three-dot row menu holds the actions (View Data, New Query,
  Structure, Rename, Refresh, Export, Import, Copy, Count Rows, Copy DDL). Object
  type labels (BASE TABLE, MATERIALIZED VIEW, …) are replaced by leading icons
  (database/table/view/materialized view/schema).

## [0.1.0] — prepared (not yet published)

Initial release: the DataDeck Database Explorer & Table Data Management feature set.

### Added
- Connection profiles with AES-256-GCM encrypted credentials (PostgreSQL, MySQL, SQLite).
- PostgreSQL server-level profiles with multi-database discovery and lazy per-database pools.
- Schema Explorer with lazy metadata loading, table/database context menus, and keyboard access.
- Typed multi-tab workspace (Query / Table Data / Table Structure) with explicit, tab-local
  connection/database binding and an empty-workspace lifecycle.
- Server-side Table Data pagination, sorting, and filtering (typed values, parameter-bound).
- Safe single-row CRUD (insert/update/delete/duplicate) with declared-PK identity,
  optimistic concurrency, and affected-row (0/1/>1) safety.
- Read-only Table Structure inspection (columns, keys, indexes, constraints, foreign keys).
- Table export (CSV/XLSX/SQL) and table import (CSV + restricted INSERT-only SQL, atomic).
- Database SQL export (base tables; schema+data / schema / data) and restore (PostgreSQL/SQLite).
- Database reconnect (database-scoped, fail-closed) and two-phase Delete Database with a
  protected-database policy.
- PWA/static frontend embedded in a single CGO-free Go binary; loopback-only by default.
- `datadeck --version`, `/api/v1/health` version metadata, and a version-consistency validator.

### Notes
- MySQL restore and SQLite database deletion are intentionally disabled (documented policy).
- Database export covers base tables only.
- XLSX import is deferred to a future release.

[0.1.0]: https://github.com/alfiannuha/DataDeck
