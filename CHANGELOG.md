# Changelog

All notable changes to DataDeck are documented here. This project follows
[Semantic Versioning](https://semver.org/).

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
