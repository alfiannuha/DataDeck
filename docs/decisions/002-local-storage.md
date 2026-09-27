# ADR-002: Embedded SQLite for Local Application Storage

## Status

Accepted

## Context

DataDeck must remember connection profiles, query history, and saved queries
between runs without requiring a separate database server, installer, or
container. The product goal is a zero-config, ultra-light, local-first daemon.
The PRD defines an embedded SQLite database created automatically at
`~/.datadeck/datadeck.db` (or `./data/datadeck.db`) with a fixed schema
(`connection_profiles`, `query_history`, `saved_queries`) and uses
`modernc.org/sqlite` (pure Go, no CGO) for this store and as a target driver.

## Decision

Use embedded SQLite via `modernc.org/sqlite` as the application's local storage:

- The daemon opens/creates the database on startup and runs idempotent DDL.
- Access is confined to `internal/repository`.
- Passwords are stored only as AES-256-GCM ciphertext.
- Path is configurable via `STORAGE_PATH`, defaulting to `~/.datadeck/` (or
  `./data/`).
- Foreign keys enforce history cascade and saved-query null-on-delete.

## Consequences

- Zero external dependency and no CGO; the binary stays self-contained
  (`CGO_ENABLED=0`).
- Cross-platform file-based storage is trivial to back up or move.
- Single-writer semantics: fine for a single local daemon, unsuitable for
  concurrent multi-process writers.
- Schema migrations must be handled carefully as the app evolves (not specified
  by PRD beyond initial DDL).
- The store contains sensitive data and must be protected by filesystem
  permissions and never synced off-device by default.

## Alternatives Considered

- **External PostgreSQL/MySQL for app storage.** Rejected: contradicts
  zero-config local-first goal.
- **JSON/plain files.** Rejected: no transactional integrity, no foreign keys, no
  indexed history queries.
- **`mattn/go-sqlite3` (CGO).** Rejected: introduces CGO, breaking the static
  single-binary build.
- **bbolt/Badger embedded KV store.** Rejected: relational schema and SQL are
  already specified by the PRD.

## Constraints

- Must remain pure Go (`modernc.org/sqlite`) for `CGO_ENABLED=0` builds.
- Schema and indexes are defined by PRD §4.
- The app store MUST NOT be exposed through the API except as masked profile
  data and history/snippet payloads.
- **Needs Validation:** migration/versioning strategy for future schema changes;
  exact default path resolution (`~/.datadeck` vs. `./data`); file permission
  hardening expectations.
