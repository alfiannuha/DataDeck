# DataDeck v0.1.0 — API / Localhost / Origin Security

## Localhost binding
- Default `HOST=127.0.0.1` (`config.DefaultHost`). Live check: `TCP 127.0.0.1:8099 (LISTEN)` — no public listener.
- A non-loopback `HOST` is **refused at startup** unless `ALLOW_REMOTE=1`
  (verified: `HOST=0.0.0.0` → refusal message mentioning "no authentication").
- `IsLoopback()` treats `localhost` and IP loopback forms as loopback.

## CORS (`middleware/cors.go`)
- Explicit origin allowlist (`CORS_ALLOWED_ORIGINS`, default
  `http://localhost:3000,http://127.0.0.1:3000`).
- Only allowlisted origins receive `Access-Control-Allow-Origin`; others get none.
- Methods/headers are restricted to those the API uses.

## CSRF (`middleware/csrf.go`) — new in R04
CORS alone is insufficient: a cross-origin `multipart/form-data` POST is a CORS
"simple request" that executes server-side without a preflight. The CSRF middleware
rejects any **unsafe method** (not GET/HEAD/OPTIONS) whose `Origin` is neither
**same-origin** (Origin host == request Host) nor allowlisted.

Dynamically verified:
| Request | Result |
|---|---|
| `POST` cross-origin JSON (`Origin: https://evil.example`) | **403** `CROSS_ORIGIN_REJECTED` |
| `POST` cross-origin multipart (file upload) | **403** |
| `POST` same-origin (`Origin: http://127.0.0.1:8099`) | passes CSRF (400 validation) |
| `POST` no Origin (curl/CI/non-browser) | passes CSRF (400 validation) |
| `GET` cross-origin | 200 but no ACAO → browser cannot read the response |

Unit tests: `middleware/csrf_test.go`. Full browser E2E 10/10 passes (same-origin).

## HTTP headers (`middleware/security.go`)
`X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `Referrer-Policy:
no-referrer` on every response. CSP/X-Frame are deliberately omitted for the JSON
API; the static HTML served by the binary does not set them (pre-existing LOW
finding — see `findings-register.md`).

## Authentication boundary
No general-purpose authentication exists (explicit in docs). The control is the
loopback boundary plus Origin enforcement; the app must not be exposed publicly
(and startup refuses wider binds without an explicit opt-in).

## Residual
**DNS rebinding** (attacker origin == attacker Host) is not fully blocked by the
same-origin rule; mitigated by unguessable connection IDs and unreadable
cross-origin responses (MEDIUM, documented).
