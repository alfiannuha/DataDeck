# DataDeck v0.1.0 — Release Notes

> **Status:** prepared for release. Artifacts are **not yet published**; downloading
> and hosting occur in a later release step. This document does not contain
> download links or checksums.

## Overview

DataDeck v0.1.0 is the first release: a lightweight, **local-first** database GUI
shipped as a single self-contained binary (Go backend + embedded Next.js frontend)
for PostgreSQL, MySQL, and SQLite.

## Highlights

- Connection profiles with **AES-256-GCM encrypted credentials**.
- PostgreSQL **multi-database discovery**; MySQL and SQLite support.
- Schema Explorer with lazy per-database metadata.
- SQL editor with explicit query tabs, history, and saved queries.
- Virtualized **Table Data** browsing with server-side pagination, sorting, and filtering.
- Safe single-row **CRUD** with optimistic concurrency and affected-row safety.
- Read-only **Table Structure** inspection.
- **Table import/export** (CSV/XLSX/SQL export; CSV + restricted SQL import, atomic).
- **Database export/restore** (base tables; PostgreSQL + SQLite restore).
- Database **reconnect** (database-scoped, fail-closed) and two-phase
  **Delete Database** with a protected-database policy.
- PWA/static frontend embedded in the binary; loopback-only by default.

## Supported platforms

macOS (arm64, amd64), Linux (amd64, arm64), Windows (amd64). See
[../../reference/supported-platforms.md](../../reference/supported-platforms.md).

## Supported databases

| Engine | Notes |
|---|---|
| PostgreSQL | Full feature set, including multi-database discovery, restore, delete |
| MySQL | All except **restore** (disabled — DDL implicitly commits) |
| SQLite | All except **delete database** (disabled — local file) |

## Security

Local-first, loopback-bound by default, **no general-purpose authentication**.
Credentials are encrypted at rest with a required `ENCRYPTION_KEY`. Raw SQL and
destructive operations run with your database credentials. See
[../../security/security-guide.md](../../security/security-guide.md).

## Known limitations

No SSH tunneling, no authentication/public hosting, no schema designer, no MySQL
restore, no SQLite database deletion, table import is INSERT-only, database export
covers base tables only, and XLSX import is deferred. Full list:
[../../reference/limitations.md](../../reference/limitations.md).

## Installation

See [../../getting-started/installation.md](../../getting-started/installation.md).
Build from source until artifacts are published
([../../development/building.md](../../development/building.md)).

## Upgrade notes

First release; no migration from a prior version. `ENCRYPTION_KEY` must be set and
kept stable — losing it makes stored passwords undecryptable (no rotation in
v0.1.0).

## Known issues

Accepted, non-blocking findings carried from validation:

- Single-binary size grew to ~32 MB (from ~15.8 MB at M6); explained by added
  dependencies (XLSX, pure-Go SQLite, database drivers) plus the embedded UI.
- Startup time reference differs from the M6 baseline; measurement methodology
  differs and no functional impact was observed.
- Windows/macOS-amd64/Linux binaries are compile-verified; native smoke tests on
  those platforms are pending.
- Root HTML does not set `Content-Security-Policy`/`X-Frame-Options` (pre-existing).

## Release artifacts

Expected names (not yet published): `datadeck_0.1.0_<os>_<arch>[.exe]`. No download
URLs or checksums are provided here — they are produced by the release workflow.
