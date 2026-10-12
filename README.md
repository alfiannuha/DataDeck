# DataDeck

**Version 0.1.0**

DataDeck is a lightweight, **local-first** database GUI. It pairs a small Go
daemon (connection pooling, schema introspection, query execution, credential
encryption, local app state) with a Next.js/React interface built for fast,
virtualized data browsing. It ships as a **single self-contained binary** with the
frontend embedded — no Node.js runtime is required to run it.

DataDeck binds to **localhost by default** and has **no general-purpose
authentication**; it is intended to run on your own machine, not on a public
network.

## Core capabilities

- Connection profile management with AES-256-GCM encrypted credentials.
- Target databases: **PostgreSQL**, **MySQL**, **SQLite**.
- PostgreSQL **multi-database discovery** (one server-level profile, many databases).
- Schema Explorer with lazy, per-database metadata loading.
- SQL editor (CodeMirror 6) with explicit query tabs, history, and saved queries.
- Virtualized, server-side paginated **Table Data** browsing with filtering and sorting.
- Safe single-row **CRUD** (insert / update / delete / duplicate) with optimistic concurrency.
- Read-only **Table Structure** inspection (columns, keys, indexes, constraints, foreign keys).
- **Table import/export** (CSV, XLSX, restricted SQL) and **database export/restore** (SQL, base tables).
- Database **reconnect** and a two-phase, policy-gated **Delete Database**.
- PWA/static frontend served from the embedded binary.

## Supported databases

| Engine | Connect | Browse/CRUD | Import/Export | Reconnect | Restore | Delete database |
|---|---|---|---|---|---|---|
| PostgreSQL | Yes | Yes | Yes | Yes | Yes | Yes |
| MySQL | Yes | Yes | Yes | Yes | **No** (disabled) | Yes |
| SQLite | Yes | Yes | Yes | Yes | Yes | **No** (disabled) |

See [docs/reference/database-capabilities.md](docs/reference/database-capabilities.md).

## Supported platforms

Prebuilt single binaries are produced for: macOS (arm64, amd64), Linux (amd64,
arm64), and Windows (amd64). See
[docs/reference/supported-platforms.md](docs/reference/supported-platforms.md).

## Installation

Standalone binaries are named `datadeck_<version>_<os>_<arch>[.exe]`. Release
artifacts are **not yet published** (publishing is a later release step); build
from source for now: [docs/development/building.md](docs/development/building.md).

## Quick start

```bash
# 1. Generate an encryption key (32 raw bytes / 64 hex chars) and keep it safe.
export ENCRYPTION_KEY="$(openssl rand -hex 32)"

# 2. Run the daemon (defaults: http://127.0.0.1:8080, ./data/datadeck.db).
./datadeck

# 3. Open http://127.0.0.1:8080 in your browser, create a connection, and browse.
```

Full walkthrough: [docs/getting-started/quick-start.md](docs/getting-started/quick-start.md).

## Security model

Local-first; loopback binding by default (`ALLOW_REMOTE=1` required to bind
wider, which is unsupported for public use); stored credentials encrypted with a
required `ENCRYPTION_KEY`; no authentication layer. See
[docs/security/security-guide.md](docs/security/security-guide.md).

## Known limitations

No SSH tunneling, no authentication, no public hosting, no schema designer, no
MySQL restore, no SQLite database deletion, table import is INSERT-only, and
database export covers base tables only. See
[docs/reference/limitations.md](docs/reference/limitations.md).

## Documentation

- Getting started: [installation](docs/getting-started/installation.md) · [quick start](docs/getting-started/quick-start.md) · [configuration](docs/getting-started/configuration.md)
- User guide: [connections](docs/user-guide/connections.md) · [database explorer](docs/user-guide/database-explorer.md) · [query editor](docs/user-guide/query-editor.md) · [table data](docs/user-guide/table-data.md) · [row CRUD](docs/user-guide/row-crud.md) · [table structure](docs/user-guide/table-structure.md) · [table import/export](docs/user-guide/table-import-export.md) · [database management](docs/user-guide/database-management.md)
- Reference: [capabilities](docs/reference/database-capabilities.md) · [platforms](docs/reference/supported-platforms.md) · [limitations](docs/reference/limitations.md) · [troubleshooting](docs/reference/troubleshooting.md)
- Security: [security guide](docs/security/security-guide.md)
- Development: [local development](docs/development/local-development.md) · [testing](docs/development/testing.md) · [building](docs/development/building.md)
- Architecture & API: [architecture](docs/architecture.md) · [API contract](docs/api-contract.md) · [OpenAPI](backend/docs/swagger.json)
- Release: [v0.1.0 release notes](docs/releases/v0.1.0/release-notes.md) · [CHANGELOG](CHANGELOG.md)

## Development

```bash
make setup    # install frontend deps + download Go modules
make test     # backend + frontend tests
make lint     # go vet + eslint
make build    # single binary with the embedded frontend
```

See [docs/development/local-development.md](docs/development/local-development.md).

## Contributing

Contributions follow [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md).
`PRD.md` is the product source of truth.
