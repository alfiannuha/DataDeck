# PRF02-T20 — Architecture & ADR Compliance

Verified at HEAD `e3f6c20`. Implementation inspected in source; no unexplained
deviation found.

| ADR-010 decision | Implementation reference | Verdict |
|---|---|---|
| Typed `WorkspaceTab` union (query/table-data/table-structure) | `store/useWorkspaceStore.ts` | CONSISTENT |
| Tab-local connection/database binding; never explorer/tab-driven | stores + `resolveTargetDatabase` | CONSISTENT |
| QueryTab SQL/dirty preservation across explorer/reconnect/refresh | `useWorkspaceStore`, `schema-explorer.tsx` | CONSISTENT |
| Empty workspace; no auto QueryTab | `workspace-empty-state.tsx`, open actions | CONSISTENT |
| Backend structured TableData API (no frontend raw SELECT) | `handler/table_data.go` | CONSISTENT |
| Metadata validation + dialect-aware identifier quoting | `quoteIdentifier`, `resolveTargetDatabase` | CONSISTENT |
| RowIdentity policy: declared PK, composite, **no unique/rowid/ctid** | `table_data.go` (`hasPK`, comment L201) | CONSISTENT |
| Optimistic concurrency + affected-rows 0/1/>1 | `mutation.go` L176-188 | CONSISTENT |
| Table Structure dedicated endpoint (§12; Open Question #6 resolved) | `GET /connections/{id}/table-structure`; `table_structure.go` | CONSISTENT |
| `/schemas` stays lightweight (tree only) | `postgres_schema.go` (no per-table constraint/definition) | CONSISTENT |
| Import scope CSV+SQL (XLSX import deferred) | `tableimport`, T15 docs | CONSISTENT |
| Database export base-tables-only; no external tooling | `dbscript`, ADR §16 | CONSISTENT |
| Reconnect database-scoped, fail-closed, serialized, no replay | `Manager.Reconnect`, ADR §11a | CONSISTENT |
| Delete Database two-phase, protected policy, no FORCE, SQLite disabled | `database_admin.go`, `database_delete.go`, ADR §17 | CONSISTENT |
| No circular package deps (`dbscript`→`model` only; `database` never imports `dbscript`/`export`) | imports inspected | CONSISTENT |

## ADR §7 special checks
- **Table Structure API:** dedicated route present (`router.go:80`), documented in `api-contract.md` §9 and OpenAPI; `/schemas` unchanged and lightweight. **Resolved.**
- **RowIdentity:** PK-only enforced; no unique-index fallback (source comment + tests).
- **Mutation concurrency:** affected-rows safety implemented and tested.
- **Destructive safeguards:** confirmed in `database_delete.go` (token binding, protected list, no `WITH (FORCE)`, verification).

No contradiction requiring an ADR rewrite. Any documented scope reductions (MySQL
restore, SQLite delete, XLSX import) are already recorded as ADR amendments.
