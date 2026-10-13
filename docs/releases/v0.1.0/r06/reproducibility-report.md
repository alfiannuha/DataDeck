# DataDeck v0.1.0 — R06 Reproducibility Report

- Build uses `-trimpath` and a pinned toolchain (`go1.26.9`).
- **Not bit-for-bit reproducible**: the build embeds a `BuildDate` timestamp
  (`2026-10-13T00:17:33Z`). `SOURCE_DATE_EPOCH` is supported by
  `scripts/build-release.sh` but was **not set** for this RC run, so timestamps
  differ between builds.
- Frontend static export may contain build-time hashes/timestamps that affect
  archive bytes.

## Rebuild comparison
A second full rebuild was **NOT performed** (time). Therefore bit-for-bit
reproducibility is **NOT VERIFIED**. The build is **toolchain-reproducible**
(locked Go modules + npm lockfile; `go mod verify` + `npm ci` clean).

Recommendation for R07: set `SOURCE_DATE_EPOCH=<commit time>` and rebuild to compare
hashes, or record that bit-for-bit reproducibility is not a v0.1.0 requirement.
