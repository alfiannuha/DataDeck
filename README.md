# DataDeck

DataDeck is a lightweight, local-first Web/PWA database GUI. It pairs a small Go
daemon (connection pooling, schema introspection, query execution, credential
encryption, local app state) with a Next.js/React interface designed for fast,
virtualized data browsing.

> **Status:** Feature-complete through M4 (product complete) and in M5
> production hardening. Connections, schema explorer, SQL editor, virtualized
> grid, history, saved queries, CSV/JSON export, and the PWA/offline shell are
> implemented. Packaging/distribution work is not part of the current milestone.

## Architecture Summary

- **Frontend** — Next.js (App Router), React, TypeScript, Tailwind CSS. Owns UI,
  editor, virtualized grid, and transient view state. Talks to the backend over
  HTTP/REST at `/api/v1` using a standard response envelope.
- **Backend** — Go daemon (Chi router target) that owns connection pooling,
  schema introspection, query execution, AES-256-GCM credential encryption, and
  the embedded SQLite app store.
- **Local Storage** — Embedded SQLite (`modernc.org/sqlite`) for connection
  profiles, query history, and saved queries.
- **Target Databases** — PostgreSQL (`jackc/pgx/v5`), MySQL
  (`go-sql-driver/mysql`), and SQLite (`modernc.org/sqlite`) via
  `database/sql`.

See [`docs/architecture.md`](docs/architecture.md) for the full design, and
[`PRD.md`](PRD.md) for the product source of truth.

## Repository Structure

```text
datadeck/
├── .github/workflows/   # CI workflows (placeholders)
├── backend/             # Go daemon
│   ├── cmd/server/      # Entry point
│   ├── internal/        # api, config, database, model, repository, security
│   ├── docs/            # Generated Swagger docs (placeholder)
│   ├── go.mod
│   └── Makefile
├── frontend/            # Next.js App Router app
│   └── src/app/         # Minimal App Router shell
├── docker/              # Docker assets (placeholder)
├── docs/                # Architecture, API, security, testing, ADRs
├── scripts/             # Developer scripts (placeholder)
├── Makefile
├── PRD.md
└── README.md
```

## Prerequisites

- Go **1.25+** (module minimum; PRD §8.1 says 1.23+). The repo pins a patched build toolchain (`toolchain go1.26.6`) for standard-library security fixes.
- Node.js **20 LTS+** with `npm` (PRD §8.1)
- Accessible PostgreSQL or MySQL server for future integration testing

## Development Workflow

Canonical workflow: **clone → setup → dev → test → build**. `make build`
produces a single self-contained executable (Go API + embedded PWA frontend) —
no Node.js runtime is required to run it.

```bash
git clone <repository-url>
cd datadeck
make setup     # install frontend deps + download Go modules
make dev       # start the development environment
make test      # run available tests
make build     # produce the single self-contained binary (embedded frontend)
```

Run `make help` to list every command.

| Command | Description |
|---|---|
| `make setup` | Install frontend deps and download Go modules |
| `make dev` | Start the development environment |
| `make backend` | Run the Go backend |
| `make frontend` | Run the Next.js dev server |
| `make test` | Run backend and frontend tests |
| `make lint` | Run backend `go vet` and frontend ESLint |
| `make typecheck` | Compile backend and run frontend `tsc --noEmit` |
| `make build` | Build the single self-contained binary (embedded frontend) at `backend/bin/datadeck` |
| `make clean` | Remove build artifacts |
| `make check` | Run `lint` + `typecheck` |

> **Development note:** `make dev` starts the frontend dev server; run
> `make backend` in a second terminal for the Go daemon
> (`http://127.0.0.1:8080`). A combined parallel dev runner is future work.

Individual tools remain available directly, e.g. `cd backend && go test ./...`
or `cd frontend && npm run typecheck`.

## Configuration & Safe Defaults

Copy [`.env.example`](.env.example) to `.env` and edit locally. Placeholders
only — never commit real secrets.

| Variable | Default | Notes |
|---|---|---|
| `HOST` | `127.0.0.1` | Loopback only. A non-loopback value is **refused** unless `ALLOW_REMOTE=1`. |
| `ALLOW_REMOTE` | *(unset)* | Explicit opt-in for wider binding; falsy/typo values fail safe. |
| `PORT` | `8080` | 1–65535. |
| `STORAGE_PATH` | `./data/datadeck.db` | Embedded SQLite app store; directory created `0700`, file narrowed to `0600`. |
| `ENCRYPTION_KEY` | *(required)* | 32 raw bytes or 64 hex chars. Absent/invalid **aborts startup** (no fallback). |
| `ENVIRONMENT` | `development` | `development` \| `test` \| `production`. |
| `LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error`. Logs never contain secrets. |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000,http://127.0.0.1:3000` | Explicit allowlist; no wildcard. |
| `NEXT_PUBLIC_API_URL` | `http://127.0.0.1:8080` | Frontend API base. Set `""` for future same-origin/single-binary. |

Enforced limits (safe defaults, tested):

| Limit | Value |
|---|---|
| Query timeout (default / maximum) | 30 s / 300 s |
| Request body | 1 MiB (`application/json` only) |
| Result payload cap | 50 MB, then `truncated: true` |
| Pagination | `page ≤ 1,000,000`, `page_size ≤ 200` (default 50) |
| Target DB pool | `MaxOpenConns 5`, `MaxIdleConns 2` (SQLite: 1) |
| Credential key | AES-256-GCM, fresh nonce per record |

## Documentation

- [PRD.md](PRD.md) — product requirements and technical blueprint
- [docs/architecture.md](docs/architecture.md) — system architecture and boundaries
- [docs/api-contract.md](docs/api-contract.md) — API envelope and endpoints
- [docs/security.md](docs/security.md) — mandatory security rules
- [docs/testing-strategy.md](docs/testing-strategy.md) — testing layers and gates
- [docs/decisions/](docs/decisions/) — architecture decision records

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
