# DataDeck v0.2.0 — Known Limitations

## v0.2.0-specific
- **Version string remains `0.1.0`.** Version finalization (R02-style) and any
  `v0.2.0` tag/release are out of scope for this implementation task (not bumped, not
  tagged, not published).
- **Explorer shows one connection's tree at a time.** Multiple connections remain
  *active* concurrently and are usable via open tabs (each tab keeps its own
  `(connection, database)` binding and pool); selecting another connection switches the
  visible tree without retargeting tabs. A fully simultaneous *multi-tree* explorer
  (several connections expanded at once) is a future UX enhancement.
- **MySQL integration was not executed this session** (local MySQL server down). MySQL
  driver behavior is unchanged and covered by prior PRF-02 integration suites.
- **Native runtime verified on macOS arm64 only**; other platforms are build/format
  verified (carried from v0.1.0).
- **Idle janitor uses `database/sql` `Stats().InUse` as the active-operation reference**
  (no per-handler lease API). A pool with any in-use connection is never closed; a pool
  with only idle connections is reclaimable after the idle timeout.
- **Per-connection LRU cap** (`MaxPoolsPerConnection`, default 16) still bounds pool
  count in addition to the idle sweep.
- **Single janitor goroutine** (ticker), configurable; `POOL_JANITOR_INTERVAL=0`
  disables it.

## Carried from v0.1.0
No SSH tunneling, no general-purpose authentication/public hosting, no schema designer,
no MySQL restore, no SQLite database deletion, XLSX import deferred, base-tables-only
database export, raw SQL can mutate data, `ENCRYPTION_KEY` has no rotation.
