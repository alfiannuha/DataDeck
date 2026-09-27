# ADR-005: Query Execution Architecture

## Status

Accepted

## Context

Users run arbitrary SQL against target databases and expect fast, resilient
results. The PRD requires: timeout-enforced execution via `context.WithTimeout`
(default 30s), dynamic scanning of arbitrary row shapes, execution duration in
milliseconds, NULL handling, clean binary/UUID serialization, preservation of
64-bit integer precision by serializing BIGINT as strings, a 50 MB result cap
with a `truncated` flag, and an audit record in `query_history` for every
execution (PRD §2.1, §6.2, §11).

## Decision

Implement a synchronous query execution pipeline behind `POST /api/v1/query/execute`:

1. Resolve the connection pool from `ConnectionManager`.
2. Create `context.WithTimeout(ctx, timeout)` (default 30s; client may override
   within a server-enforced maximum).
3. Execute via `QueryContext` (rows) or `ExecContext` (DML).
4. Build dynamic typed scan pointers for arbitrary column count/types.
5. Scan rows, converting `NULL` → `null`, BIGINT/64-bit integers → JSON strings,
   and binary/UUID → JSON-safe values.
6. Accumulate result while measuring payload size; stop and set
   `truncated: true` at the 50 MB cap.
7. Measure `execution_time_ms`.
8. Persist an audit row to `query_history` (`SUCCESS`/`ERROR`, sql text, status,
   duration, rows affected, error message).
9. Return the standard envelope.

## Consequences

- One generic pipeline serves all drivers and arbitrary result schemas.
- Precision loss in JavaScript is avoided by emitting 64-bit integers as strings,
  at the cost of clients needing to know which columns are big integers (the
  `columns[].type` metadata aids this).
- Synchronous execution keeps the API simple; the PRD mentions SSE in its
  architecture diagram but specifies no streaming endpoint. Large results are
  bounded by truncation rather than streamed.
- Every execution writes to the app store, adding a small write cost per query
  and growing history over time.
- Timeout and truncation protect server/client memory and pool availability.
- Auditing includes failed queries; the `error_message` column must be sanitized
  of secrets.

## Alternatives Considered

- **Streaming/SSE results.** Rejected for now: PRD §6.1/§6.2 define a synchronous
  endpoint; the diagram's SSE mention is unexploited. **Needs Validation.**
- **Returning all rows unbounded.** Rejected: violates the 50 MB protection
  requirement.
- **Server-side pagination / cursor.** Rejected for now: no pagination contract
  is specified by the PRD. **Needs Validation.**
- **Serializing BIGINT as JSON numbers.** Rejected: loses precision beyond
  2^53-1, which PRD §11.4 explicitly forbids.
- **`rows.Columns()` typed scanning with fixed structs.** Rejected: user SQL is
  arbitrary and schema is unknown at compile time.

## Constraints

- Default timeout MUST be 30s and MUST be context-enforced (PRD §11.3).
- Results exceeding 50 MB MUST be truncated with `truncated: true` (PRD §11.2).
- 64-bit integers MUST be serialized as strings (PRD §11.4).
- Every execution MUST be recorded to `query_history`.
- Errors MUST be returned in the standard envelope with stable codes.
- **Needs Validation:** maximum allowed `timeout_seconds`; truncation measurement
  point (pre- vs. post-JSON encoding); uniform `rows_affected` semantics for DML
  across drivers; exact binary/UUID encoding; whether SSE/pagination will replace
  the synchronous contract.
