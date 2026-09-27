# DataDeck Threat Model

Status: baseline for M5 (Production Hardening). Derived from PRD §11,
`docs/architecture.md` §15, `docs/security.md`, the API contract, and a review of
the M4 implementation.

## 1. Scope

DataDeck is a local-first database GUI. A Go daemon owns credentials, pooling,
query execution and introspection on the user's machine; a Next.js PWA is the UI.
This model covers the browser/PWA, the local HTTP API, the embedded SQLite app
store, target database drivers, the filesystem/env, service-worker caches, export
files and logs.

Out of scope: multi-user/hosted deployment, network-reachable production use,
and supply-chain of the operating system. DataDeck is single-user by design.

## 2. Trust Boundaries

```
[ Browser / PWA ]                    untrusted UI; may be compromised by XSS/malicious page
        |  HTTP over loopback (application/json)
        v
[ Local Go HTTP API ]                trusted process boundary (no authn; loopback-trust)
        |  database/sql
        +--> [ Embedded SQLite app store ]   trusted local file (encrypted secrets + history)
        |
        +--> [ Driver layer: pgx / go-sql-driver/mysql / modernc sqlite ]
                    |
                    v
             [ PostgreSQL / MySQL / target SQLite ]   untrusted data source
```

Additional boundary surfaces:

- **Filesystem / env**: `ENCRYPTION_KEY`, `HOST`, `STORAGE_PATH`, target SQLite paths.
- **Service worker / Cache Storage**: browser cache for the app shell.
- **Exported files**: CSV/JSON written to the user's download location.
- **Logs**: stdout JSON logs.
- **Target database**: untrusted data (values, identifiers, DDL) crossing into the UI/export.

Boundary crossing rules (intended):

- Browser → API: all inputs untrusted; JSON body capped at 1 MiB; responses are
  sanitized envelopes. No credentials are ever returned.
- API → app store: only the repository layer touches the store; credentials are
  decrypted in memory only when a pool is opened.
- API → target DB: identifiers are quoted by the driver; values are parameterized
  by `database/sql`; result payloads are capped at 50 MB.
- API → browser: SQL error messages are surfaced (expected for a DB tool) but
  never DSNs or credentials.
- PWA → Cache Storage: only same-origin static/app-shell assets; never `/api`.

## 3. Assets

| Asset | Location | Sensitivity |
|---|---|---|
| Database passwords | app store, AES-256-GCM ciphertext; plaintext only in memory | High |
| Encryption key | `ENCRYPTION_KEY` env var; never persisted by the app | Critical |
| Connection metadata (host/port/db/user) | app store; API responses | Medium |
| Saved SQL / snippets | app store; API | Medium |
| Query history (SQL text, error messages, timing) | app store; API | Medium |
| Query results | in-memory in the browser; target DB | Medium–High (data) |
| Target SQLite paths | app store; API | Low–Medium |
| Exported CSV/JSON | user's filesystem (download) | Medium–High (data) |
| Logs | stdout | Low (must contain no secrets) |

## 4. Adversaries and Assumptions

- **A1 Local malicious process** running as the same user: can call the loopback
  API and read the store. Same-user trust is not defended (OS-level isolation).
- **A2 Malicious web page** in the user's browser: cannot read API responses due
  to CORS allowlist; JSON content type forces preflight for state changes.
- **A3 Untrusted database content** (values, column names, DDL): must not be able
  to inject formulas into exports, HTML into the DOM, or SQL into generated
  statements.
- **A4 Network peer** (only if bound beyond loopback): must not reach the API by
  default.
- **A5 Operator misconfiguration**: e.g., committing a real key, binding wide,
  pointing `STORAGE_PATH` somewhere shared.

## 5. Threats and Mitigations

| # | Threat | Asset | Boundary | Existing mitigation | Residual |
|---|---|---|---|---|---|
| T1 | Credential disclosure via API | passwords | API→browser | responses exclude secrets; no credentials serialized | none known |
| T2 | Credential disclosure via logs | passwords/DSNs | API→logs | request logger logs method/path/status/size only; driver errors do not include password | driver error text may include host/user/db |
| T3 | Key disclosure | key | env→process | key read from env, never logged/persisted; startup fails without valid key | env vars visible to same-user processes (A1) |
| T4 | Plaintext credential persistence | passwords | API→store | AES-256-GCM, fresh nonce per record, versioned format | none known |
| T5 | Malformed/malicious API input | daemon | browser→API | JSON body 1 MiB cap; typed decoding; validation errors sanitized | none known |
| T6 | Oversized response / memory exhaustion | daemon/browser | API→browser | 50 MB result cap with `truncated: true`; 30 s default timeout | long max timeout (300 s) holds a pool slot |
| T7 | Resource exhaustion (many queries) | daemon/DB | local | bounded pool sizes; timeout | no rate/concurrency cap per connection |
| T8 | Cross-connection execution | data integrity | API→DB | tab/pool keyed by connection id; execute uses the profile's driver+DSN | none known |
| T9 | Path misuse (SQLite target) | filesystem | API→FS | explicit user path only; normalized absolute; separate from app store | can open any readable file by design |
| T10 | Sensitive service-worker caching | API data | PWA→cache | worker returns early for `/api*`; only shell assets cached | none known |
| T11 | Export formula injection | user's spreadsheet | API→file | CSV cells starting `= + @ -` (non-numeric) prefixed with `'`; numerics untouched | heuristic, leading-whitespace bypass |
| T12 | Exported filename path traversal | filesystem | API→file | connection name sanitized; fixed pattern `datadeck_<conn>_<ts>.<ext>` | reserved device names not special-cased |
| T13 | Error information leakage | schema | API→browser | errors sanitized; no DSNs | DB SQL error messages/include schema names (expected for a DB tool) |
| T14 | Unsafe network exposure | all | daemon→network | default bind `127.0.0.1`; CORS allowlist; loopback flag logged | non-loopback binding is not blocked or explicitly confirmed |
| T15 | Target database outage | availability | API→DB | connection errors mapped to sanitized envelopes; pools retry on next use | pool not auto-evicted on failure |
| T16 | Abandoned pools/resources | daemon | internal | pool closed on connection delete and on shutdown (`CloseAll`) | no idle eviction for unused profiles |
| T17 | HTML injection via data | browser DOM | API→UI | React escapes all rendered values; no `dangerouslySetInnerHTML` | none known |
| T18 | Client-side credential persistence | passwords | PWA→storage | no localStorage/sessionStorage/IndexedDB/cookies used | none known |

## 6. Security Invariants

1. The daemon binds to `127.0.0.1` unless `HOST` is explicitly widened.
2. The daemon refuses to start without a valid 32-byte/64-hex `ENCRYPTION_KEY`.
3. Credentials are never returned by the API and never written to logs.
4. Every query runs under a `context.WithTimeout` (default 30 s).
5. Every result payload is capped at 50 MB with `truncated: true`.
6. 64-bit integers are serialized as strings.
7. The service worker never caches `/api` responses.
8. Export never re-executes SQL and never scrapes rendered DOM state.

## 7. Known Gaps (see audit)

- Internal store file permissions rely on umask (defense-in-depth).
- Non-loopback binding has no confirmation/guard despite no authentication.
- No per-connection concurrency/rate governance; max timeout holds pool slots.
- Export formula heuristic does not inspect leading whitespace/control chars.

These are tracked in `docs/reviews/m5-security-audit.md` and mapped to hardening
tasks.
