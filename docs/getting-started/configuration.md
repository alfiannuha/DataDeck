# Configuration

DataDeck is configured through environment variables. Placeholders live in
[`.env.example`](../../.env.example); copy it to `.env` for local development and
**never commit real secrets**.

## Required

| Variable | Description |
|---|---|
| `ENCRYPTION_KEY` | AES-256-GCM key for stored credentials. Exactly **32 raw bytes** or **64 hexadecimal characters**. Required at startup; an absent/invalid key aborts startup. There is no insecure fallback. |

Generate one and keep it stable:

```bash
export ENCRYPTION_KEY="$(openssl rand -hex 32)"
```

> **Losing this key makes stored connection passwords undecryptable.** There is no
> key-rotation or recovery flow in v0.1.0 (see the [security guide](../security/security-guide.md)).

## Network

| Variable | Default | Description |
|---|---|---|
| `HOST` | `127.0.0.1` | Bind address. Loopback by default. |
| `PORT` | `8080` | HTTP port. |
| `ALLOW_REMOTE` | _(unset)_ | Must be `1` to bind a non-loopback `HOST`. Startup **refuses** a non-loopback host without it (no authentication exists). Binding beyond loopback is **not supported for public use**. |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000,http://127.0.0.1:3000` | Comma-separated browser origins allowed by CORS (explicit allowlist). |

## Storage

| Variable | Default | Description |
|---|---|---|
| `STORAGE_PATH` | `./data/datadeck.db` | Embedded SQLite app store (profiles, history, saved queries). The parent directory is created `0700`; the store file is narrowed to `0600`. |

## Runtime

| Variable | Default | Description |
|---|---|---|
| `ENVIRONMENT` | `development` | `development` \| `test` \| `production`. Affects validation/logging expectations. |
| `LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error`. |

## Frontend (build-time)

| Variable | Description |
|---|---|
| `NEXT_PUBLIC_API_URL` | Development: point at the daemon (`http://localhost:8080`). The embedded single-binary build uses same-origin (`""`). |
| `NEXT_PUBLIC_DATADECK_VERSION` / `_CHANNEL` / `_COMMIT` | Optional build metadata for the embedded frontend; defaults to `0.1.0` / `development` / `unknown`. |

## Example (development)

```bash
export ENCRYPTION_KEY="$(openssl rand -hex 32)"
export HOST=127.0.0.1
export PORT=8080
export STORAGE_PATH=./data/datadeck.db
export ENVIRONMENT=development
export LOG_LEVEL=info
./datadeck
```
