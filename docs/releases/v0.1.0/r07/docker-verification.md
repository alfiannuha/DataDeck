# DataDeck v0.1.0 — R07 Docker Verification

**NOT RUN / NOT APPLICABLE for this RC set.**

- No Docker daemon is available in the verification environment (`docker version`
  fails), and R06 did not produce a Docker image.
- The primary v0.1.0 distribution is the **standalone binary** (5 targets, verified).
  Docker is provided as a **packaging option**, not a required RC artifact.
- The Dockerfile was re-inspected: multi-stage, `golang:1.26.9-alpine` builder,
  `node:24-alpine` frontend, `alpine:3.20` runtime, **non-root** (uid 10001),
  `ENCRYPTION_KEY` not baked, OCI labels, exposes `8080`, runtime `ALLOW_REMOTE=1`.

## Networking note
Container binds `0.0.0.0:8080` by design; operators must not publish the port to
untrusted networks (no authentication). The native binary's loopback default is
unaffected.

## Disposition
Carried to R08 if a Docker image is part of the publication scope; otherwise recorded
as N/A. No image built or pushed in R07.
