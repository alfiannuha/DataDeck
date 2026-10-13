# DataDeck v0.1.0 — R06 Provenance Report

`provenance/provenance.json` (checksum in `checksums/SBOM-SHA256SUMS`) records:

```json
{
  "application": "DataDeck",
  "version": "0.1.0",
  "channel": "rc",
  "source_commit": "f92ce49ee1297a6c53bec8a51a3621862c391b85",
  "source_commit_short": "f92ce49",
  "build_environment": "macOS 26.6.2 arm64",
  "go_version": "go1.26.9",
  "node_version": "v24.14.0",
  "npm_version": "11.9.0",
  "build_timestamp": "2026-10-13T00:17:33Z",
  "artifacts": [ …5 entries with filename/platform/arch/sha256/size/build_result/smoke_test/package… ]
}
```

- Source commit is the **full** approved SHA embedded in every binary and in the
  manifest (`Commit` ldflag = full SHA).
- Toolchain is identical across all five targets (one Go toolchain, one frontend
  export, one set of ldflags).
- No secrets or personal paths in the manifest.
