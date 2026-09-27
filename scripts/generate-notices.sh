#!/usr/bin/env bash
#
# Generate the third-party license inventory/notices for a release.
#
# Authoritative sources (not hand-written):
#   - Go: the CycloneDX SBOM of the release binary (syft) defines the shipped
#     module set; license files are read from the Go module cache.
#   - npm: the CycloneDX SBOM produced by `npm sbom`.
#
# Output: release/licenses/THIRD_PARTY_NOTICES.txt
#
# This is an inventory, not a legal conclusion. Flagged items require human
# review (see docs/release/licenses.md).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/release/licenses/THIRD_PARTY_NOTICES.txt"
GO_SBOM="$ROOT/release/sbom/datadeck-go.cdx.json"
NPM_SBOM="$ROOT/release/sbom/frontend.cdx.json"
mkdir -p "$(dirname "$OUT")"

( cd "$ROOT/backend" && go list -m -f '{{.Path}}|{{.Version}}|{{.Dir}}' all ) > /tmp/datadeck-go-modules.txt

{
  echo "DataDeck third-party license inventory"
  echo "Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "Version: $(tr -d '[:space:]' < "$ROOT/VERSION")"
  echo
  echo "This inventory is generated from dependency SBOMs and license files."
  echo "It is not legal advice; flagged entries require human/legal review."
  echo
  echo "======================================================================"
  echo "GO MODULES SHIPPED IN THE RELEASE BINARY (source: $GO_SBOM)"
  echo "======================================================================"
  python3 - "$GO_SBOM" /tmp/datadeck-go-modules.txt <<'PY'
import json, os, sys
sbom_path, modules_path = sys.argv[1], sys.argv[2]
dirs = {}
try:
    sbom = json.load(open(sbom_path))
except FileNotFoundError:
    sbom = None
    print("  NOTE: Go SBOM not found; listing the full module graph instead.")
    print("        Generate it with: syft scan file:release/datadeck_<version>_<os>_<arch> -o cyclonedx-json=" + sbom_path)
try:
    with open(modules_path) as fh:
        for line in fh:
            parts = line.rstrip("\n").split("|")
            if len(parts) == 3:
                dirs[(parts[0], parts[1])] = parts[2]
except FileNotFoundError:
    pass

def classify(text):
    t = text.lower()
    if "mozilla public license" in t:
        return "MPL-2.0"
    if "apache license" in t:
        return "Apache-2.0"
    if "permission is hereby granted, free of charge" in t:
        return "MIT"
    if "redistribution and use in source and binary forms" in t:
        return "BSD-3-Clause" if "\n3." in text or " 3." in text else "BSD-2-Clause"
    if "permission to use, copy, modify, and/or distribute" in t:
        return "ISC"
    if "gnu lesser general public license" in t:
        return "LGPL"
    return "REVIEW"

rows = []
if sbom is None:
    entries = sorted(dirs.keys())
else:
    entries = []
    for c in sbom.get("components", []):
        purl = c.get("purl") or ""
        if purl.startswith("pkg:golang/"):
            entries.append((c.get("name"), c.get("version")))
for name, version in entries:
    d = dirs.get((name, version), "")
    license_name = "UNKNOWN"
    if d and os.path.isdir(d):
        for entry in sorted(os.listdir(d)):
            if entry.lower().startswith(("license", "licence", "copying", "notice")):
                try:
                    head = open(os.path.join(d, entry), errors="ignore").read(4000)
                except OSError:
                    continue
                license_name = classify(head)
                break
    rows.append((name, version, license_name))
for name, version, lic in sorted(rows):
    print(f"  {name:<55} {version:<18} {lic}")
print(f"  ({len(rows)} shipped Go modules)")
PY
  echo
  echo "======================================================================"
  echo "NPM PACKAGES (source: $NPM_SBOM)"
  echo "======================================================================"
  if [ -f "$NPM_SBOM" ]; then
    python3 - "$NPM_SBOM" <<'PY'
import json, sys, collections
d = json.load(open(sys.argv[1]))
counts = collections.Counter()
flagged = []
for c in d.get("components", []):
    ids = [l.get("license", {}).get("id") or l.get("expression") for l in (c.get("licenses") or [])]
    for i in ids or ["UNKNOWN"]:
        counts[i] += 1
    if any(i and ("LGPL" in i or "GPL" in i or "MPL" in i) for i in ids):
        flagged.append((c.get("name"), c.get("version"), ids, c.get("scope")))
print("  License id counts (component-level, includes dev/optional):")
for k, v in sorted(counts.items(), key=lambda kv: -kv[1]):
    print(f"    {v:4d}  {k}")
print()
print("  Flagged for review (copyleft-family):")
for name, ver, ids, scope in flagged:
    print(f"    {name}@{ver} :: {ids} :: scope={scope}")
PY
  else
    echo "  SBOM not found at $NPM_SBOM; run 'npm sbom --sbom-format cyclonedx' first."
  fi
  echo
  echo "Human-review flags:"
  echo "  - go-sql-driver/mysql (MPL-2.0): file-level copyleft; retain license/notices."
  echo "  - lightningcss / lightningcss-* and axe-core (MPL-2.0): retain notices."
  echo "  - @img/sharp-* (LGPL-3.0-or-later, optional): verify whether shipped."
  echo "    DataDeck disables next/image optimization (images.unoptimized) and does"
  echo "    not require sharp at runtime."
} > "$OUT"

echo "Wrote $OUT"
