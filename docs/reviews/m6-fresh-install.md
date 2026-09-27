# M6 Fresh-Install Validation

Status: M6-T09. Release artifacts (not the source tree) were installed and
exercised in a clean temporary environment. The Docker production image was also
validated because Docker became available in this environment.

Version under test: `0.1.0-dev`. Date: 2026-09-28.

## 1. Artifact Tested

Built by `scripts/build-release.sh` and packaged by `scripts/package_artifacts.py`:

- `datadeck_0.1.0-dev_darwin_arm64.tar.gz` extracted to `/tmp/fresh/extract/`
  containing only `datadeck` (16,574,802 bytes) and `VERSION`.
- No repository checkout, `node_modules`, or Node/Go runtime was used at run
  time.

## 2. Environment

| Item | Value |
|---|---|
| Host | macOS arm64 |
| Runtime env | `env -i PATH=/usr/bin:/bin` (verified: `node`, `npm`, `go` absent) |
| Storage | clean temp dir `/tmp/fresh/data/datadeck.db` |
| Key | ephemeral 32-byte test key supplied via env |
| Databases | real PostgreSQL 17.11, MySQL 8.0.46 |
| Docker | Docker Desktop available (image built and run) |

## 3. Binary Fresh-Install Result — PASS

1. Extracted archive (no source tree).
2. Supplied `ENCRYPTION_KEY`.
3. Started `datadeck` from `/tmp/fresh/extract`.
4. Startup → `/api/v1/health` in **866 ms**.
5. Health `{"status":"healthy"}`; `GET /` **200**; `manifest.json` **200**;
   `sw.js` **200**.
6. Created connections and executed queries (see matrix).

## 4. Database Matrix

| Engine | Connect | Query result |
|---|---|---|
| PostgreSQL 17.11 | PASS | `9007199254740993::bigint` → `"9007199254740993"` (exact) |
| MySQL 8.0.46 | PASS | `CAST(9223372036854775807 AS SIGNED)` → `"9223372036854775807"` |
| SQLite (file) | PASS | DDL + insert + `SELECT` → exact BIGINT `"9007199254740993"` |

Also verified: schema introspection for PostgreSQL and SQLite; wrong MySQL
password → `CONNECTION_ERROR` with no credential leak.

## 5. First-Run Storage — PASS

- Storage directory is **auto-created** when missing; the parent directory mode
  is `0700` (`drwx------` verified on a nested path).
- Files created: `datadeck.db`, `datadeck.db-shm`, `datadeck.db-wal`, all mode
  `0600`.
- Migrations applied: `schema_migrations` row `(1, '001_initial.sql')`; all
  tables present.

## 6. Restart — PASS

SIGTERM produced `shutdown_complete`; the packaged binary was started again
against the same store:

- Connections persisted (`Fresh MySQL`, `Fresh MySQL OK`, `Fresh PG`,
  `Fresh SQLite`).
- Stored **encrypted** PostgreSQL credential still decrypted (query succeeded).
- History persisted (grew only from new activity).
- Saved query (`Fresh saved`) present.
- Migrations unchanged (no re-run/destructive action).

## 7. PWA — PASS

Against the packaged single binary (Chromium):

- Manifest `display: standalone`, icons `192x192`/`512x512`.
- Service worker active at `/sw.js`.
- Offline emulation shows the backend-unavailable banner with the shell intact.
- Cache Storage contains **no `/api` entries**.

## 8. Docker — PASS

Production image built from `docker/Dockerfile` (multi-stage, Alpine runtime).

- Image size: **25,692,645 bytes (~24.5 MB)**.
- Final image contains **no node/npm/go** (`command -v` → absent).
- Container runs as **non-root** (`uid=10001`, user `datadeck`).
- Health/frontend/manifest/SW all **200**.
- Created a SQLite connection and executed a query inside the container.
- **Restart** persisted the connection; volume `/data` held
  `datadeck.db`/`-shm`/`-wal` + the target SQLite file.

## 9. Failure Scenarios (all actionable, exit 1)

| Scenario | Observed message |
|---|---|
| Missing encryption key | `datadeck: credential cipher: security: invalid encryption key: encryption key is not configured` |
| Invalid key length | `datadeck: credential cipher: security: invalid encryption key: expected 32 raw bytes or 64 hexadecimal characters` |
| Unwritable storage path | `datadeck: storage: create storage directory: mkdir /dev/null: not a directory` |
| Occupied port | `datadeck: http server: listen tcp 127.0.0.1:8080: bind: address already in use` |

All failures stop startup before serving, with no credential exposure.

## 10. Review Document

This file: `docs/reviews/m6-fresh-install.md`.

## Summary

| Area | Result |
|---|---|
| Packaged binary install | PASS |
| No source checkout / no Node runtime | PASS |
| First-run storage + migrations | PASS |
| Restart persistence + credential decrypt | PASS |
| PostgreSQL / MySQL / SQLite | PASS / PASS / PASS |
| PWA (manifest, SW, offline, cache safety) | PASS |
| Docker fresh install + persistence + non-root | PASS |
| Common startup errors actionable | PASS |

## Limitations

- Linux/Windows packaged binaries were not runtime-tested on their native
  platforms here (see `docs/release/platform-matrix.md`); this validation used
  the darwin/arm64 artifact.
- Docker validation used a single container and a named volume on Docker
  Desktop (macOS); no orchestration/network policy was exercised.
- Database credentials for the local test servers were environment-specific
  (PostgreSQL trust auth; MySQL root with an empty password).
