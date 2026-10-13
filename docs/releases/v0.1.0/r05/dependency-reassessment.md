# DataDeck v0.1.0 — R05 Dependency Reassessment

Reconciles the R04 wording inconsistency against the **actual** dependency state.

## Actual state at `e81ca8e`
- Frontend manifests are at the R03 baseline: `frontend/package.json` /
  `package-lock.json` **unchanged by R04** (the `npm audit fix` trial was reverted;
  `typescript` = 5.9.3).
- `go.mod`/`go.sum` unchanged; `go mod verify` → all modules verified.

## Go — `govulncheck ./...`
`No vulnerabilities found. Your code is affected by 0 vulnerabilities.` (1
import-level + 1 module-level advisory not called.) **PASS / 0 reachable.**

## npm — `npm audit --omit=dev` (actual, 4 advisories)
| Package | Installed | Severity | Applicability | Reachable in shipped product? | Mitigation / residual |
|---|---|---|---|---|---|
| `sharp` | as locked (Next optional image lib) | high | optional image optimization | **No** (no Next server in static-export binary) | deferred to a dependency task; accepted MEDIUM |
| `source-map-js` | as locked | high | build-time source-map utility | **No** (build only) | accepted MEDIUM |
| `postcss` (under `next`) | as locked | high | build-time CSS | **No** (build only) | needs major Next; accepted MEDIUM |
| `postcss` (under `next`) | as locked | moderate | build-time CSS | **No** | accepted MEDIUM |

**Reconciliation:** R04's `dependency-audit.md` initially described a `sharp`/
`source-map-js` fix; that change was **reverted** (it downgraded `typescript`). The
authoritative state is: **no dependency change; 4 build-time/optional advisories,
all MEDIUM, none reachable in the shipped static export or Go binary.** The R04
docs were corrected to match (see `r04/dependency-audit.md`).

## Accepted residual risk
All four advisories are build-time or optional-image-lib; the shipped artifact is a
static export embedded in a CGO-free Go binary with no Node runtime. Risk to end
users: **low**. No `--force` major upgrade applied (out of release scope).
