# DataDeck v0.1.0 — R05 Baseline Verification

Commit inspected: **`e81ca8e`** (R04 CSRF hardening), branch `main`.

```text
branch:            main
HEAD:              e81ca8e63cba42d74e23482b24f8289ead4e3bda
HEAD (short):      e81ca8e
working tree:      clean (tracked); 1 unrelated untracked file (below)
tags:              (none)
origin/main:       624defc (PRF02-T09)
ahead/behind:      0 behind / 15 ahead
R04 CSRF present:  backend/internal/api/middleware/csrf.go tracked in HEAD
```

## Commits since the release source baseline `351444f`
| Commit | Task |
|---|---|
| `d0e6461` | R01 release freeze & preflight |
| `f29e0b3` | R02 version finalization |
| `ec444da` | R03 documentation |
| `e81ca8e` | R04 security audit + CSRF middleware |

## Freeze compliance
All changes since `351444f` are **version metadata, release documentation, and
approved security hardening**, plus this R05 test-harness reliability fix. No new
product features, no new engines, no v0.2.0 concurrency engine, no unrelated
dependency upgrades (frontend manifests unchanged), no API-shape changes beyond the
R02 health `version`/`channel` metadata.

## Untracked file (R01-FIND-007)
`multi_connection_architectural_specification.md` — untracked, never committed, not
created by the release work. Disposition: **kept outside the release** (not tracked,
not deleted). It is not referenced by any build/CI/release step and cannot enter
release artifacts (which are built from tracked sources + generated output).
Recommended: move to a v0.2.0 planning location or keep out of the repo.
