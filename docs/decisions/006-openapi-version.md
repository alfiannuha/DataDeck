# ADR-006: OpenAPI 3.1.0 for API Documentation

## Status

Accepted

## Context

PRD §3 selects "OpenAPI 3.0 / Swagger (`swaggo/swag`)", while PRD §8.2 installs
`swaggo/swag` v1, whose only output is Swagger **2.0**. The two PRD statements
are inconsistent, and the M1 exit review recorded this as finding F3 (OpenAPI
version decision).

The backend already generates a machine-readable document from handler
annotations. M2 (frontend) will consume that document for type/client
generation, so the version must be explicit and stable.

The implementation uses `swaggo/swag/v2` with `--v3.1`, which emits
**OpenAPI 3.1.0** (swag v2 has no explicit OpenAPI 3.0 mode).

## Decision

Emit **OpenAPI 3.1.0** from `swaggo/swag/v2 --v3.1`, generated from Go handler
annotations and committed as:

- `backend/docs/swagger.json`
- `backend/docs/swagger.yaml`

Regenerate with `make swagger` (backend). Do **not** commit a generated
`docs.go`, and do **not** add `swaggo/swag` as a runtime dependency; the tool is
invoked on demand via `go run`. The committed spec is guarded by contract tests
(`internal/api/contract_test.go`).

## Consequences

- One accurate, contract-tested spec is the single source for M2 code generation.
- No runtime dependency, no Go-version bump, and CGO-free builds are unaffected
  (`go 1.23.0` retained).
- OpenAPI 3.1 semantics differ from 3.0: JSON Schema 2020-12 (`examples`
  keyword, `nullable` expressed via type unions, `webhooks`). Consumers must use
  a 3.1-capable toolchain.
- PRD §3's "OpenAPI 3.0" wording is superseded by this ADR rather than edited in
  place, per AGENTS §1/§10.
- Regeneration needs network access (`go run ...@latest`).

## Compatibility Considerations

- 3.1 is a superset of the 3.0 feature set for practical API description; most
  modern generators support it, but older 3.0-only tools may need an upgrade.
- Response schemas: success uses `response.Envelope` with a per-endpoint `data`
  override; failures use `response.ErrorEnvelope` (`data` always `null`).
- Request schemas mark required fields (`binding:"required"`); `password`
  remains a write-only request property and never appears in any response schema.
- The document is plain JSON/YAML and is asserted to parse and to match the
  implemented route set.

## Frontend Code Generation Implications

- Use an OpenAPI 3.1-capable generator (for example `openapi-typescript`,
  `orval`, or `openapi-generator`) and pin it.
- Types must mirror the envelope (`success`, `data`, `error`, `meta`),
  the error codes, and the required/optional request fields.
- Confirm the chosen generator's 3.1 support before wiring M2 codegen.

## Alternatives Considered

- **`swaggo/swag` v1 (Swagger 2.0):** matches the PRD §8.2 command but is not
  OpenAPI 3.x and uses an older converter path.
- **Hand-written OpenAPI 3.0 document:** exact version control but drifts from
  code and loses generation/annotation benefits.
- **Other spec toolchains (Stoplight, etc.):** not the PRD-selected tool.

## Constraints

- Must not introduce a runtime `swaggo` dependency or break the pure-Go,
  Go 1.23, CGO-free build (it does not).
- Must be regenerated whenever handlers or DTOs change; the contract test fails
  on drift.
- **Finalized in M3-T00:** OpenAPI **3.1.0** is the project contract version.
  No technical incompatibility has appeared. Frontend type generation uses
  `openapi-typescript` v7 (3.1-capable; verified generating
  `src/types/generated/openapi.ts` from the committed spec). The former
  "Needs Validation" note on version acceptance is resolved.
