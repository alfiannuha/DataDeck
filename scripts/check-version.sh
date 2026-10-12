#!/usr/bin/env bash
#
# Version consistency validator (PRF02-R02).
#
# The repository VERSION file is the single source of truth. Every component must
# report the same application version. This script fails nonzero on any mismatch.
#
# Usage: scripts/check-version.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

canonical="$(tr -d '[:space:]' < "$ROOT/VERSION")"
if [ -z "$canonical" ]; then
  echo "FAIL: VERSION file is empty" >&2
  exit 1
fi
echo "canonical version (VERSION): $canonical"

fail=0
check() {
  # check <label> <value>
  if [ "$2" != "$canonical" ]; then
    echo "FAIL: $1 = '$2' (expected '$canonical')" >&2
    fail=1
  else
    echo "ok:   $1 = $2"
  fi
}

# Backend internal/version default.
backend_version="$(sed -n 's/^[[:space:]]*Version = "\(.*\)"/\1/p' "$ROOT/backend/internal/version/version.go" | head -1)"
check "backend/internal/version.Version" "$backend_version"

# Frontend package.json + lockfile root (structured JSON parsing).
pkg_version="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["version"])' "$ROOT/frontend/package.json")"
check "frontend/package.json version" "$pkg_version"

lock_version="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["version"])' "$ROOT/frontend/package-lock.json")"
check "frontend/package-lock.json version" "$lock_version"

build_info_version="$(sed -n "s/.*NEXT_PUBLIC_DATADECK_VERSION ?? \"\(.*\)\".*/\1/p" "$ROOT/frontend/src/lib/build-info.ts" | head -1)"
check "frontend/src/lib/build-info.ts fallback" "$build_info_version"

# Dockerfile default ARG VERSION.
docker_version="$(sed -n 's/^ARG VERSION=\(.*\)/\1/p' "$ROOT/docker/Dockerfile" | head -1)"
check "docker/Dockerfile ARG VERSION" "$docker_version"

if [ "$fail" -ne 0 ]; then
  echo "VERSION CONSISTENCY: FAILED" >&2
  exit 1
fi
echo "VERSION CONSISTENCY: OK ($canonical)"
