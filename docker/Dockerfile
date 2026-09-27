# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Stage 1 — build the static frontend export (Node is build-time only).
# ---------------------------------------------------------------------------
FROM node:24-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# DATADECK_STATIC_EXPORT=true + empty API base -> same-origin relative /api/v1
RUN DATADECK_STATIC_EXPORT=true NEXT_PUBLIC_API_URL= npm run build:static

# ---------------------------------------------------------------------------
# Stage 2 — compile the CGO-free single binary with the frontend embedded.
# ---------------------------------------------------------------------------
FROM golang:1.26.6-alpine AS backend
ARG VERSION=0.1.0-dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /src/frontend/out ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags "-s -w \
        -X github.com/datadeck/datadeck/backend/internal/version.Version=${VERSION} \
        -X github.com/datadeck/datadeck/backend/internal/version.Commit=${COMMIT} \
        -X github.com/datadeck/datadeck/backend/internal/version.BuildDate=${BUILD_DATE}" \
      -o /out/datadeck ./cmd/server

# ---------------------------------------------------------------------------
# Stage 3 — minimal runtime. Only the binary, CA certs and a healthcheck tool.
# No Node, no Go toolchain, no source tree, no package-manager caches.
# ---------------------------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates \
    && addgroup -S datadeck -g 10001 \
    && adduser -S -G datadeck -u 10001 datadeck \
    && mkdir -p /data && chown datadeck:datadeck /data

COPY --from=backend /out/datadeck /usr/local/bin/datadeck

# Container networking requires binding beyond loopback inside the container
# namespace; this is an explicit packaging choice (documented in
# docs/release/docker.md). Native binary defaults remain 127.0.0.1 + no opt-in.
ENV HOST=0.0.0.0 \
    ALLOW_REMOTE=1 \
    PORT=8080 \
    STORAGE_PATH=/data/datadeck.db \
    ENVIRONMENT=production \
    LOG_LEVEL=info

# ENCRYPTION_KEY is intentionally NOT set: the container must fail fast unless
# the operator supplies a key at runtime. Never bake a real key into the image.

USER datadeck
EXPOSE 8080
VOLUME ["/data"]

# Unauthenticated health endpoint; requires no credentials.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/api/v1/health" >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/datadeck"]
