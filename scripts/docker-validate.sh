#!/usr/bin/env bash
#
# Build and validate the production DataDeck container image.
# Requires a working Docker daemon; reports NOT RUN when unavailable.
#
# Usage: scripts/docker-validate.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v docker >/dev/null 2>&1; then
  echo "NOT RUN: docker is not installed in this environment."
  exit 2
fi

# Bound the daemon probe: a stopped Docker Desktop can make the CLI hang.
docker info >/dev/null 2>&1 &
probe=$!
for _ in $(seq 1 20); do
  kill -0 "$probe" 2>/dev/null || break
  sleep 0.5
done
if kill -0 "$probe" 2>/dev/null; then
  kill -9 "$probe" 2>/dev/null || true
  echo "NOT RUN: docker daemon is unresponsive (is Docker running?)."
  exit 2
fi
if ! wait "$probe"; then
  echo "NOT RUN: docker daemon is not available."
  exit 2
fi

VERSION="$(tr -d '[:space:]' < VERSION)"
IMAGE="datadeck:${VERSION}-validate"
NAME="datadeck-validate"
KEY="0123456789abcdef0123456789abcdef" # ephemeral, test-only

cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "==> Building image"
docker build -f docker/Dockerfile --build-arg VERSION="$VERSION" -t "$IMAGE" .

echo "==> Image contents (no node/go expected)"
docker run --rm --entrypoint sh "$IMAGE" -c \
  'command -v node || command -v npm || command -v go || echo "no node/npm/go present"'

echo "==> Starting container"
docker run -d --name "$NAME" -e ENCRYPTION_KEY="$KEY" -p 18099:8080 "$IMAGE" >/dev/null

for _ in $(seq 1 60); do
  if curl -sf "http://127.0.0.1:18099/api/v1/health" >/dev/null 2>&1; then break; fi
  sleep 1
done

echo "==> Health"
curl -sf "http://127.0.0.1:18099/api/v1/health" && echo

echo "==> Frontend + PWA assets"
for path in / /manifest.json /sw.js /icons/icon-192x192.png; do
  code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:18099$path")
  echo "  $path -> $code"
done

echo "==> Non-root user"
uid=$(docker exec "$NAME" id -u)
echo "  uid=$uid"
[ "$uid" != "0" ] || { echo "FAIL: container runs as root" >&2; exit 1; }

echo "==> Persistence across restart"
curl -sf -X POST "http://127.0.0.1:18099/api/v1/connections" -H 'Content-Type: application/json' \
  -d '{"name":"docker-persist","driver":"sqlite","database_name":"/data/target.sqlite3"}' >/dev/null
docker restart "$NAME" >/dev/null
for _ in $(seq 1 60); do
  curl -sf "http://127.0.0.1:18099/api/v1/health" >/dev/null 2>&1 && break
  sleep 1
done
curl -sf "http://127.0.0.1:18099/api/v1/connections" | grep -q docker-persist \
  && echo "  profile persisted" || { echo "FAIL: profile lost after restart" >&2; exit 1; }

echo "==> Image size"
bytes=$(docker image inspect --format '{{.Size}}' "$IMAGE")
echo "  ${IMAGE}: ${bytes} bytes"

echo "==> Validation complete"
