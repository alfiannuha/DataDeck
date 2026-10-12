# DataDeck v0.1.0 — Scope Manifest (R01 Scope Freeze)

## Included (verified against implementation + tests at `351444f`)
| Area | Status | Evidence |
|---|---|---|
| Local-first desktop-style web app (single binary) | INCLUDED | `cmd/server`, `internal/web` embed; e2e release-flow |
| Connection profiles + encrypted credentials | INCLUDED | `internal/security`, repository tests |
| PostgreSQL / MySQL / SQLite connection support | INCLUDED | drivers + integration tests |
| PostgreSQL multi-database discovery | INCLUDED | `postgres_databases.go`, PRF01 tests |
| Schema Explorer | INCLUDED | `schema-explorer.tsx`, tests |
| Query Editor / Results / History / Saved Queries | INCLUDED | M1–M3 suites |
| Typed workspace tabs + empty lifecycle | INCLUDED | `useWorkspaceStore`, T01/T02 tests |
| TableData browse, server-side page/sort/filter | INCLUDED | T03–T06 tests + integration |
| Safe row CRUD (insert/update/delete/duplicate) | INCLUDED | T07–T11 tests + integration |
| Table Structure inspection | INCLUDED | T13 tests + integration |
| Table import (CSV + restricted SQL) / export (CSV/XLSX/SQL) | INCLUDED | T14/T15 tests + integration + e2e |
| Database Context Menu + Database Reconnect | INCLUDED | T16 tests + integration + e2e |
| Database import/export (base tables) | INCLUDED | T17 tests; PG/SQLite restore; MySQL export-only |
| Safe Delete Database | INCLUDED with engine policy | T18 tests; PG e2e + MySQL integration |
| PWA/static frontend + embedded Go backend | INCLUDED | `build:static`, embedding, e2e |

## Deferred / out of scope (approved)
SSH bastion/tunneling; general-purpose authentication; public multi-user hosting;
schema designer; arbitrary SQL restore beyond the approved manifest/allowlist;
unrestricted destructive operations; cloud sync; scheduled backup; replication;
enterprise backup/recovery; unsupported engines. Also previously deferred: Drop
Table, SSE streaming, PostgreSQL Copy DDL.

## Engine limitations (approved policy — ADR-010)
- XLSX **import** deferred (all engines).
- MySQL **restore** disabled (`RESTORE_UNSUPPORTED`; DDL implicit commit).
- SQLite **delete database** disabled (`DATABASE_DELETE_UNSUPPORTED`; local file).
- Database export = **base tables only**; views/sequences/triggers/functions/
  extensions excluded and named in the artifact header.

No scope change is proposed. No deferred feature is pulled in; no approved feature
is removed.
