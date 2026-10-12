# DataDeck v0.1.0 — R04 Threat Model

## Assets
- Connection credentials (passwords) and the `ENCRYPTION_KEY`.
- Target database contents and query results.
- Saved queries, query history, and the local metadata store (`STORAGE_PATH`).
- Import/export files.
- Active database sessions/pools.
- Release binaries and build metadata.

## Attackers
- A **malicious website** open in the user's browser (CSRF / DNS rebinding).
- A **local unprivileged process** (same host).
- A **malicious imported file** (CSV/XLSX/SQL).
- **Untrusted SQL** the user runs.
- A **compromised dependency** (Go/npm) or build script.
- A **malicious/misconfigured database server**.
- **Accidental** user action or incorrect connection/tab state.

## Trust boundaries
```text
[ Browser ] --HTTP loopback--> [ Go daemon ] --database/sql--> [ Target DB ]
     |                              |
     |  same-origin only (CSRF)     +-- filesystem (SQLite store, temp files)
     +-- PWA cache (no /api)        +-- AES-256-GCM credentials
                                    +-- outbound: database servers only
```

## Assumptions
- DataDeck binds **loopback** by default and has **no authentication**; it is not
  safe for public exposure. `ALLOW_REMOTE=1` is an explicit operator opt-in.
- The OS account and its filesystem are trusted; the app store is `0600` in a
  `0700` directory.
- Database servers enforce their own authorization; DataDeck never escalates.
- Raw SQL is an intentional capability executed with the user's credentials.

## Primary mitigations (mapped in `security-control-matrix.md`)
- Loopback default + non-loopback refusal without `ALLOW_REMOTE=1`.
- **Origin-enforcing CSRF middleware** for unsafe methods (blocks cross-origin
  simple/multipart mutations) and an explicit CORS allowlist.
- AES-256-GCM credential encryption with a required key, random nonce, tag verify.
- Explicit per-operation database binding (no explorer fallback).
- Metadata-validated, dialect-quoted identifiers + bound parameter values.
- INSERT-only, allowlisted import/restore; DELETE DATABASE two-phase confirmation.
- `no-store` on API responses; service worker bypasses `/api`.

## Out-of-scope / documented residuals
- **DNS rebinding** (Origin==Host under a rebinding attacker) is not fully blocked;
  see `findings-register.md` (MEDIUM) — mitigated by unguessable connection IDs and
  unreadable cross-origin responses.
- **No authentication** by design; local-only boundary is the control.
- Non-macOS native runtime assumptions are unverified (see R01/R04 findings).
