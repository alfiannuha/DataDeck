#!/usr/bin/env bash
#
# Cross-platform single-binary release build.
#
# Produces deterministic, CGO-free executables for the supported matrix:
#   darwin/arm64  darwin/amd64  linux/amd64  linux/arm64  windows/amd64
#
# Artifacts: release/datadeck_<version>_<os>_<arch>[.exe]
#
# The frontend is exported once and embedded identically for every target.
# Runtime verification is only performed for platforms actually executed (see
# docs/release/platform-matrix.md); cross-compilation alone is BUILD PASS only.
#
# Usage: scripts/build-release.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
[ -n "$VERSION" ] || { echo "ERROR: VERSION file is empty" >&2; exit 1; }

COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
if [ -n "${SOURCE_DATE_EPOCH:-}" ]; then
  BUILD_DATE="$(date -u -r "$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)"
else
  BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi

RELEASE_DIR="$ROOT/release"
DIST="$ROOT/backend/internal/web/dist"
LDFLAGS="-s -w -X github.com/datadeck/datadeck/backend/internal/version.Version=$VERSION \
  -X github.com/datadeck/datadeck/backend/internal/version.Commit=$COMMIT \
  -X github.com/datadeck/datadeck/backend/internal/version.BuildDate=$BUILD_DATE"

echo "==> Version: $VERSION  Commit: $COMMIT  Build date: $BUILD_DATE"

echo "==> Building static frontend export"
( cd "$ROOT/frontend" && npm ci && npm run build:static )
[ -f "$ROOT/frontend/out/index.html" ] || { echo "ERROR: frontend/out/index.html missing" >&2; exit 1; }

echo "==> Refreshing embedded assets"
rm -rf "$DIST"; mkdir -p "$DIST"
cp -R "$ROOT/frontend/out/." "$DIST/"
find "$DIST" -name '*.map' -type f -delete
find "$DIST" -name '.DS_Store' -type f -delete

mkdir -p "$RELEASE_DIR"

TARGETS=(
  "darwin arm64"
  "darwin amd64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
)

for target in "${TARGETS[@]}"; do
  set -- $target
  goos="$1"; goarch="$2"
  ext=""
  [ "$goos" = "windows" ] && ext=".exe"
  output="$RELEASE_DIR/datadeck_${VERSION}_${goos}_${goarch}${ext}"

  echo "==> Building $goos/$goarch"
  ( cd "$ROOT/backend" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -ldflags "$LDFLAGS" -o "$output" ./cmd/server )

  bytes=$(wc -c < "$output" | tr -d ' ')
  echo "    $output ($bytes bytes)"
done

echo "==> Done. Artifacts in $RELEASE_DIR"
ls -1 "$RELEASE_DIR"
