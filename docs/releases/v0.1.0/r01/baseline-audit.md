# DataDeck v0.1.0 — R01 Baseline Audit

## Repository state (recorded)
```text
git branch --show-current      -> main
git rev-parse HEAD             -> 351444fdd3982cd8d73784e3b666becdf8b4662a
git log -5 --oneline           -> 351444f T20 / e3f6c20 T19 / 7958007 T18 / efeb9ce T17 / 8a015a2 T16
git remote -v (sanitized)      -> origin https://github.com/alfiannuha/DataDeck.git
git tag --list                 -> (none)
git rev-list --left-right --count origin/main...HEAD -> 0    11   (0 behind, 11 ahead)
origin/main                    -> 624defc (PRF02-T09)
git status --short             -> ?? multi_connection_architectural_specification.md (untracked, not ours)
```

## Candidate baseline
| Field | Value |
|---|---|
| Commit | `351444f` |
| Branch | `main` |
| Subject | PRF02-T20: final readiness review + MySQL delete integration verification |
| Parent | `e3f6c20` |
| Readiness evidence | `docs/validation/prf02-t20/*` + `docs/validation/prf02-t19/*` |
| CI status | **NOT VERIFIED** — commit is unpushed; no CI run targets it |

## Baseline composition
- PRF02-T00…T18 implementation (code commit `7958007`).
- T19 evidence (`e3f6c20`) and T20 evidence + one in-scope MySQL delete integration
  test (`351444f`). No product source change after `7958007` except that test.

## Divergence & tags
- 11 commits ahead of `origin/main`; T10–T20 are **not pushed** → **CI has never
  run on this code**. Local equivalents pass (see `ci-preflight.md`).
- No `v0.1.0` tag exists; no release branch exists. `release.yml` triggers on
  `push: tags: ["v*"]` and contains a `gh release create` step — so **tagging
  publishes**. R01 must not tag (R08 owns it).

## Immutability
`351444f` is the immutable source baseline for audit. The R01 documentation
commit (this change set) is separate and does not alter it.

## Safety
No reset/clean/stash/rebase/force-push/delete was performed. The untracked file
was neither modified nor deleted.
