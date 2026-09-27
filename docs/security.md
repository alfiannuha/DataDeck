# DataDeck — Security

> Mandatory security rules for DataDeck. Requirement levels follow RFC 2119:
> **MUST**, **SHOULD**, **MAY**. Rules marked **Needs Validation** are not
> settled by `PRD.md` and must be confirmed before being treated as immutable.

---

## 1. Network Exposure

- The Go daemon **MUST** bind to `127.0.0.1` by default (PRD §11.1). It MUST NOT
  default to `0.0.0.0`.
- Binding to any non-loopback address **MUST** be an explicit, documented opt-in.
  Implemented as `ALLOW_REMOTE=1` (with `HOST` set to the wider address): startup
  **MUST** refuse a non-loopback `HOST` without it, and **MUST** log a warning
  when remote binding is enabled. Falsy/typo values fail safe (treated as not
  allowed).
- **MUST NOT** expose the daemon to public networks.
- **SHOULD** validate the `Host`/`Origin` of incoming requests to reduce
  DNS-rebinding and cross-origin risk from malicious local web pages.

## 2. Credentials

- Credentials (target DB passwords, SSH passwords) **MUST** be encrypted at rest
  with **AES-256-GCM** before being written to the embedded SQLite store
  (PRD §3, §11).
- Credentials **MUST NEVER** be returned through the API. List/read responses
  MUST omit or mask `encrypted_password` and `ssh_encrypted_password`.
- Credentials **MUST NEVER** be logged, included in error messages, included in
  panic recovery output, or written to the query-history table.
- Credentials **MUST** be decrypted only in memory, only when needed (connection
  establishment or connection test), and MUST NOT be cached in plaintext beyond
  the live pool/config.
- Full connection strings containing passwords **MUST NOT** be logged; redact
  userinfo from any DSN before logging.
- `query_history.sql_text` may contain user literals; history **SHOULD** be
  treated as sensitive local data and MUST NOT leave the machine by default.

## 3. Credential Encryption (AES-256-GCM)

- Algorithm: **AES-256-GCM** via Go stdlib `crypto/cipher` (PRD §3).
- Key length **MUST** be exactly 32 bytes (256 bits).
- A **unique nonce MUST** be generated for every encryption operation. Never
  reuse a nonce with the same key. Use `crypto/rand` for nonce generation.
- Nonce **MUST** be stored alongside the ciphertext (prepended or explicitly
  serialized) so decryption is possible; nonces are not secret but MUST be
  unique.
- GCM authentication tag **MUST** be verified on every decrypt; authentication
  failure MUST be treated as an error, never as empty plaintext.
- Encrypted payloads **SHOULD** be stored in a versioned format to permit future
  algorithm/key migration.
- **Key provisioning (implemented in M1-T04):** the key is supplied via the
  `ENCRYPTION_KEY` environment variable as **either** exactly 32 raw bytes
  **or** 64 hexadecimal characters decoding to 32 bytes. The application never
  generates, derives, or persists the key. **Needs Validation:** key rotation
  and re-encryption remain unspecified (see §4).

## 4. Key Management

- **MUST NOT** commit `ENCRYPTION_KEY` or any secret to Git.
- Secrets **MUST** be supplied via environment variables or an external secret
  mechanism; example values in docs/config **MUST** be clearly non-production.
- The daemon **MUST** fail fast with a clear error if the key is missing or not
  32 bytes, rather than silently falling back to an insecure default.
  Implemented: `security.NewCipher` rejects missing/incorrect keys at startup.
- **SHOULD** support key rotation. **Needs Validation:** rotation and
  re-encryption procedure are unspecified.

## 5. Secrets in Version Control

- No secrets, keys, tokens, database dumps, or `.env` files **MUST** be committed.
- `.gitignore` **MUST** exclude local app storage (`data/`, `*.db`), `.env*`
  (except committed examples), and build artifacts.
- The Docker example's `ENCRYPTION_KEY=0123...` is illustrative only and **MUST
  NOT** be used or deployed as a real key.

## 6. Safe Error Handling

- Internal errors **MUST** be sanitized before being returned to clients: no
  stack traces, no driver internals containing connection strings, no secrets.
- Panics **MUST** be recovered by middleware and returned as `INTERNAL_ERROR`.
- SQL error messages MAY be returned to the client (the user is the data owner)
  but **MUST** be scrubbed of embedded credentials/DSNs.
- Logs **MUST** be sanitized (see §10).

## 7. CORS Policy

- Because the frontend and backend are separate origins (`:3000` and `:8080`),
  CORS is required for the decoupled dev/Docker modes.
- Allowed origins **MUST** be an explicit allowlist, not `*`, when credentials or
  sensitive data are involved.
- **SHOULD** allow only the known frontend origin(s):
  - dev/Docker: `http://localhost:3000` (and `http://127.0.0.1:3000`).
  - single-binary: same origin — no CORS needed.
- Allowed methods **MUST** be limited to those used (`GET`, `POST`, `PUT`,
  `DELETE`, `OPTIONS`). `PUT` is required for saved-query updates.
- Allowed headers **SHOULD** be limited to `Content-Type` and (if adopted)
  `X-Request-ID`.
- **Needs Validation:** the exact allowlist and whether it is configurable via
  environment.
