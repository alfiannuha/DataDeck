# Known Limitations

Approved limitations for DataDeck **v0.1.0**. These reflect deliberate scope
decisions, not untested features.

## Deferred / out of scope

- **No SSH tunneling / bastion support.**
- **No general-purpose authentication** and **no public multi-user hosting** —
  DataDeck is local-first and loopback-only by default.
- **No schema designer** and no `ALTER TABLE` / table-management UI.
- **No unrestricted SQL restore** — restore accepts only DataDeck-exported artifacts
  against an allowlist, into an empty target.
- **No enterprise backup/recovery**, scheduled backups, replication, or cloud sync.
- **No `DROP TABLE` / `TRUNCATE TABLE` UI.**
- **No SSE/streaming query results** (results are returned as a bounded payload).
- **PostgreSQL "Copy DDL"** is not offered for server-level passwordless flows; the
  table menu does not expose DDL copy in v0.1.0.

## Engine limitations

- **MySQL restore disabled** (`RESTORE_UNSUPPORTED`; DDL implicitly commits).
- **SQLite database deletion disabled** (`DATABASE_DELETE_UNSUPPORTED`; local file).
- **Database export covers base tables only** (views/sequences/triggers/functions/
  extensions excluded and named in the artifact header).
- **XLSX import** not available (export only).
- **Row Update/Delete** require a declared primary key; no-PK tables support
  Insert/Duplicate only.
- **Table import** is INSERT-only for a single table.

## Deployment limitations

- **Docker deployment** binds `0.0.0.0` by design; do not expose it publicly.
  Vercel (or any static-only host) cannot host the persistent Go backend.
- **Windows/macOS AMD64 and Linux** binaries are compile-verified but were not
  natively smoke-tested at the time of this release.

## Roadmap (not in v0.1.0)

The **Concurrent Multi-Connection Engine** is a proposed **v0.2.0** enhancement
(cross-profile concurrent execution, safe pool-of-pools lifecycle, idle-pool
janitor, connection telemetry, environment badges). v0.1.0 already supports
explicit **per-tab connection/database binding** through PRF01/PRF02, but the
v0.2.0 concurrency engine is **not implemented** and is described here only as
future direction. No delivery date is promised.
