# DataDeck v0.1.0 — R05 CI Verification

Remote: `origin` = https://github.com/alfiannuha/DataDeck.git
Branch: `release/v0.1.0-rc` → PR **#1** (base `main`).
Trigger: `push` + `pull_request` (test.yml, e2e.yml). `release.yml` requires a
`v*` tag, so **nothing was published**.

## Runs on the release source revision
| Workflow | Run ID | SHA | Conclusion | URL |
|---|---|---|---|---|
| CI | 38325379999 | `02d3ed5` | **success** | https://github.com/alfiannuha/DataDeck/actions/runs/38325379999 |
| Browser E2E (release validation) | 38325379969 | `02d3ed5` | **success** | https://github.com/alfiannuha/DataDeck/actions/runs/38325379969 |

`gh pr checks 1` → **4 passed, 0 failed** (Backend, Frontend, Backend Integration
PostgreSQL+MySQL, Browser E2E).

## Coverage
- CI `backend`: gofmt check, `go vet`, `go test`, `go test -race`, govulncheck,
  `CGO_ENABLED=0` build — includes the R04 CSRF middleware.
- CI `frontend`: `npm ci`, lint, typecheck, test, build, build:static, `npm audit
  --audit-level=critical` — includes the R05 e2e fix.
- CI `integration`: PostgreSQL 17 + MySQL 8, `-p 1`.
- E2E: builds the embedded binary, starts it, runs Playwright.

## R01-FIND-006 resolution
The prior CI gap (baseline unpushed) is **resolved**: CI is green on the release
branch tip. The merged-to-main commit will run the same workflows on `main`.

## Notes
- A documentation-only follow-up commit (this CI-verification + readiness report)
  triggers a further run on the new tip; it is verified before R05 is declared READY.
- No release tag was created; no artifact was published.
