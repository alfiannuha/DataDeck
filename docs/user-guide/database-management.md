# Database Management

Actions on a **database** node in the Schema Explorer's context menu: **New
Query**, **Refresh Database**, **Reconnect**, **Export SQL**, **Import SQL
(Restore)**, **Copy Database Name**, **Delete Database**.

## New Query

Opens a new Query tab bound to that database. It does not execute anything and does
not overwrite an existing tab.

## Refresh Database

Re-fetches that database's discovery and schema/table metadata. It is a metadata
refresh only — it does not reconnect, reload tab data, or rebind tabs.

## Reconnect

Replaces the connection pool for **that database** with a fresh, health-checked one.

- **Database-scoped:** only the target `(connection, database)` pool is replaced;
  other databases' pools and their in-flight queries are untouched.
- **Fail-closed:** the replacement is established and health-checked before it is
  published; if it fails, the existing pool is kept and a sanitized error is shown.
- **No SQL is executed and no mutation is replayed.**
- Tabs are never closed, rebound, or auto-executed; unsaved editor content is
  preserved.
- Supported for PostgreSQL, MySQL, and SQLite (SQLite reopens the same file; it is
  never deleted).

## Export SQL

Streams a dialect SQL script for the database's **base tables**. Modes:

- **Schema + Data** (default) · **Schema Only** · **Data Only**.

Coverage: base tables — columns/types/NOT NULL/DEFAULT/identity/generated, PRIMARY
KEY, UNIQUE, CHECK, indexes, FOREIGN KEY. **Excluded and named in the artifact
header:** views, materialized views, sequences, triggers, functions/procedures,
extensions, and expression/partial indexes. This is a **SQL export of base tables**,
not a full database backup.

Ordering is dependency-free: tables are created without foreign keys, data is
loaded, then indexes and foreign keys are added (so circular/self-referencing FKs
work). The artifact carries a `-- datadeck:export` manifest.

## Import SQL (Restore)

Restores a **DataDeck SQL export artifact** into the target database.

- **Only DataDeck artifacts are accepted.** The manifest must match the target
  engine and database exactly, or the restore is rejected.
- Statements are validated against a strict allowlist (`CREATE TABLE`,
  `CREATE [UNIQUE] INDEX`, `ALTER TABLE … ADD CONSTRAINT`, `INSERT`, transaction
  control). `CREATE TABLE … AS SELECT` and cross-database / dangerous statements
  are refused.
- **The target must be empty** (no base tables). Restore never drops, truncates, or
  replaces existing objects — if the database is not empty, it is refused (preflight
  conflict).
- Applied in **one all-or-nothing transaction** on **PostgreSQL and SQLite**.
- **MySQL restore is disabled** (`RESTORE_UNSUPPORTED`): MySQL DDL implicitly
  commits, so a full-rollback guarantee cannot be made.

Flow: **upload → validate/preflight → explicit confirmation → restore → verified.**

## Copy Database Name

Copies the canonical database name to the clipboard (no database request).

## Delete Database

> **Destructive and irreversible.** Protected databases cannot be deleted, and
> testing against production data is strongly discouraged.

Supported for **PostgreSQL** and **MySQL**. **SQLite deletion is disabled**
(a database is a local file; safe file deletion needs a separate design).

Two-phase, fail-closed workflow:

1. **Preflight** validates the target and returns a short-lived, single-use
   confirmation token bound to the exact connection + database + engine. **No DROP
   runs.**
2. **Execute** requires a **typed exact database name** (case-sensitive) and an
   **acknowledgment**, re-validates the token/target/protection, and runs
   `DROP DATABASE <quoted>`.

Guarantees and limits:

- **Protected** databases are refused: PostgreSQL `postgres`/`template0`/`template1`;
  MySQL `mysql`/`information_schema`/`performance_schema`/`sys`.
- **Active operations** on the target block deletion; sessions are **never
  force-terminated**. No `WITH (FORCE)`, no `CASCADE`.
- PostgreSQL runs `DROP` on a **maintenance connection** (never the target).
- **No automatic retry.** If the outcome cannot be verified (e.g. network loss),
  DataDeck reports an **unknown outcome** — verify the server before retrying.
- Success is reported only after the database is **verified gone**.
- The **connection profile is not deleted**, and saved queries/history are not
  removed. Tabs are preserved; a tab pointing at a deleted database simply fails on
  its next request and is never rebound to another database.

See the [security guide](../security/security-guide.md) for the destructive-operation
safety model.
