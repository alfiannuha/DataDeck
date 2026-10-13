# DataDeck v0.1.0 — R05 CI Verification

Remote: `origin` = https://github.com/alfiannuha/DataDeck.git.
Trigger: GitHub Actions on `push` and `pull_request` (`test.yml`, `e2e.yml`).
`release.yml` requires a `v*` tag, so **nothing was published**.

## Runs on the final release revision `f23e620`
| Workflow | Run ID | SHA | Conclusion | URL |
|---|---|---|---|---|
| CI | 38325725001 | `f23e620` | **success** | https://github.com/alfiannuha/DataDeck/actions/runs/38325725001 |
| Browser E2E (release validation) | 38325725035 | `f23e620` | **success** | https://github.com/alfiannuha/DataDeck/actions/runs/38325725035 |

Earlier green runs on the code baseline `02d3ed5`: CI `38325379999`, E2E
`38325379969` (both success).

## Coverage
- **CI backend**: gofmt check, `go vet`, `go test`, `go test -race`, govulncheck,
  `CGO_ENABLED=0` build — includes the R04 CSRF middleware.
- **CI frontend**: `npm ci`, lint, typecheck, test, build, build:static,
  `npm audit --audit-level=critical` — includes the R05 e2e fix.
- **CI integration**: PostgreSQL 17 + MySQL 8, `-p 1`.
- **E2E**: builds the embedded single binary, starts it, runs Playwright.

## R01-FIND-006 resolution
The prior CI gap (baseline unpushed, no CI) is **resolved**: CI is green on the
release revision.

## Branch / PR note (deviation, disclosed)
The intended flow was a feature branch + PR without moving `main`. Because the
working tree remained on `main` (the `release/v0.1.0-rc` branch was created but not
checked out), the follow-up commit was pushed to `main`, and PR #1
(`release/v0.1.0-rc` → `main`) auto-merged because `main` then contained the head.
Net effect: the release commits and green CI are on `origin/main` at `f23e620`. No
tag was created and nothing was published. If desired, `main` can be rewound with an
**explicit, separate authorization** (force-push) — not done here.

## Notes
- No release tag was created; no artifact was published.