- API responses **MUST** be marked `Cache-Control: no-store` and
  `X-Content-Type-Options: nosniff`; a `Referrer-Policy: no-referrer` header
  **SHOULD** be set. HSTS/CSP/X-Frame-Options are deliberately omitted (the
  daemon is loopback HTTP and serves no HTML).

## 8. Request Body Limits

- The backend **MUST** enforce a maximum request body size to prevent memory
  exhaustion (e.g., via `http.MaxBytesReader`).
- The concrete limit is **1 MiB** for every JSON endpoint (`maxBodyBytes`).
  Oversized bodies are rejected with `400 VALIDATION_ERROR` and an explicit
  size message; the envelope contract defines no `413`/`PAYLOAD_TOO_LARGE` code.
- `Content-Type` **MUST** be `application/json`; other or missing types are
  rejected with `400 VALIDATION_ERROR`. This blocks cross-origin simple-request
  CSRF (a form/text POST cannot set `application/json` without a preflight).
- Exactly one JSON value per body is accepted; trailing values are rejected.

## 9. Query Timeout, Result Cap & Destructive Queries

- Every query **MUST** run inside `context.WithTimeout` (default **30 seconds**,
  PRD §11.3).
- The client MAY request a custom `timeout_seconds`; the server **MUST** clamp it
  to a safe maximum. **Needs Validation:** the maximum value is unspecified.
- Queries returning more than **50 MB** of raw payload **MUST** be truncated and
  MUST return `"truncated": true` (PRD §11.2) rather than crashing or exhausting
  memory on client or server.
- **Destructive queries** (e.g., `DROP TABLE` from the schema context menu) are
  executed because the user explicitly requests them. The backend **SHOULD NOT**
  silently block them, but the frontend **MUST** present explicit confirmation
  for destructive actions (context menu: *Drop Table*). **Needs Validation:**
  whether the backend requires any server-side confirmation for destructive DDL.
- The daemon **MUST NOT** auto-run user SQL on startup, on connection activation,
  or from history without an explicit user action.

## 10. Log Sanitization

- Logs **MUST** redact: passwords, SSH passwords, `ENCRYPTION_KEY`, auth headers,
  and DSN userinfo.
- Request/response bodies **MUST NOT** be logged wholesale; if logged at all,
  sensitive fields **MUST** be masked.
- Structured logging **SHOULD** be used for consistent redaction.
- **SHOULD** include the (optional) request ID for correlation rather than full
  payloads.

## 11. Local Data Protection

- The embedded SQLite store (`datadeck.db`) contains encrypted secrets and
  query history; it **SHOULD** live in a user-private directory
  (`~/.datadeck/`) with restrictive file permissions where the OS supports it.
  The backend creates the parent directory `0700` and best-effort narrows the
  database (and `-wal`/`-shm`) files to `0600`.
- **MUST NOT** sync or transmit the app store off-device by default.
- **Target SQLite paths are user-explicit.** A SQLite target connection
  requires an explicit file path (no host/credentials); the path is normalized
  to absolute and opened only where configured. The connector never defaults to
  DataDeck's own application database, and no filesystem-browsing API is
  exposed.
- Backups/exports **SHOULD** exclude encrypted secrets unless re-encrypted.

## 12. CSV Export Formula Injection

Data exported from an untrusted database may be opened in a spreadsheet, where
text beginning with `=`, `+`, `-`, or `@` can be interpreted as a formula or
command.

- **MUST** neutralize formula-leading text cells in CSV export by prefixing a
  single quote (`'`); this is an intentional, documented transformation, not
  silent corruption.
- **MUST NOT** prefix numeric values: a leading `-` followed by a digit is a
  legitimate negative number/BIGINT and is exported unchanged to preserve
  exactness.
- JSON export is not affected (JSON is data, not executed by spreadsheets).
- The policy is implemented once in the shared export layer
  (`frontend/src/lib/export/values.ts`, `guardFormulaExport`) and covered by
  tests.

## 13. Dependency & Supply Chain

- **SHOULD** pin dependency versions and review backend driver dependencies
  (`pgx`, `go-sql-driver/mysql`, `modernc.org/sqlite`) for known CVEs.
- **MUST** build the distributed binary with `CGO_ENABLED=0` (PRD §9.1) to keep
  the artifact self-contained and reduce native attack surface.
- **SHOULD** run dependency/vulnerability scanning in CI (see
  `testing-strategy.md`).

## 14. Summary Requirement Table

| Rule | Level | Source |
|---|---|---|
| Bind `127.0.0.1` by default | MUST | PRD §11.1 |
| Never log credentials | MUST | Security baseline |
| Never return credentials via API | MUST | Security baseline |
| AES-256-GCM for credential storage | MUST | PRD §3, §11 |
| Unique nonce per encryption | MUST | GCM requirement |
| 32-byte encryption key | MUST | PRD §3 |
| No secrets in Git | MUST | Security baseline |
| Sanitized errors | MUST | Security baseline |
| Explicit CORS allowlist | SHOULD | Derived |
| Request body limit | MUST | Memory safety |
| 30s default query timeout | MUST | PRD §11.3 |
| 50 MB result truncation | MUST | PRD §11.2 |
| Confirm destructive actions in UI | SHOULD | PRD §7.2 |
| Neutralize CSV formula injection | MUST | Security baseline |
| Warn on exporting truncated result | SHOULD | PRD §11.2 |
| Log sanitization | MUST | Security baseline |
