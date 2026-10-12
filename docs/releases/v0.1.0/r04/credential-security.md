# DataDeck v0.1.0 — Credential & Key Security

## AES-256-GCM (`internal/security/cipher.go`)
- Standard-library crypto only (`crypto/aes`, `crypto/cipher`, `crypto/rand`); no custom crypto.
- **Fresh random nonce per encryption** (`rand.Read`); nonce is never caller-supplied or reused.
- Ciphertext format: `base64(version || nonce || ciphertext || tag)`; a format byte
  allows deliberate migration.
- `Decrypt` verifies the GCM **authentication tag**; malformed input, unsupported
  version, short payloads, wrong key, or tampering all return `ErrInvalidCiphertext`
  with **no plaintext and no echoed value**.

## ENCRYPTION_KEY handling
- Required at startup; accepted as **exactly 32 raw bytes** or **64 hex chars**;
  anything else is rejected (`ErrInvalidKey`). Verified dynamically: missing key →
  `encryption key is not configured`; short key → `expected 32 … or 64 …`; startup aborts.
- **No insecure default, no plaintext fallback.** The key is never generated,
  derived, or persisted by the app.
- Never exposed via API, CLI (`--version` prints only metadata), logs, or binaries
  (injected only as process env at runtime; not compiled in).
- **Rotation is not implemented** (documented in the security guide); changing the
  key breaks existing ciphertext until profiles are recreated.

## Storage
- Profiles persist only the encrypted blob; passwords are decrypted in memory when
  a connection is opened.
- App store file `0600` inside a `0700` directory.

## Failure testing (disposable fixtures; unit tests)
| Case | Result |
|---|---|
| Missing key | rejected at construction; startup aborts |
| Invalid key length | rejected |
| Wrong key (decrypt valid ciphertext) | `ErrInvalidCiphertext`, no plaintext |
| Corrupted ciphertext/tag/nonce | `ErrInvalidCiphertext` |
| Truncated payload | `ErrInvalidCiphertext` |

Evidence: `internal/security/cipher_test.go`, `internal/config` validation, live
startup checks. No credential values appear in any log or report.
