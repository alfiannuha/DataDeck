# DataDeck v0.1.0 — R09 Security Smoke (published binary)

| Control | Test | Result |
|---|---|---|
| Loopback binding | listener | `127.0.0.1:8099` only |
| Missing ENCRYPTION_KEY | startup | aborts ("encryption key is not configured") |
| Health metadata | `/health` | `0.1.0` / `release`; no secrets |
| Security headers | `/health` | `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` |
| CSRF cross-origin | `POST` with `Origin: https://evil.example` | **403** |
| CSRF same-origin | `POST` with same-origin | passes → 400 validation |
| PWA | `/manifest.json`, `/sw.js` | 200 (SW bypasses `/api` per ADR-008) |
| Unexpected outbound | process sockets | none (listener only) |

## DNS rebinding (R04-FIND-001)
Disposition unchanged from R05/R07: a rebinding request reaches a multipart endpoint
only with a valid unguessable 128-bit connection id; browser JSON mutations are
CORS-preflight-blocked; responses are unreadable cross-origin. **MEDIUM retained**;
no practical browser exploit path demonstrated.

## Documentation/metadata finding
The published **frontend UI** title shows `development · unknown` (build channel =
`development`), while the backend reports `release`. Root cause: `release.yml` runs
`npm run build:static` without `NEXT_PUBLIC_DATADECK_CHANNEL/_COMMIT`. Version is
correct (`v0.1.0`); only the channel/commit labels are stale. Not a security issue
(R09-F-001).

No credential/secret was exposed in any test.
