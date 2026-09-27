#!/usr/bin/env bash
#
# Reproducible backend startup / idle-memory benchmark.
#
# Builds a release-mode backend (CGO disabled), launches it against a temporary
# store, records time-to-health-ready and idle RSS, then shuts it down.
#
# Usage: scripts/bench-backend.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${PORT:-18090}"
WORK="$(mktemp -d)"
BIN="$WORK/datadeck-bench"
STORE="$WORK/datadeck.db"
KEY="0123456789abcdef0123456789abcdef" # benchmark-only key
LOG="$WORK/backend.log"

now_ms() { python3 -c 'import time; print(int(time.time()*1000))'; }

echo "# DataDeck backend benchmark"
echo "# os: $(uname -srm)"
echo "# build: CGO_ENABLED=0, environment=test, port=$PORT"

( cd "$ROOT/backend" && CGO_ENABLED=0 go build -o "$BIN" ./cmd/server )
BIN_BYTES=$(wc -c < "$BIN" | tr -d ' ')
echo "binary_bytes=$BIN_BYTES"

start=$(now_ms)
ENCRYPTION_KEY="$KEY" STORAGE_PATH="$STORE" HOST=127.0.0.1 PORT="$PORT" ENVIRONMENT=test \
  "$BIN" >"$LOG" 2>&1 &
PID=$!
trap 'kill "$PID" 2>/dev/null || true; rm -rf "$WORK"' EXIT

ready=""
for _ in $(seq 1 500); do
  if curl -sf "http://127.0.0.1:$PORT/api/v1/health" >/dev/null 2>&1; then
    ready=$(now_ms)
    break
  fi
  sleep 0.01
done
if [ -z "$ready" ]; then
  echo "FAILED: backend did not become healthy" >&2
  cat "$LOG" >&2
  exit 1
fi
echo "startup_ms=$((ready - start))"

sleep 2
rss_kb=$(ps -o rss= -p "$PID" | tr -d ' ')
rss_mb=$(python3 -c "print(round($rss_kb/1024, 2))")
echo "idle_rss_kb=$rss_kb"
echo "idle_rss_mb=$rss_mb"

# Report a second sample after the process has settled further.
sleep 3
rss_kb2=$(ps -o rss= -p "$PID" | tr -d ' ')
rss_mb2=$(python3 -c "print(round($rss_kb2/1024, 2))")
echo "settled_rss_kb=$rss_kb2"
echo "settled_rss_mb=$rss_mb2"
