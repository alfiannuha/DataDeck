# DataDeck v0.1.0 — Project Closure Document

## Overview
DataDeck is a lightweight, local-first database GUI (Go backend + Next.js/React
frontend) shipped as a single CGO-free binary with the static frontend embedded.
It targets PostgreSQL, MySQL, and SQLite.

## Initial objectives (PRD)
Local-first DB GUI; connection management with encrypted credentials; schema
exploration; SQL editor with history/saved queries; table data browsing; safe row
CRUD; table structure; import/export; multi-database (PRF-01); database management;
single-binary, cross-platform distribution.

## Delivered scope (v0.1.0)
All of the above, plus PRF-02: typed workspace tabs (Query/TableData/Structure),
server-side pagination/sort/filter, safe PK-scoped CRUD with optimistic concurrency,
table structure endpoint, table export (CSV/XLSX/SQL) + import (CSV/SQL, atomic),
database context menu + database-scoped reconnect, database SQL export/restore
(PG/SQLite; base tables), two-phase Delete Database (PG/MySQL), CSRF/Origin hardening.

## Final architecture
Next.js/React/TS/Tailwind/Shadcn/CodeMirror 6/TanStack/Zustand/PWA → HTTP `/api/v1`
→ Go (Chi; `internal/{api,config,database,dbscript,export,model,repository,security,storage,tableimport,version,web}`)
→ target DBs (pgx / go-sql-driver/mysql / modernc sqlite); AES-256-GCM credentials;
embedded SQLite metadata; single binary.

## Supported platforms / databases
macOS arm64/amd64, Linux amd64/arm64, Windows amd64 (native runtime verified on macOS
arm64). PostgreSQL (full), MySQL (all except restore), SQLite (all except delete).

## Release
Version `0.1.0` · Tag `v0.1.0` → `6d99856` ·
https://github.com/alfiannuha/DataDeck/releases/tag/v0.1.0

## Test / security summary
Backend unit+race (15 pkgs), integration PG+MySQL, frontend 472 tests, e2e 10/10,
OpenAPI contract; govulncheck 0 reachable; CSRF/loopback/encryption verified; 10/10
artifact checksums (R06/R07) and 9/9 published checksums (R09).

## Known limitations
No SSH tunneling/auth/public hosting/schema designer; XLSX import unavailable; MySQL
restore disabled; SQLite delete disabled; base-tables-only DB export; no key rotation;
raw SQL can mutate data. See `docs/reference/limitations.md`.

## Residual risks
MEDIUM/LOW accepted (R04-FIND-001 DNS rebinding residual, R04-FIND-002 build-time
advisories, R07-F-001 reproducibility, R09-F-001/002/003 release-metadata ergonomics),
plus non-mac native runtime NOT VERIFIED. See `residual-risk-register.md`.

## Maintenance ownership / future
Maintainers: repository owner/engineering. v0.1.x maintenance plan in
`operational-maintenance.md`; v0.2.0 handoff + debt register in `lessons-and-roadmap.md`.

## Final decision
**R10 CLOSED** — v0.1.0 is released, audited, and closed with documented residual risks.
