# DataDeck v0.1.0 — R10 Requirements & Architecture Compliance

## Requirements traceability (PRD → implementation → evidence)
| Area | Status | Implementation | Evidence |
|---|---|---|---|
| Local-first single-binary GUI | IMPLEMENTED & VERIFIED | Go embed + static export | R06/R09 (binary runs, UI 200) |
| Connection profiles + encrypted credentials | IMPLEMENTED & VERIFIED | `internal/security`, repository | unit tests; R09 smoke |
| PostgreSQL (+ multi-database) | IMPLEMENTED & VERIFIED | pgx driver, discovery, pools | PRF01 + R05/R09 |
| MySQL | IMPLEMENTED & VERIFIED (restore disabled) | mysql driver | R05 integration; R09 blocked (server down) |
| SQLite | IMPLEMENTED & VERIFIED (delete disabled) | modernc sqlite | unit/round-trip; R09 API smoke |
| Schema Explorer | IMPLEMENTED & VERIFIED | schema-explorer/tree | unit + e2e |
| SQL editor / results / history / saved | IMPLEMENTED & VERIFIED | editor, query API | M1–M3 + e2e |
| Typed workspace tabs / empty state | IMPLEMENTED & VERIFIED | `useWorkspaceStore` | T01/T02 |
| Table Data (page/sort/filter) | IMPLEMENTED & VERIFIED | `table_data`, `table_filter` | T03–T06 + e2e |
| Row CRUD (insert/update/delete/duplicate) | IMPLEMENTED & VERIFIED | `mutation`, dialogs | T07–T11 + e2e |
| Table Structure | IMPLEMENTED & VERIFIED | `table_structure` | T13 + e2e |
| Table import/export | IMPLEMENTED & VERIFIED | `tableimport`, `export` | T14/T15 + e2e |
| DB context menu + reconnect | IMPLEMENTED & VERIFIED | T16 | unit + integration + e2e |
| DB export/restore | IMPLEMENTED & VERIFIED (MySQL restore N/A) | `dbscript` | T17 + e2e |
| Delete Database | IMPLEMENTED & VERIFIED (SQLite N/A) | `database_admin` | T18 + e2e |
| PWA / offline shell | IMPLEMENTED & VERIFIED | service worker | release-flow e2e |
| Cross-platform builds | IMPLEMENTED (native runtime macOS only) | CGO-free cross-compile | R06/R07/R09 |

Deferred (NOT v0.1.0 gaps): SSH tunneling, auth, public hosting, schema designer,
XLSX import, MySQL restore, SQLite delete, concurrent multi-connection engine.

## Architecture compliance
| Decision | Implemented | Note |
|---|---|---|
| Next.js + React + TS + Tailwind + Shadcn + CodeMirror 6 + TanStack + Zustand + PWA | yes | `frontend/` |
| Go + Chi + embedded SQLite + pgx/mysql/modernc-sqlite + AES-256-GCM | yes | `backend/` |
| Single binary, embedded frontend, CGO_ENABLED=0, localhost-first | yes | R06/R09 |
| Explicit DB binding; per-(connection,database) pools | yes | ADR-010/009 |
| No frontend raw SQL for structured APIs; identifiers validated+quoted, values bound | yes | T19/R04 |

No undocumented architectural deviation found. ADR-010 (incl. §11a/§12/§16/§17
amendments) matches implementation.
