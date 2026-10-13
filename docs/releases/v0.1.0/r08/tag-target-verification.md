# DataDeck v0.1.0 — R08 Tag Target & Channel Policy

## Release identity
```text
Application : DataDeck
Version     : 0.1.0
Git tag     : v0.1.0            (NOT created)
Channel     : release           (required for official artifacts)
Source code : f92ce49·          (code commit the RC binaries were built from)
```

## Approved source
`f92ce49ee1297a6c53bec8a51a3621862c391b85` — CI green (CI 38326084373, E2E
38326084548). This is the R05-approved source and the R06/R07 artifact source.

## Channel policy decision (blocker)
The version contract (R02) defines channel = `development | rc | release`, with the
official release built as `channel=release` (`scripts/build-release.sh` injects
`Channel=release`). The R06/R07-verified artifacts report **`channel: rc`**:

```text
$ release/rc/v0.1.0/binaries/datadeck_0.1.0_darwin_arm64 --version
datadeck 0.1.0 (commit f92ce49…, built …, channel rc)
```

Per R08 §11/§12, RC-channel binaries **cannot be promoted** to the official
`release` by renaming or retitling. The default is the approved **release-channel
contract**, so a release-channel build is required.

Additionally, `.github/workflows/release.yml` builds without `-X …Channel=release`,
so a tag-triggered official build would report `channel: development`.

## Consequence
The tag target cannot be finalized and no tag/release may be created until:
1. `release.yml` injects `Channel=release`, and
2. official `channel=release` artifacts are produced and independently verified.

No tag was created; no release was created.
