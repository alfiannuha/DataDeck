# DataDeck v0.1.0 — R06 Docker Build Report

**Status: NOT RUN (BLOCKED — no Docker daemon in this environment).**

- `docker version` → not available.
- The `docker/Dockerfile` was inspected (R02/R04): multi-stage; builder
  `golang:1.26.9-alpine`; frontend stage `node:24-alpine`; runtime `alpine:3.20`;
  **non-root** user `datadeck` (uid 10001); minimal image (binary + CA certs);
  `ENCRYPTION_KEY` intentionally not baked in; OCI labels + `ARG VERSION=0.1.0` /
  `CHANNEL=release`; exposes `8080`; runtime `ENV HOST=0.0.0.0 ALLOW_REMOTE=1`.
- No image was built or pushed.

## Docker networking security (review)
- The container binds `0.0.0.0:8080` with `ALLOW_REMOTE=1` — an **explicit packaging
  choice** documented in `docs/release/docker.md`. DataDeck has no authentication, so
  operators **must not** publish the port to untrusted networks; use a private
  network or a loopback host port mapping. The localhost security model is **not**
  weakened (the native binary still defaults to loopback + refuses wider binds).

## Disposition
Docker is **not** a required v0.1.0 binary-release deliverable; the primary
distribution is the standalone binary. The Docker build is carried to R07 (verify
image builds) if Docker is available there. This does **not** block R06.
