# Local Development

## Prerequisites

- **Go** — `go.mod` declares `go 1.26.0` with `toolchain go1.26.9` (the toolchain
  is downloaded automatically by modern Go).
- **Node.js 24** with `npm` (lockfile-driven; CI uses Node 24).
- Optional: a reachable **PostgreSQL 17** and/or **MySQL 8** for integration tests.

## Repository layout

```text
datadeck/
├── backend/            Go daemon
│   ├── cmd/server/     entry point (--version handled before startup)
│   ├── internal/       api, config, database, dbscript, export, model,
│   │                   repository, security, storage, tableimport, version, web
│   └── docs/           generated Swagger (OpenAPI 3.1)
├── frontend/           Next.js App Router + React + TypeScript
│   └── src/            components, stores, lib, types (generated openapi)
├── docker/             Dockerfile
├── docs/               architecture, API, security, releases, validation
├── scripts/            build/validate scripts (build-embedded, build-release, …)
├── Makefile            top-level dev tasks
├── PRD.md / AGENTS.md  product + agent rules
```

## Setup

```bash
make setup          # npm install (frontend) + go mod download (backend)
```

## Run the backend

```bash
export ENCRYPTION_KEY="$(openssl rand -hex 32)"
make backend        # go run ./cmd/server  (defaults: 127.0.0.1:8080, ./data/datadeck.db)
```

## Run the frontend (dev server)

```bash
make frontend       # next dev on http://localhost:3000
```

Point the dev frontend at the daemon with `NEXT_PUBLIC_API_URL=http://localhost:8080`
(see `.env.example`). The single-binary build uses same-origin instead.

## Conventions

- Backend is idiomatic Go (`gofmt`/`go vet` clean); packages under `internal/`.
- Frontend is strict TypeScript; server state lives in TanStack Query, transient UI
  state in Zustand; all HTTP goes through `lib/api-client.ts`.
- `AGENTS.md` governs how work is done; `PRD.md` governs what is built.
