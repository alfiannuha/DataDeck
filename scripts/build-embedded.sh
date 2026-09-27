#!/usr/bin/env bash
#
# Deterministic single-binary build: install frontend deps, produce the static
# export, embed it into the Go binary, and build a CGO-free executable.
#
# Usage: scripts/build-embedded.sh [output-binary]
#   output-binary defaults to backend/bin/datadeck
#
# This does NOT perform release packaging (archives, checksums); it only proves
# the embed pipeline. Run `make clean` and reset the dist placeholder afterwards
# if you do not want the build output committed.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT="${1:-$ROOT/backend/bin/datadeck}"
DIST="$ROOT/backend/internal/web/dist"

echo "==> Installing frontend dependencies (npm ci)"
( cd "$ROOT/frontend" && npm ci )

echo "==> Building static frontend export (frontend/out)"
( cd "$ROOT/frontend" && npm run build:static )

if [ ! -f "$ROOT/frontend/out/index.html" ]; then
  echo "ERROR: frontend/out/index.html is missing after the static build." >&2
  echo "       The static export did not produce a usable bundle; aborting." >&2
  exit 1
fi

echo "==> Refreshing embedded assets ($DIST)"
rm -rf "$DIST"
mkdir -p "$DIST"
cp -R "$ROOT/frontend/out/." "$DIST/"

# Keep the embedded payload lean: never ship source maps or dev artifacts.
find "$DIST" -name '*.map' -type f -delete
find "$DIST" -name '*.DS_Store' -type f -delete

echo "==> Building Go binary ($OUTPUT)"
mkdir -p "$(dirname "$OUTPUT")"
( cd "$ROOT/backend" && CGO_ENABLED=0 go build -o "$OUTPUT" ./cmd/server )

echo "==> Done."
echo "    binary: $OUTPUT"
echo "    embedded files: $(find "$DIST" -type f | wc -l | tr -d ' ')"
