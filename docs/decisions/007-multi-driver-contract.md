# ADR-007: Multi-Driver Contract & Capability Model

## Status

Accepted

## Context

DataDeck currently ships a PostgreSQL connector only, but M3 expands to MySQL
and target SQLite. The existing `database.Connector` interface, the schema
model, and error handling had implicit PostgreSQL assumptions: the handler
imported `pgconn` to classify SQL errors, the schema model assumed a
database → schema → table hierarchy, and there was no representation of
driver-level feature differences. These would leak into (or block) additional
engines.

## Decision

Establish a small, capability-focused, database-neutral contract:

- **`database.Connector`** exposes only what the shared pipeline uses:
  `Name()`, `Open()`, `Ping()`, `Introspect()`, `Execute()`, and
  `Capabilities()`. No giant interface; new needs require a new decision.
- **`model.Capabilities`** carries the differences that matter now:
  `Schemas`, `ForeignKeys`, `Indexes`, `SSL`, `SSH`, `Dialect`,
  `IdentifierQuote`.
- **`model.Database`** supports variable hierarchy depth: engines with schemas
  populate `Schemas`; engines without a schema level populate `Tables` directly.
  No fake `public` schema is created.
- **`database.SQLError`** normalizes driver statement errors
  (`Driver`, native `Code`, safe `Message`, `Position`); connection failures use
  the `database.ErrConnection` sentinel. Handlers no longer import `pgconn`.
- PostgreSQL remains the reference implementation and its behavior is unchanged.

## Consequences

- Handlers, the query engine, the schema models, and the manager are
  driver-neutral; only the connector packages know engine specifics.
- The frontend explorer renders either schemas or database-level tables from the
  same response shape.
- Error classification is centralized (`SQLError`/`ErrConnection`), so future
  drivers only translate their native errors.
- A new engine is added by implementing `Connector` + registering it with the
  manager; no core changes.
- Capabilities are currently exposed on the Go `Connector`/`Manager`; they are
  not yet surfaced through the API (see Constraints).

## Alternatives Considered

- **One large "Database" interface** with query/introspect/transaction/dialect
  methods: rejected as speculative and coupling.
- **Driver-specific result structs**: rejected; the neutral
  `model.QueryResult` already covers columns/rows/rows_affected/time/truncated.
- **Forcing every engine into database → schema → table** (synthetic schema):
  rejected; it would fake concepts, especially for SQLite.
- **Leaving `pgconn` in handlers**: rejected; it made the API layer
  PostgreSQL-specific.

## Constraints

- Connector implementations MUST NOT return driver-specific result shapes.
- The schema model MUST NOT invent schemas to satisfy a fixed depth.
- PostgreSQL behavior and tests MUST remain unchanged.
- MySQL and target SQLite MUST NOT be implemented in this task.
- **Needs Validation:** whether/when to expose capabilities to the frontend
  (e.g. for MySQL identifier quoting and hiding the schema level). Until then
  the frontend assumes PostgreSQL quoting and hierarchy when the response has
  schemas.
