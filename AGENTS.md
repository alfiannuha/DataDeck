# AGENTS.md — DataDeck Development Rules

Permanent rules for humans and AI coding agents working on DataDeck. These rules
are mandatory unless an approved task states otherwise. They govern **how** work
is done; the `PRD.md` governs **what** is built.

---

## 1. Source of Truth Hierarchy

Resolve questions in this precedence order (highest first):

1. **The current approved task** (its explicit scope and acceptance criteria).
2. **`PRD.md`** — product and technical source of truth.
3. **`docs/architecture.md`** — system boundaries, responsibilities, lifecycles.
4. **ADRs** under `docs/decisions/` — binding architectural decisions.
5. **`docs/api-contract.md`** — API envelope, endpoints, error codes.
6. **`docs/security.md`** — mandatory security rules.
7. **`docs/testing-strategy.md`** — testing layers and gates.
8. **Existing implementation** — must conform to the above; it is never authority.

### Conflict Handling

- If any two sources conflict, **report the conflict** and the specific
  locations; **do not silently pick an interpretation**.
- Prefer the higher-ranked source only after stating the conflict, unless the
  conflict is already resolved by an accepted ADR or the task.
- If a task contradicts the PRD or an ADR, stop and report before implementing.
- Never "fix" a lower-ranked artifact by rewriting a higher-ranked one without
  approval. Record newly settled decisions as an ADR rather than editing rules in
  place.

---

## 2. Task Execution Workflow

Every task follows this sequence, in order:

```text
READ CONTEXT → VERIFY SCOPE → IMPLEMENT → TEST → LINT → TYPECHECK → SELF REVIEW → REPORT
```

1. **Read context.** Read the task, `PRD.md`, relevant `docs/`, ADRs, and the
   code the change touches. Understand the real flow before editing.
2. **Verify scope.** Confirm what is explicitly in and out of scope. Confirm
   acceptance criteria. If anything is ambiguous, ask or default to the laziest
   interpretation that satisfies the task, and note it.
3. **Implement.** Smallest change that satisfies the task and matches the
   architecture. No speculative code.
4. **Test.** Add/update tests per `docs/testing-strategy.md`; run the relevant
   suites.
5. **Lint.** Run available linters (backend: `go vet`; frontend: configured lint
   when present).
6. **Typecheck.** TypeScript `tsc --noEmit` for frontend; Go compiler for backend.
7. **Self review.** Re-read the diff for scope creep, secrets, and contract
   drift.
8. **Report.** Summarize files changed, validation performed, results, and risks.

### Stop Conditions

An agent **MUST stop** and report when any of these occur:

- The task is complete per its acceptance criteria.
- The task explicitly says not to start the next task (never auto-continue).
- A conflict exists between sources of truth (stop before implementing).
- A required change would alter architecture, an API contract, a security rule,
  or a selected technology without approval.
- A change requires adding a dependency that is not justified by the PRD/ADR.
- A blocking failure cannot be resolved within scope (failing tests, unavailable
  tooling, ambiguous requirements).

Never proceed to the next task automatically. Never fabricate "success" for
unrun validation.

---

## 3. Scope Control

Agents **MUST NOT**:

- implement future tasks or features not requested by the current task;
- perform unrelated refactors or drive-by formatting/reorganization;
- change architecture silently;
- change API contracts silently;
- replace the selected technologies (see `PRD.md` §3) without explicit approval;
- add dependencies without justification (see §11);
- suppress, skip, or delete failing tests to make CI green;
- commit secrets or generated local data (see §8, §12).

Anything outside the task's scope is reported, not implemented.

---

## 4. Architecture Rules

Follow `docs/architecture.md` and the ADRs. In particular:

- **Boundaries are fixed.** Frontend renders and owns transient UI state;
  backend owns persistence, credentials, pooling, execution, and introspection.
- **Layering.** HTTP handlers stay thin and delegate to `internal/database` and
  `internal/repository`. Repositories are the only code touching the embedded
  SQLite app store.
- **No silent layering violations.** Cross-layer shortcuts are reported and
  justified, not added ad hoc.
