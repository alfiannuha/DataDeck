# DataDeck v0.1.0 — R01 Release Risk Register

Likelihood/Impact: L/M/H. Status: OPEN / MITIGATED / ACCEPTED.

| Risk ID | Area | Description | L | Impact | Severity | Mitigation | Owner | Status | Release decision |
|---|---|---|---|---|---|---|---|---|---|
| R-01 | Data integrity | Wrong-database read/mutation/deletion | L | H | HIGH | Explicit captured target + backend validation; α/β isolation gate passes (e2e 10/10) | Eng | MITIGATED | Accept |
| R-02 | Data loss | Destructive delete without confirmation | L | H | HIGH | Two-phase preflight token, typed name + ack, protected policy, post-DROP verification, no FORCE/retry | Eng | MITIGATED | Accept |
| R-03 | Integrity | Restore incompatibility / partial restore reported as success | L | M | MEDIUM | Manifest-bound artifact + allowlist; empty-target preflight; verify; MySQL restore disabled | Eng | MITIGATED | Accept |
| R-04 | Security | Ambiguous mutation outcome after network loss | L | M | MEDIUM | No automatic retry; honest `OUTCOME_UNKNOWN`; no mutation replay | Eng | MITIGATED | Accept |
| R-05 | Lifecycle | Pool leak / stale pool after reconnect/delete | L | M | MEDIUM | Per-key serialization; target pool retired; no pool-stacking proven | Eng | MITIGATED | Accept |
| R-06 | Engine | SQLite destructive ops unsupported | — | L | LOW | Disabled by policy; backend `501`; documented | PM/Eng | ACCEPTED | Accept |
| R-07 | Engine | MySQL restore unsupported (no atomic guarantee) | — | M | MEDIUM | Disabled; `RESTORE_UNSUPPORTED`; documented | PM/Eng | ACCEPTED | Accept |
| R-08 | Registry | Encryption key loss ⇒ credentials unrecoverable | L | H | HIGH | Documented key persistence requirement; no key rotation in scope | Ops | ACCEPTED | Document |
| R-09 | Packaging | Binary size growth | M | L | LOW | Explained; optional trim | Eng | ACCEPTED | Accept |
| R-10 | Runtime | Cross-platform behavioral differences (non-macOS) | M | M | MEDIUM | Cross-compiles pass; native smoke **NOT VERIFIED** | Eng | OPEN | Close in R05/R06 |
| R-11 | External tooling | A core feature now needs external tools | L | H | HIGH | Verified self-contained (no pg_dump/mysqldump/sqlite3); CGO-free build passes | Eng | MITIGATED | Accept |
| R-12 | Release pipeline | Tagging publishes automatically (`release.yml`) | M | H | HIGH | R01 does not tag; R08 owns publishing; verified trigger inspection | Release | MITIGATED | Accept |
| R-13 | CI/baseline | Baseline never CI-validated (T10–T20 unpushed) | H | M | MEDIUM | Local equivalents pass; **push/PR + CI green required before R06** (R01-FIND-006) | Release | OPEN | Gate R06 |
| R-14 | PWA caching | Sensitive API responses cached by SW | L | H | HIGH | SW bypasses `/api` (ADR-008 §46); `no-store` on API | Eng | MITIGATED | Accept |

No risk is marked resolved without evidence. R-10 and R-13 are the open items to
close before release; neither is BLOCKER/HIGH-severity data/security exposure.
