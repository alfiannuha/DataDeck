# DataDeck v0.1.0 — R05 Security Regression (CSRF / DNS Rebinding / Encryption)

## R04 CSRF/Origin middleware (regression)
Dynamic matrix against the running binary (loopback):

| Request | Result |
|---|---|
| same-origin GET | 200 |
| same-origin POST (JSON) | passes CSRF → 400 validation |
| cross-origin POST (JSON, `Origin: https://evil.example`) | **403** `CROSS_ORIGIN_REJECTED` |
| cross-origin multipart upload | **403** |
| no Origin (non-browser/CLI) | allowed → 400 validation |
| null Origin | **403** |
| OPTIONS preflight | 204, **no ACAO** |

Cross-origin `GET` returns 200 without `Access-Control-Allow-Origin` → browser
cannot read the response. Unit tests: `csrf_test.go`. Full browser E2E 10/10 (proves
same-origin mutations are not weakened).

## DNS rebinding assessment (R04-FIND-001)
Reproduced with `Host: attacker.example`, `Origin: http://attacker.example`:
- multipart upload to a **valid** connection id → reached handler (400) — residual.
- multipart upload to an **unknown** id → 404 (ids are random 128-bit; not enumerable).
- JSON mutation → reaches the handler via curl, but a **browser** would be blocked by
  the CORS preflight (non-allowlisted origin → no ACAO).

**Classification: MEDIUM** (not BLOCKER/HIGH): exploitation requires prior knowledge
of an unguessable connection id, the response is unreadable, and browser JSON
mutations are preflight-blocked. Full browser rebinding exploit **NOT RUN**.

## Credential encryption
AES-256-GCM (random nonce, tag verified); required key (32B/64hex); missing/invalid
key **aborts startup** (verified). Wrong key / corrupt ciphertext → controlled
`ErrInvalidCiphertext`, no plaintext. No rotation (documented). Unit:
`internal/security/cipher_test.go`.

## SQL injection / mutation safety
Structured APIs use metadata-validated, dialect-quoted identifiers and bound values;
PK-only identity + optimistic concurrency; unchanged since T19/T20. PASS.

## Localhost boundary
Loopback-only listener; non-loopback bind refused without `ALLOW_REMOTE=1`. PASS.
