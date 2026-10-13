# DataDeck v0.1.0 — R07 Reproducibility Assessment

Controlled rebuild (no original artifacts modified): rebuilt `linux/amd64` from the
same source `f92ce49` with identical `-trimpath -ldflags` and a fixed
`BuildDate=2026-10-13T00:17:33Z`.

| Comparison | Result |
|---|---|
| Rebuild #1 vs Rebuild #2 | **identical** (`c2c0d4e6…1a2a59`) — Go build is deterministic |
| Rebuild vs R06 artifact (`44939859…1d374`) | **differs** |

## Cause
The Go layer is deterministic, but the **Next.js static export differs across runs**
(build-time hashes/timestamps in the bundle), and R06 did not set `SOURCE_DATE_EPOCH`.
Therefore cross-build **bit-for-bit reproducibility is NOT achieved**.

- Toolchain reproducibility: **PASS** (locked `go.mod`/`go.sum`, `npm ci`,
  `go mod verify`).
- Bit-for-bit reproducibility: **NOT VERIFIED** (documented MEDIUM limitation).

Recommended (future task): pin `SOURCE_DATE_EPOCH`, investigate frontend export
determinism, and compare checksums in CI. Not a v0.1.0 release blocker.
