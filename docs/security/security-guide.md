# Security Guide

DataDeck v0.1.0 is a **local-first** tool. It is designed to run on your own
machine and is **not safe to expose to the public internet**.

## Local-first architecture

- The daemon binds to **loopback (`127.0.0.1`) by default**. Binding wider requires
  an explicit `ALLOW_REMOTE=1` (see [configuration](../getting-started/configuration.md)).
- There is **no general-purpose authentication** in v0.1.0. Anyone who can reach the
  port can use the app, so the boundary is the network binding, not a login.

## Credential storage & encryption

- Connection passwords are encrypted at rest with **AES-256-GCM** using your
  `ENCRYPTION_KEY`.
- The key is required at startup; an absent/invalid key aborts startup. There is no
  insecure fallback.
- Passwords are never returned by the API or shown in the UI, and are decrypted in
  memory only when a connection is opened.

### ENCRYPTION_KEY responsibility

- Supply exactly **32 raw bytes** or **64 hexadecimal characters**.
- **Store it safely and keep it stable.** Losing the key makes stored passwords
  undecryptable.
- **Rotation is not implemented** in v0.1.0. Changing the key without re-entering
  passwords will break existing profiles.
- Never commit the key or paste it into shared documents; `.env*` is gitignored and
  `.env.example` holds placeholders only.

## Database permissions

DataDeck runs with the credentials **you** provide and **cannot override** database
server permissions. Use the least privilege that works (see below).

## Raw SQL risks

The SQL editor executes arbitrary SQL with your credentials. It can modify or drop
data and is **not** automatically reversible. Review statements before running them.

## Import risks

- **Table import** is restricted to value-level `INSERT`s for one table (no arbitrary
  SQL) and runs atomically.
- **Database restore** accepts only DataDeck-exported artifacts, validated against an
  allowlist, into an **empty** target. It never executes arbitrary uploaded SQL
  against the general query path.
- Export files may contain sensitive data — store and share them accordingly.

## Destructive operations

[Delete Database](../user-guide/database-management.md#delete-database) uses a
two-phase, token-bound confirmation, a protected-database policy, and post-DROP
verification; it never force-terminates sessions and never auto-retries an
ambiguous outcome.

## Sensitive results & caching

- Query results may contain sensitive data; they live in the browser tab.
- API responses are served with `Cache-Control: no-store`.
- The PWA service worker **does not cache `/api` responses** (ADR-008), so sensitive
  database data is not persisted in the offline cache.

## Logs

Logs are structured and **credential-free**; DSNs, passwords, and encryption keys
are never logged. Destructive-operation audit lines exclude confirmation tokens.

## Backups

DataDeck does **not** manage backups. Before destructive actions — and generally —
take your own database backups and keep the local app store
(`STORAGE_PATH`) backed up alongside a copy of your `ENCRYPTION_KEY`.

## Shared-computer considerations

Because there is no authentication, anyone using the same OS account can open
DataDeck. Treat the machine and the app store file as sensitive.

## Least privilege

Recommended database credentials:

- Use a **read-only user** for exploration and browsing where possible.
- Grant **write** permissions only when you need row CRUD or import.
- **Avoid administrative/superuser** credentials except for database
  management/reconnect/delete, which require elevated rights.
- Use **separate profiles** for development and production.
- **Back up** before destructive operations.

## What DataDeck does not protect against

- Other processes/users on the same machine.
- A compromised or shared OS account.
- Exposing the port beyond loopback without additional network controls.
- Malicious SQL you choose to run.
