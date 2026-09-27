#!/usr/bin/env bash
#
# Reproducible frontend performance benchmarks (grid virtualization, schema
# rendering, export serialization). Emits [perf]/[export]/[schema] lines.
#
# Usage: scripts/bench-frontend.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/frontend"

echo "# DataDeck frontend benchmark"
echo "# node: $(node --version), os: $(uname -srm)"
npx vitest run \
  src/components/grid/result-grid.perf.test.tsx \
  src/components/sidebar/schema-tree.perf.test.tsx \
  src/lib/export/export.bench.test.ts
