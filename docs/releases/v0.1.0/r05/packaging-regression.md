# DataDeck v0.1.0 — R05 Packaging Regression

| Check | Command | Result |
|---|---|---|
| Static export | `npm run build:static` | PASS (`frontend/out`) |
| Embedded assets | copied into `backend/internal/web/dist` | PASS |
| Single binary | `CGO_ENABLED=0 go build ./cmd/server` | PASS (~32 MB) |
| CLI version | `./datadeck --version` | `datadeck 0.1.0 (commit …, built …, channel …)` |
| Health version | `GET /api/v1/health` | `{status, version:"0.1.0", channel}` |
| Loopback listener | `lsof` | `127.0.0.1:PORT (LISTEN)` |
| ENCRYPTION_KEY behavior | missing/invalid → startup aborts | PASS |
| Graceful shutdown | `SIGTERM` | process exits cleanly |
| Cross-compile | darwin/{amd64,arm64}, linux/{amd64,arm64}, windows/amd64 | all compile PASS |

## Native runtime vs cross-compile
Native runtime smoke: **macOS arm64 only**. Other targets are **compile-only**
(successful cross-compilation ≠ native runtime validation). Native smoke on
macOS-amd64/Linux/Windows: **NOT RUN** (residual).

## Docker
`docker/Dockerfile` inspected: multi-stage; runtime `alpine:3.20`; **non-root**
(uid 10001); minimal (binary + CA certs); `ENCRYPTION_KEY` not baked in; OCI labels
+ `ARG VERSION=0.1.0`/`CHANNEL=release`; exposes 8080. Docker **build not executed**
(no Docker daemon in this environment) → **NOT RUN / BLOCKED**; carried to R06. No
image published.
