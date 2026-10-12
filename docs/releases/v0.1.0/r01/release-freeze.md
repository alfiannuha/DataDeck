# DataDeck v0.1.0 — R01 Release Freeze Record

```text
Release:              DataDeck v0.1.0
Freeze Stage:         R01 — Freeze & Preflight
Candidate Commit:     351444f (PRF02-T20)   [source baseline]
Source baseline:      351444f  (all verified PRF02 work)
Prior code baseline:  7958007  (PRF02-T18)  — T19/T20 added evidence + 1 test
Branch:               main
Freeze Date:          2026-10-13
PRF02 Decision:       READY (docs/validation/prf02-t20/final-readiness-report.md)
Working Tree:         CLEAN (tracked) — 1 unrelated untracked file, see below
CI Status:            NOT VERIFIED for 351444f (commit is unpushed; origin/main = 624defc / T09)
Open BLOCKER:         0
Open HIGH:            0
Scope Changes Allowed: Release-blocking fixes only (bug/security/data-integrity/build/doc/test)
Tag Created:          NO
Published:            NO
```

## Working-tree note
- Tracked tree is clean at `351444f`.
- One **untracked, never-committed** file exists at repo root:
  `multi_connection_architectural_specification.md`. It was **not** created by
  PRF02 work and is excluded from the baseline commit. It must be resolved
  (kept, moved, or ignored) by its owner **before R06** so it cannot enter release
  artifacts. Not a source change; not deleted.
- `docs/release/` is gitignored by the repo (`.gitignore:46` `release/`), so R01
  artifacts are placed under the tracked `docs/releases/v0.1.0/r01/` (documented
  deviation from the prompt's suggested path).

## Change control (post-freeze)
Every post-freeze change records: reason, finding/issue id, affected files, risk
class, reviewer, tests rerun, new baseline commit, updated evidence, release
impact. A documentation-only R01 commit is separate from the source baseline and
does not alter `351444f`. No untracked change may enter release artifacts.

## Boundary
No tag, no version bump, no publish, no deploy performed. R01 does not begin R02.
