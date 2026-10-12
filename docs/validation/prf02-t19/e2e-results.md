# PRF02-T19 — Browser E2E Results

Runner: Playwright 1.63.0 / Chromium, 1 worker, against the single embedded binary
(`HOST=127.0.0.1 PORT=8099`), PostgreSQL 17.11. Command:
`E2E_BASE_URL=http://127.0.0.1:8099 E2E_PG_HOST=127.0.0.1 npx playwright test`.

| # | Spec | Workflow | Result |
|---|---|---|---|
| 1 | `release-flow.spec.ts` | shell boot, connection, query, result, PWA; unknown static asset → 404; API miss → JSON | PASS |
| 2 | `table-data.spec.ts` | α/β isolation, pagination, refresh via double-click | PASS |
| 3 | `table-structure.spec.ts` | α/β isolation, read-only flags, constraint text not HTML | PASS |
| 4 | `table-menu.spec.ts` | menu capabilities, α/β binding, copy, refresh, read-only reasons | PASS |
| 5 | `table-export.spec.ts` | full-table export (1000 rows despite page size 100), menu-target binding | PASS |
| 6 | `table-import.spec.ts` | CSV import bound to menu target; read-only object blocked | PASS |
| 7 | `database-menu.spec.ts` | reconnect stays bound to the menu's database (α/β isolation) | PASS |
| 8 | `database-script.spec.ts` | DB export α-scoped; restore into emptied target; non-empty target → 409 | PASS |
| 9 | `delete-database.spec.ts` | delete α only, β untouched; protected DB refused; token substitution → 400 | PASS |
| 10 | (same run) | total | **10 passed / 10** |

## Workflow coverage vs §39 checklist
- Connect → Browse → Filter → Sort: `table-data.spec.ts` (+ filter/sort assertions).
- Browse → Insert → Update → Delete: `table-data.spec.ts` CRUD section.
- Browse → Duplicate → Insert: `table-data.spec.ts`.
- Table → Structure: `table-structure.spec.ts`.
- Table → Export: `table-export.spec.ts`.
- Table → Import: `table-import.spec.ts`.
- Database → Reconnect: `database-menu.spec.ts`.
- Database → Export / Restore: `database-script.spec.ts`.
- Database → Delete disposable target: `delete-database.spec.ts`.

All specs assert **actual backend state** (follow-up API queries) in addition to
UI notifications where relevant (e.g., restored rows, β data after α delete).

## Multi-database isolation evidence (§24, release-blocking)
- `table-data.spec.ts`: α and β tabs retain distinct bindings; switching the
  Explorer selection never retargets an open tab.
- `table-menu.spec.ts` / `table-export.spec.ts`: table actions/export stay bound
  to the menu's database.
- `database-menu.spec.ts`: reconnect targets α while the Explorer is on β.
- `database-script.spec.ts`: export of α contains only α data; restore targets α;
  β unchanged.
- `delete-database.spec.ts`: α deleted, β still queries `BETA`; token for α
  rejected for β.

No α/β crossover observed → no BLOCKER.
