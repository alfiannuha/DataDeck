# ADR-004: Database Driver Abstraction

## Status

Accepted

## Context

DataDeck connects to multiple target database engines. The PRD chooses
`jackc/pgx/v5` (PostgreSQL), `go-sql-driver/mysql` (MySQL), and
`modernc.org/sqlite` (SQLite), and specifies a single `ConnectionManager` pool, a
single query execution endpoint, and a single schema introspection endpoint that
must work across engines. The pool is keyed by connection id in a `sync.Map` of
`*sql.DB` (PRD §2.1, §5).

## Decision

Abstract target databases behind Go's `database/sql` interface:

- The pool manager stores `*sql.DB` handles keyed by connection id.
- A driver-specific module (`internal/database/postgres.go`, `mysql.go`,
  `sqlite.go`) supplies engine-specific behavior: DSN construction, dialect
  details, and catalog queries.
- Query execution and result scanning are generic and operate on `sql.Rows`; no
  per-driver result pipeline unless required for type fidelity.
- Schema introspection returns a normalized nested model (`internal/model`),
  with each driver implementing its own catalog queries.
- Drivers are selected from the profile's `driver` field (constrained by the
  SQLite schema to `postgres | mysql | sqlite`).

## Consequences

- One execution and introspection pipeline serves all engines, minimizing
  duplicated handler logic.
- Adding an engine means adding one driver module and a DSN mapping, not new
  endpoints.
- `database/sql` adds an abstraction layer over `pgx`'s native interface; this is
  acceptable for a GUI client and keeps uniformity.
- Some engine-specific type/serialization quirks still need handling (BIGINT as
  string, JSON/JB, binary/UUID, NULL), possibly per driver.
- Introspection capability may differ per engine; the model must tolerate partial
  metadata.

## Alternatives Considered

- **Per-driver handlers and pools.** Rejected: duplicates query/schema logic and
  breaks the uniform API envelope.
- **Use pgx native pool directly (skip `database/sql`).** Rejected for the
  generic path: it would make the cross-engine pipeline non-uniform.
- **An interface with only one implementation / speculative driver registry.**
  Rejected (YAGNI): implement the drivers the PRD names; add abstraction only
  where they genuinely differ.
- **ORM layer.** Rejected: DataDeck executes raw user SQL; an ORM adds weight and
  no value.

## Constraints

- Must use the exact drivers named by the PRD.
- Pool keyed by connection id; activation pings with a 5-second timeout.
- SQLite target support is only partially specified by the PRD
  (`internal/database/sqlite.go` is a "pragma reader", and `'sqlite'` is an
  allowed driver value, but execution/introspection for SQLite is not detailed).
  **Needs Validation.**
- **Needs Validation:** pool eviction/LRU capacity, per-driver pooling options,
  and SSH tunnel integration point relative to DSN construction.
