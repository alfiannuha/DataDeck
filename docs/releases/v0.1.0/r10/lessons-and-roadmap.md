# DataDeck v0.1.0 — R10 Lessons Learned & v0.2.0 Roadmap Handoff

## Lessons learned (evidence-based)
**Worked well**
- Explicit per-operation DB binding + α/β e2e repeatedly caught/validated isolation.
- Two-phase destructive token + protected policy + fail-closed reconnect.
- Contract tests (swagger ↔ router ↔ envelope) kept the API honest.
- Version validator + single-source `VERSION` kept the toolchain aligned.

**Required rework / bottlenecks**
- Channel policy: RC (`channel=rc`) artifacts could not be promoted; required a
  release-channel rebuild + a `release.yml` `Channel=release` fix (R08-B-001/B-002).
- `release.yml` also missed the frontend channel env (R09-F-001) and provenance
  (R08-F-001) — release automation gaps.
- Test-harness flakes: fixed-name e2e DBs under a long-lived server (R04-FIND-005);
  Playwright browser revision after `npm ci`.
- Local-only verification: no CI on T10–R04 until R05 push; no non-mac native runners.

**Improvements to make**
- Release automation: inject frontend `NEXT_PUBLIC_DATADECK_*`, emit provenance,
  flatten `SHA256SUMS` paths, name inner binary consistently.
- Add multi-OS native CI (macOS amd64, Linux amd64/arm64, Windows), reproducible builds
  via `SOURCE_DATE_EPOCH`, signed provenance (Sigstore/in-toto).
- Failure-injection + large-scale perf in CI.

## v0.2.0 roadmap handoff (planning input only — NOT implemented)
Primary candidate: **Concurrent Multi-Connection Engine** (`multi_connection_architectural_specification.md`
is a planning input, not proof of readiness). Areas to design/verify:
- Pool-of-pools lifecycle; per-(connection,database) identity; idle-pool janitor.
- Connection generation + stale async response handling; cancellation.
- Tab↔connection and tab↔database binding across concurrent profiles.
- Resource limits, telemetry, environment badges; persistence boundaries; security.

## v0.2.0 technical-debt register (prioritized)
| # | Item | Risk | Value | Complexity |
|---|---|---|---|---|
| 1 | Fix R09-F-001 frontend channel env | Low | High (metadata integrity) | Low |
| 2 | Emit + sign provenance; flatten checksums; align archive naming | Med | High (supply chain) | Low–Med |
| 3 | Multi-OS native CI runners | Med | High | Med |
| 4 | Bit-for-bit reproducible builds (`SOURCE_DATE_EPOCH`, frontend determinism) | Med | Med | Med |
| 5 | Larger failure-injection + load/perf suites | Low | Med | Med |
| 6 | Concurrent Multi-Connection Engine | High | High | High |
| 7 | XLSX import, engine capability expansion | Med | Med | High |

Priorities reflect actual v0.1.0 evidence; items are proposals, not commitments, and
carry no dates. Do **not** implement v0.2.0 during R10.
