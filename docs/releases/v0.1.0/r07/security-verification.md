# DataDeck v0.1.0 — R07 Security Verification

Verified against the RC binary `datadeck_0.1.0_darwin_arm64` (SHA `dfe8411d…`).

| Control | Test | Result |
|---|---|---|
| Loopback binding | listener | `127.0.0.1:8095` only (no public listener) |
| CSRF/Origin present in binary | cross-origin JSON POST (`Origin: https://evil.example`) | **403** |
| CSRF same-origin | same-origin POST | passes CSRF → 400 validation |
| ENCRYPTION_KEY missing | startup | aborts ("encryption key is not configured") |
| ENCRYPTION_KEY invalid | startup | aborts ("expected 32 raw bytes or 64 hex") |
| Version/channel | `/health` | `0.1.0` / `rc`, no secrets exposed |
| Unexpected outbound | process sockets at startup | none (1 = listener) |
| Secret scan (binaries) | `strings` for PEM/AWS/placeholder key | **0** hits |
| Secret scan (archives) | entries for `.env/.db/.pem/.key` | **0** |

## DNS rebinding (R04-FIND-001 / R05 disposition)
Re-verified independently (R05): a rebinding request with `Host=Origin=attacker.tld`
reaches a multipart endpoint **only** with a valid, unguessable 128-bit connection id
(unknown id → 404); cross-origin JSON mutations are blocked by CORS preflight in the
browser; responses are unreadable cross-origin. No practical browser exploit path was
demonstrated. **Classification retained: MEDIUM** (accepted) — not escalated.

## Provenance trust
`provenance.json` is **unsigned, self-reported** metadata produced by the trusted
local build. It is not a signed attestation; there is no Sigstore/in-toto signature.
Stated explicitly rather than claimed as cryptographically trusted.
