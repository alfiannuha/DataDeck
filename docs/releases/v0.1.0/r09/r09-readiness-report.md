# DataDeck v0.1.0 — R09 Published Artifact Smoke Test

## A. Executive Summary
- **Release URL:** https://github.com/alfiannuha/DataDeck/releases/tag/v0.1.0
- **Version:** `0.1.0` · **Tag:** `v0.1.0` → `6d99856` · **Channel:** `release` (backend)
- **Decision:** **R09 READY** — the published release is authentic, downloadable, installable, and functional on the native runner; no BLOCKER/HIGH. Documented LOW findings + BLOCKED/NOT-VERIFIED items.

## B. Prerequisites
R01–R08 READY; release published (not draft); tag target matches approved source. CI green on the source.

## C. Published Release
`alfiannuha/DataDeck`, tag `v0.1.0`→`6d99856` (local+remote), published 2026-10-13T02:19:20Z, 11 assets.

## D. Download Integrity
Fresh download (not local build); 11 assets; **9/9 `SHA256SUMS` entries match**; provenance hashes match downloaded bytes (10/10); archives safe (no traversal/absolute paths/symlinks); formats/arch correct.

## E. Platform Matrix
darwin/arm64 **native PASS** (CLI, startup, health, UI, shutdown); darwin/amd64, linux/amd64, linux/arm64, windows/amd64 = download+hash+format verified, **runtime NOT VERIFIED** (no runners).

## F. Installation
Extracted per docs and ran successfully; note the archive binary is named `datadeck`/`datadeck.exe` (R09-F-003).

## G. Runtime
CLI `0.1.0 … channel release` exit 0; loopback `127.0.0.1`; `/health` `0.1.0/release`; `/`, `/manifest.json`, `/sw.js` 200; SIGTERM clean.

## H. Database
PostgreSQL full E2E (α/β isolation, CRUD, import/export, reconnect, delete) PASS; SQLite API smoke PASS; **MySQL BLOCKED** (server down; correct `CONNECTION_ERROR`).

## I. Data Management
Table Data / CRUD / import/export / structure / delete — PASS via the published binary (E2E). No α/β crossover.

## J. Security
Loopback-only; CSRF cross-origin 403 / same-origin 400; `no-store`/`nosniff`/`no-referrer`; ENCRYPTION_KEY fail-closed; no unexpected outbound; DNS-rebinding residual MEDIUM (unchanged).

## K. Supply Chain
SBOM (CycloneDX 1.5/1.7, SPDX 2.3) + `published-provenance.json` verified (unsigned, disclosed).

## L. Performance
Native startup ~sub-second to health; RSS/UI served from the embedded bundle. (No new benchmark; consistent with R05/R07.)

## M. Findings
No BLOCKER/HIGH. LOW: **R09-F-001** (frontend UI channel label `development` vs backend `release` — metadata only), R09-F-002 (SHA256SUMS path prefixes), R09-F-003 (archive binary name), R09-F-004 (e2e cleanup flake), R09-F-005 (Playwright browser install), R04-FIND-003, R05-FIND-004/005. MEDIUM carried: R04-FIND-001/002, R07-F-001. BLOCKED/NOT-RUN: MySQL runtime, non-mac native runtime.

## N. Release Impact
Eligible for R10 (final release review). No incident/recovery required. Recommended follow-ups (non-blocking): fix R09-F-001 in a future patch (inject `NEXT_PUBLIC_DATADECK_CHANNEL` in `release.yml`), flatten SHA256SUMS paths, align archive binary naming with docs.

## O. Final Decision
**R09 READY.** The published artifacts are authentic (checksums/SBOM/provenance), the native binary is functional and secure, and no BLOCKER/HIGH was found. Residual items are documented LOW/BLOCKED/NOT-VERIFIED.

## P. Commit / Report
- **Files inspected:** AGENTS.md, PRD, ADR-010, R01–R08 artifacts, `docs/releases/v0.1.0/r08/*`, GitHub Release live data, downloaded assets.
- **Files changed:** `docs/releases/v0.1.0/r09/*`.
- **Commands:** `gh release view/download`, `shasum -a 256 -c`, `file`, `tar -xzf`, `unzip`, `--version`, `curl` (health/UI/CSRF/headers), `npx playwright install chromium`, `npx playwright test` (published binary), disposable PG/SQLite/MySQL API smoke, `python3` SBOM/provenance checks.
- **Published URLs:** https://github.com/alfiannuha/DataDeck/releases/tag/v0.1.0
- **Downloaded artifact hashes:** `SHA256SUMS` (9/9 verified); see `download-integrity.md`.
- **Evidence paths:** `docs/releases/v0.1.0/r09/`.
- **Final report commit:** this R09 docs commit.

**Stop — do not begin R10 automatically.**