- **Deployment modes** (dev, Docker, single binary) and the monorepo layout are
  defined by PRD §5/§9 and ADR-001; do not restructure without an ADR.
- **State ownership** follows `docs/architecture.md` §13: backend is the source of
  truth for persisted state; frontend only for transient UI state.
- New architectural decisions require an ADR under `docs/decisions/` using the
  template in §10.

---

## 5. Backend Coding Rules

Backend is Go (`backend/`).

- Write **idiomatic Go**; `gofmt`/`go vet` clean.
- **Propagate `context.Context`** through call chains that perform I/O; never
  store contexts in structs or substitute `context.Background()` to dodge a
  deadline.
- **Explicit error handling.** Return errors; wrap with context (`%w`); do not
  ignore errors or panic for expected failures. Panics are reserved for truly
  unrecoverable program bugs and must still be recovered by middleware.
- **Small package boundaries** matching `internal/` (api, config, database,
  model, repository, security). Keep packages focused and dependency-light.
- **No global mutable application state** unless architecturally justified and
  documented (e.g., a `ConnectionManager` deliberately keyed by connection id).
  Prefer dependency injection.
- **Secrets stay behind the `security` package.** Credentials are decrypted only
  in memory, when needed.
- **Tests required** for business logic and all security-sensitive logic
  (encryption, credential handling, timeouts, truncation, error sanitization).
- Driver-specific behavior lives in the driver module; the generic execution and
  introspection pipeline operates on `database/sql`.

---

## 6. Frontend Coding Rules

Frontend is Next.js App Router + React + TypeScript (`frontend/`).

- **Strict TypeScript.** `tsconfig` strict mode stays on; do not weaken it.
- **Avoid `any`.** Use precise types, generics, or `unknown` with narrowing. Any
  `any` used deliberately must be justified in the PR/commit and kept minimal.
- **API types follow backend contracts.** Types mirror `docs/api-contract.md`
  (envelope, payloads, error codes). Do not invent fields.
- **Separate server state from client UI state.** Server data lives in TanStack
  Query; transient UI state (tabs, editor buffers, active connection) lives in
  Zustand. Do not duplicate server state into local stores.
- **All HTTP through the shared API client** (`lib/api-client.ts`). No ad-hoc
  `fetch` in components.
- **Never persist credentials** or connection profiles in browser storage.
- **Accessibility.** Use accessible primitives (Shadcn/Radix), keyboard
  navigability, focus management, and appropriate ARIA where needed.
- **Performance awareness.** Virtualize large datasets (TanStack Virtual/Table);
  avoid rendering unbounded lists/rows; keep rerenders scoped.

---

## 7. API Contract Rules

Follow `docs/api-contract.md`.

- Every JSON response uses the standard envelope
  (`success`, `data`, `error`, `meta`).
- Do not invent endpoints. Only the endpoints specified by the PRD exist; adding
  any requires an explicit requirement and doc update.
- **Never return credentials**; mask/omit secret fields.
- Error shape, codes, and HTTP status usage follow the contract; new error codes
  are documented before use.
- Serialization rules are binding: 64-bit integers as strings, `NULL` as `null`,
  `truncated: true` on the 50 MB cap.
- Contract changes require updating `api-contract.md` and, if architectural, an
  ADR, before or alongside the code change.

---

## 8. Security Rules

Mandatory; see `docs/security.md` for the full set and requirement levels
(MUST/SHOULD/MAY).

- **Never log credentials**, DSNs with userinfo, keys, or auth headers.
- **Never expose passwords through the API** (responses, errors, or history).
- **Never commit secrets.** No `.env`, keys, or local SQLite data in Git; use
  `.env.example` placeholders only.
- **Query execution requires a timeout.** Every query runs under
  `context.WithTimeout` (default 30s); no unbounded execution.
- **Preserve BIGINT precision.** Serialize 64-bit integers as strings.
- **Enforce the query result safety limit.** Cap payloads at 50 MB and set
  `truncated: true`; never return unbounded results.
