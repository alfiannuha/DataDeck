# Contributing to DataDeck

## Source of Truth

The full, mandatory rules live in [AGENTS.md](AGENTS.md); this document is a
summary for humans. Resolve questions using the same hierarchy:

1. The current approved task
2. `PRD.md`
3. `docs/architecture.md`
4. ADRs under `docs/decisions/`
5. `docs/api-contract.md`
6. `docs/security.md`
7. `docs/testing-strategy.md`
8. Existing implementation (never authority)

If two sources conflict, **report the conflict** with locations; do not silently
pick an interpretation. If code and the PRD disagree, the PRD wins — propose a
doc/ADR change rather than diverging.

## Prerequisites

- Go 1.23+
- Node.js 20 LTS+ with `npm`

## Local Setup

Canonical workflow: **clone → setup → dev → test → build**.

```bash
make setup   # install frontend deps + download Go modules
make dev     # start the development environment
```

`make help` lists all commands. Underlying tools remain available directly.

## Before Committing

- Backend: `cd backend && go vet ./... && go test ./...`
- Frontend: `cd frontend && npm run lint && npm run typecheck && npm run test`
- Or via the root Makefile: `make lint && make typecheck && make test`
  (`make check` runs lint + typecheck)

Ensure no secrets are committed. `.env` files and local SQLite data are ignored;
use `.env.example` for placeholders only.

## Conventions

- Follow the existing structure and documented architectural boundaries.
- Keep backend handlers thin: business logic belongs under `internal/database`
  and `internal/repository`.
- The frontend must not persist credentials or call the backend outside the
  shared API client.
- Match `.editorconfig` for formatting (tabs for Go and Makefiles, 2 spaces
  elsewhere).
- Do not add endpoints, dependencies, or architectural layers that are not
  justified by the PRD or an accepted ADR.

## Pull Requests

- Keep changes scoped and explain how they were validated.
- Reference the relevant PRD section, doc, or ADR.
- Do not introduce product features without a corresponding requirement.
- New architectural decisions should be recorded as an ADR under
  `docs/decisions/` using the existing template (Status, Context, Decision,
  Consequences, Alternatives Considered, Constraints).
- A change is not done until it meets the Definition of Done in
  [AGENTS.md](AGENTS.md) §13 (tests, lint, typecheck, docs, no secrets, no
  unrelated refactoring).
