# DataDeck v0.1.0 — R08 Repository & Tag Verification

## Remote / repository
```text
remote:            origin https://github.com/alfiannuha/DataDeck.git
repository:        alfiannuha/DataDeck  (matches historical reference)
visibility:        PUBLIC
default branch:    main
viewer permission: ADMIN (tag + release + asset upload permitted)
```
No divergence from the historical remote — no reconciliation needed.

## Working tree
- Branch `main`; HEAD = the current release-docs commit; tracked tree clean.
- One unrelated untracked file (`multi_connection_architectural_specification.md`);
  not part of the release; not deleted (R01-FIND-007 disposition retained).

## Tag / release collision check
| Check | Result |
|---|---|
| Local tag `v0.1.0` | **does not exist** |
| Remote tag `v0.1.0` | **does not exist** |
| Existing GitHub Release `v0.1.0` | **does not exist** |

No collision → a fresh tag/release is possible once the channel blocker is resolved.

## Release workflow side effects (important)
`.github/workflows/release.yml` triggers on `push: tags: ["v*"]` and contains a
`gh release create` step. Therefore **pushing the tag auto-runs the release workflow
and publishes**. Two consequences:
1. Publication must not rely on a manual upload that could conflict with the
   workflow's independently-built artifact set.
2. The workflow's inline build does **not** inject `Channel=release` (see
   `publication-findings.md`), so a tag-triggered release would ship
   `channel: development` binaries — a release-integrity defect that must be fixed
   before tagging.
