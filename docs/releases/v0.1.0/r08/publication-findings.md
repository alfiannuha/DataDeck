# DataDeck v0.1.0 — R08 Publication Findings

## BLOCKER
| ID | Finding | Evidence | Required action |
|---|---|---|---|
| R08-B-001 | **RC-channel artifacts cannot be promoted to the official release.** R07-verified binaries report `channel: rc`; the version contract requires official artifacts to report `channel: release`. | `datadeck_0.1.0_darwin_arm64 --version` → `channel rc` | Produce official `channel=release` artifacts (R06-rerun / Option B), re-verify (R07), then publish. Do not relabel RC binaries. |
| R08-B-002 | **`release.yml` does not inject `Channel=release`.** Its inline `go build` sets only Version/Commit/BuildDate, so a tag-triggered release would ship `channel: development` binaries. | `.github/workflows/release.yml` build step (no `Channel` ldflag) | Add `-X github.com/datadeck/datadeck/backend/internal/version.Channel=release` to the workflow build, and verify a workflow-produced binary reports `channel: release`. |

## MEDIUM (carried)
R04-FIND-001 (DNS rebinding residual), R04-FIND-002 (npm build-time advisories),
R07-F-001 (bit-for-bit reproducibility), PRF02-FIND-001/002.

## LOW / NOT RUN (carried)
R07-F-002/003, R04-FIND-003, R05-FIND-004/005.

## Not performed
- No Git tag created.
- No GitHub Release created.
- No asset uploaded.
- No artifact modified.
