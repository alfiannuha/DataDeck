#!/usr/bin/env bash
#
# Non-publishing release dry run: build the full platform matrix, generate
# supply-chain artifacts, package archives and checksums into release/.
#
# Nothing is published. Use this (or workflow_dispatch with dry_run=true) to
# verify artifact generation.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
VERSION="$(tr -d '[:space:]' < VERSION)"
echo "==> Dry run for version $VERSION"

echo "==> Building platform matrix"
bash scripts/build-release.sh

mkdir -p release/sbom release/licenses

echo "==> Frontend SBOM (CycloneDX)"
( cd frontend && npm sbom --sbom-format cyclonedx > "$ROOT/release/sbom/frontend.cdx.json" )

if command -v syft >/dev/null 2>&1; then
  echo "==> Go binary SBOM (CycloneDX + SPDX)"
  BIN="release/datadeck_${VERSION}_$(go env GOOS)_$(go env GOARCH)"
  syft scan "file:$BIN" \
    -o cyclonedx-json=release/sbom/datadeck-go.cdx.json \
    -o spdx-json=release/sbom/datadeck-go.spdx.json
else
  echo "WARN: syft not available; Go SBOM skipped (run scripts/release-dry-run.sh in the release job which installs syft)"
fi

echo "==> License notices"
bash scripts/generate-notices.sh

echo "==> Packaging archives + checksums"
python3 scripts/package_artifacts.py --version "$VERSION" --release-dir release

echo "==> Dry-run artifacts:"
ls -1 release
echo "==> Done (nothing published)"
