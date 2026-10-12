# DataDeck v0.1.0 — R02 Version Contract

## Identity
```text
Product        : DataDeck
App version    : 0.1.0
Git tag        : v0.1.0        (created by R08, NOT here)
Commit SHA     : actual release source revision (full in machine metadata)
Build channel  : development | rc | release
```

## Contract
```ts
type BuildInfo = {
  version: string;                         // always "0.1.0" across channels
  commit: string;                          // git SHA, or "unknown"
  buildTime: string;                       // RFC3339 UTC, or "unknown"
  channel: "development" | "rc" | "release";
};
```

- **Version string is identical across channels** (`0.1.0`). Channel — not the
  version — distinguishes dev/rc/release builds; no `-rc` prerelease suffix
  (the repository has no SemVer prerelease convention).
- `version.go` exposes `Version`, `Commit`, `BuildDate`, `Channel`; `String()`
  renders `datadeck <version> (commit <c>, built <b>, channel <ch>)`.
- Frontend `BUILD_INFO` (build-time, `NEXT_PUBLIC_*` with deterministic
  fallbacks) mirrors the same shape; `application` = "DataDeck".

## Channel semantics
| Channel | How produced | Version |
|---|---|---|
| development | default / `go build` without ldflags, `npm run build:static` without env | 0.1.0 |
| rc | `-X …Channel=rc` (R06) | 0.1.0 |
| release | `scripts/build-release.sh`, `docker/Dockerfile` (`CHANNEL=release`) | 0.1.0 |

## Provenance
- Commit: `git rev-parse --short HEAD` at build time; `unknown` outside a checkout.
- Timestamp: RFC3339 UTC; `SOURCE_DATE_EPOCH` honored for reproducibility
  (`scripts/build-release.sh`), otherwise build-time UTC.
- No secret, DSN, credential, key, or filesystem path is ever placed in metadata.

## Version validation
`scripts/check-version.sh` asserts the invariant
`VERSION == version.go == package.json == package-lock.json == build-info ===
Dockerfile ARG` and exits nonzero on any mismatch.
