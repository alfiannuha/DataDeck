# DataDeck v0.1.0 — R05 Frontend Regression

Commit `e81ca8e`. Working dir `frontend/`.

| Check | Command | Result |
|---|---|---|
| Clean install | `npm ci` | 532 packages, lockfile-reproducible |
| Lint | `npm run lint` | 0 problems |
| Typecheck | `npm run typecheck` | clean |
| Unit/component | `npm run test` | 51 files / **472 tests** PASS |
| Production build | `npm run build` | PASS |
| Static export | `npm run build:static` | PASS (`frontend/out`) |

## Browser E2E (Playwright 1.63/Chromium, single binary)
Full suite run **twice**: **10/10 both runs**. Specs: release-flow (shell/query/PWA,
static 404 + JSON API miss), table-data (α/β isolation, pagination, refresh),
table-structure, table-menu, table-export (1000 rows despite page 100),
table-import (CSV bound to menu target), database-menu (reconnect α-scoped),
database-script (export/restore α, non-empty → 409), delete-database (α only,
protected refused, token substitution rejected).

## Workspace / tab binding
Verified via unit + E2E: empty workspace on open; new Query tab explicit;
double-click opens Table Data; Structure tab; switching/closing preserves bindings;
Explorer selection never retargets an existing tab.

## R04 CSRF compatibility
Browser same-origin requests (GET/POST/multipart) pass the new CSRF middleware —
the full E2E suite (which drives all mutations from the embedded frontend) is green.

## R05 fix
`e2e/database-script.spec.ts` uses run-unique database names, removing the
suite-order flake seen in R04/R05 (R04-FIND-005); full suite deterministic green.
