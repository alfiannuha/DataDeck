# ADR-003: AES-256-GCM Credential Encryption

## Status

Accepted

## Context

DataDeck persists target database passwords (and SSH passwords) in its local
SQLite store so users do not retype them each session. These secrets must be
protected at rest, and the encryption must be available in a pure-Go,
single-binary distribution with minimal dependencies. The PRD specifies
`crypto/cipher` (AES-256-GCM, Go stdlib) using a 32-byte secret key, with
`ENCRYPTION_KEY` supplied via environment (PRD §3, §9.2).

## Decision

Encrypt credentials at rest with AES-256-GCM using Go's standard library:

- 32-byte (256-bit) key, required; missing/invalid length is a fatal startup
  error.
- A unique random nonce (`crypto/rand`) per encryption operation, stored with the
  ciphertext.
- GCM authentication tag verified on every decrypt; authentication failure is an
  error, never empty plaintext.
- Only the `security` package performs encryption/decryption.
- Plaintext credentials exist only transiently in memory.
- Ciphertext is stored in `encrypted_password` / `ssh_encrypted_password`.

## Consequences

- Authenticated encryption protects both confidentiality and integrity of stored
  credentials.
- No external crypto dependency; stdlib only.
- Key management becomes the critical operational concern: loss of the key makes
  stored profiles unrecoverable; leakage compromises them.
- Nonce uniqueness is mandatory; nonce is stored alongside ciphertext.
- The design is symmetric and local; there is no server-side key escrow.

## Alternatives Considered

- **AES-256-CBC with HMAC.** Rejected: two primitives to combine correctly; GCM
  provides authenticated encryption in one.
- **ChaCha20-Poly1305.** Viable, but PRD explicitly names AES-256-GCM.
- **OS keychain (Keychain/DPAPI/libsecret).** Stronger UX, but platform-specific
  and not specified by the PRD; adds complexity to a cross-platform single
  binary.
- **Hashing passwords.** Rejected: credentials must be recoverable to connect to
  the target DB, so they must be encrypted, not hashed.
- **No encryption.** Rejected: PRD mandates encryption.

## Constraints

- Key MUST be exactly 32 bytes and MUST NOT be committed to Git.
- A unique nonce MUST be used for every encryption.
- Only in-memory plaintext is permitted; never log or return plaintext.
- **Needs Validation:** key derivation (raw bytes vs. passphrase KDF), key
  generation UX, rotation/re-encryption procedure, multi-instance behavior, and
  the on-disk serialization format of nonce+ciphertext.
