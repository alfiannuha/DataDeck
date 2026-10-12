# DataDeck v0.1.0 — R04 Supply-Chain Audit

## Dependency integrity
- `go.sum` committed; `go mod verify` → **all modules verified**.
- `frontend/package-lock.json` committed (lockfile v3); `npm ci` reproduces the tree
  (532 packages) and is what CI uses.
- No dependency upgrade performed in R04 except the targeted `npm audit fix`
  (`sharp`, `source-map-js`), which was re-validated.

## Build scripts
- `scripts/build-embedded.sh`, `build-release.sh`, `release-dry-run.sh`,
  `docker-validate.sh`, `check-version.sh` — inspected; no `curl | sh`, no remote
  fetch in build steps, no unexpected postinstall behavior.
- `package.json` has no `preinstall`/`postinstall`/`prepare` hooks; scripts are
  local (`next`, `tsc`, `eslint`, `vitest`, `openapi-typescript`, `playwright`).
- `release-dry-run.sh` writes archives/checksums/SBOM to the local `release/` dir
  only; it does **not** publish.

## CI / release workflow
- `test.yml`, `e2e.yml`: `push`/`pull_request` to `main`.
- `release.yml`: `push: tags: ["v*"]` + `workflow_dispatch`; contains
  `gh release create` → **tagging publishes**. R04 did not tag or push.
- Workflows use `permissions: contents: read` (except the release job's needed
  scope) and ephemeral service credentials; no third-party actions beyond
  `actions/*`.

## Docker
- `docker/Dockerfile`: multi-stage; runtime is `alpine:3.20`; runs as **non-root**
  user `datadeck` (uid 10001); minimal image (binary + CA certs); `ENCRYPTION_KEY`
  intentionally **not** baked in; OCI labels added in R02; no image pushed in R04.

## Secret scanning
- Pattern scan for private keys / AWS keys / API-token shapes / inline passwords and
  DSNs across tracked source and docs: **no matches** (values never printed).
- `.gitignore` excludes `.env*`, `data/`, `*.db*`, `node_modules/`, `release/`, and
  build output; `.env.example` holds placeholders only.

## Provenance
- No vendored or patched third-party source; modules resolved via the public module
  proxy with committed checksums. Go toolchain pinned (`go1.26.9`); Node pinned in
  CI (24) and locked.
- No release credentials are embedded in the repo; publishing credentials live only
  in GitHub Actions secrets (not inspected/printed).
