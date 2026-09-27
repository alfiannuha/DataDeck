# ADR-001: Monorepo Architecture

## Status

Accepted

## Context

DataDeck consists of a Go backend and a Next.js frontend that are distributed
together. The PRD defines a single `datadeck/` repository containing `backend/`,
`frontend/`, `docker/`, shared build tooling (`Makefile`), CI workflows
(`.github/workflows/`), and documentation. Distribution modes include a single
binary produced by embedding the static frontend export into the Go binary, which
requires both applications to be built from one repository and one release
pipeline.

## Decision

Use a single monorepo with `backend/` and `frontend/` as sibling applications:

- One repository is the unit of versioning and release.
- Each application keeps its own toolchain (`go.mod` vs. `package.json`) and is
  independently buildable.
- Cross-cutting concerns (Docker, Makefile, CI, docs) live at the repository
  root.
- The single-binary build consumes the frontend's static `out/` directory, so
  frontend and backend versions are released together.
- Contracts (API envelope, types) are documented in `docs/` and duplicated as
  typed models per side rather than via a shared generated package.

## Consequences

- Atomic changes across backend and frontend are a single commit/PR.
- A single release pipeline can build frontend, backend, Docker images, and the
  embedded binary together.
- `frontend/out` must exist before the backend embed build step runs; build order
  matters.
- Repository tooling must handle two language ecosystems; CI is split per app.
- No language-level shared type package; the API contract is the integration
  boundary.

## Alternatives Considered

- **Separate repositories.** Rejected: the single-binary distribution and
  coordinated release make cross-repo versioning error-prone.
- **Polyrepo with a shared contracts package.** Rejected as heavier than needed;
  PRD specifies one repository.
- **Nested Go module under a frontend-owned root.** Rejected: inverts the PRD
  layout and complicates `embed.FS` paths.

## Constraints

- Layout is fixed by PRD §5.
- Single-binary mode requires `frontend` production output to be generated before
  the Go embed build (`PRD §9.1`).
- Each subproject must remain independently runnable for the decoupled dev
  workflow (`PRD §8`).
- **Needs Validation:** whether shared generated API types (e.g., from the
  OpenAPI spec) are in scope; the PRD specifies Swagger generation but not a
  shared type pipeline.