- **Backend defaults to loopback binding** (`127.0.0.1`); wider binding is an
  explicit, documented opt-in.
- **Encryption changes require security review.** Any change to AES-256-GCM
  usage, key handling, nonce generation, or the credential boundary must be
  called out for review and covered by security tests.
- Sanitize errors and logs; treat history/snippets as sensitive local data.

---

## 9. Testing Requirements

Follow `docs/testing-strategy.md`.

- Add or update tests for every behavior change, including bug fixes (a
  regression test that fails before the fix).
- All existing tests must pass; failing tests block completion.
- Security-sensitive logic (encryption, timeouts, truncation, redaction) requires
  tests.
- Do not weaken or delete tests to force green. Do not mock away the behavior
  under test.
- Test tooling not fixed by the PRD is a recommendation, not a mandate; do not
  introduce heavy frameworks without justification.

---

## 10. Documentation Requirements

- Update docs when behavior, contracts, architecture, or security rules change —
  in the same change set as the code.
- Behavioral/contract changes update `docs/api-contract.md`; security changes
  update `docs/security.md`; architectural changes update `docs/architecture.md`
  and/or add an ADR.
- ADRs use the existing template: `# Title`, `## Status`, `## Context`,
  `## Decision`, `## Consequences`, `## Alternatives Considered`,
  `## Constraints`.
- Mark unresolved decisions as **Needs Validation** rather than guessing.
- Do not create documentation files that the task did not request.

---

## 11. Dependency Management

- Prefer the standard library and already-present dependencies before adding new
  ones (ladder: reuse → stdlib → native/framework → existing dep → new dep).
- **Justify every new dependency** in the PR/commit: what it replaces, why
  in-house is worse, and its footprint. Unjustified additions are rejected.
- Do not replace technologies selected in `PRD.md` §3 (`pgx`, `go-sql-driver/mysql`,
  `modernc.org/sqlite`, Chi, AES-256-GCM, CodeMirror 6, TanStack, Zustand, etc.).
- Pin versions for reproducibility; keep lockfiles committed for the frontend.
- Keep the backend pure-Go (`CGO_ENABLED=0` build must keep working); do not add
  CGO-dependent packages.
- Run available dependency/vulnerability checks; report known advisories rather
  than silently upgrading across major versions.

---

## 12. Git / Change Discipline

- Only commit when explicitly asked; never commit secrets or local data.
- Keep commits/PRs scoped to the task; no unrelated refactors or formatting.
- Inspect `git status` / diff before committing; stage only intended files.
- Reference the relevant PRD section, doc, or ADR in commit/PR descriptions.
- Do not rewrite history, force-push, or amend others' commits without explicit
  instruction.
- `.gitignore` must continue to exclude `.env*`, `data/`, SQLite files,
  `node_modules/`, and build output.

---

## 13. Definition of Done

A task is **NOT complete** unless all of the following hold:

1. Implementation matches the task and the PRD.
2. All task acceptance criteria pass.
3. Tests are added or updated for the change.
4. All existing tests pass.
5. Lint passes (backend `go vet`; frontend lint where configured).
6. Typecheck passes where applicable (frontend `tsc --noEmit`; backend compiles).
7. No secrets were introduced.
8. Documentation is updated when behavior/contracts change.
9. No unrelated refactoring was performed.
10. Changed files and remaining risks are reported.

Plus: report validation actually executed (commands and results) — never claim
validation that was not run.

---

## Quick Reference — Current Validation Commands

```bash
# Backend
cd backend && go build ./... && go vet ./... && go test ./...

# Frontend
cd frontend && npm run lint && npm run typecheck && npm run test && npm run build

# Root
make check
```

Frontend lint and tests are configured (ESLint flat config; Vitest + React
Testing Library). CI additionally runs backend PostgreSQL integration tests
(`go test -tags=integration ./... -p 1`) against a `postgres:17` service;
`-p 1` serializes packages because the database integration tests share one
server and some create/drop databases.

Frontend API types are generated from the committed OpenAPI document:

```bash
cd frontend && npm run generate:api   # src/types/generated/openapi.ts
```
