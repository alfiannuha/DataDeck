# DataDeck v0.1.0 — R04 Dependency Vulnerability Audit

Tooling: `govulncheck` (Go), `npm audit` (frontend). Baseline: R04 commit.
**No dependency was changed in R04** (an `npm audit fix` trial introduced an
unrelated `typescript` downgrade and was reverted — see §"Rejected change").

## Go — `govulncheck ./...`
```text
No vulnerabilities found.
Your code is affected by 0 vulnerabilities.
(1 import-level + 1 module-level advisory not called by DataDeck.)
```
**PASS** — 0 reachable vulnerabilities (toolchain `go1.26.9`).

## npm — `npm audit --omit=dev`
4 advisories (1 moderate, 3 high):

| Package | Sev | Nature | Applicability to the shipped product |
|---|---|---|---|
| `sharp` | high | librsvg CVE (GHSA-wq5f-xc86-pv6w) | Optional Next image-optimization lib; **not used** by the static-export single binary (no Next server at runtime) |
| `source-map-js` | high | event-loop DoS (GHSA-68fv-2mgg-jv7q) | **Build-time** source-map utility (Next/Tailwind) |
| `postcss` (under `next`) | high | sourceMappingURL path traversal (GHSA-r28c-9q8g-f849) | **Build-time only** (processes DataDeck's own CSS) |
| `postcss` (under `next`) | moderate | incomplete fix (GHSA-fxqj-rqcc-2cmp) | **Build-time only** |

### Triage
All four advisories concern **build-time tooling or an optional image library that
is not present in the shipped static export or the Go binary**. They are reachable
only while running `next build` on trusted input, not against attacker-controlled
CSS/images at runtime. Classified **MEDIUM** (high CVSS, low applicability) —
consistent with the disposition recorded since M5/T15.

### Rejected change (documented)
Running `npm audit fix` (non-force) resolved `sharp`→0.35.5 and `source-map-js`→1.2.2
but also **downgraded `typescript` from 5.9.3 to 5.8.2** and pinned it in
`package.json`. That side effect is unrelated to the advisories and outside R04's
scope, so the change was **reverted**: `package.json` and `package-lock.json` remain
at the R03 baseline (`typescript` 5.9.3), and no dependency was modified in R04.
The `postcss`/Next advisories additionally require a **major** Next upgrade
(`next@15.5.27` outside the declared range) — deferred to a dedicated dependency task.

## Summary
| Ecosystem | Reachable/applicable vulns | Status |
|---|---|---|
| Go | 0 | PASS |
| npm (shipped runtime) | 0 | PASS |
| npm (build-time tooling / optional image lib) | 4 (traced above) | MEDIUM, documented |

No dependency was upgraded or downgraded in the final R04 change set.
