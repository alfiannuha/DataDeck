# DataDeck v0.1.0 — R07 Runtime Smoke Matrix

Approved source `f92ce49`. Native host: macOS 26.6.2 arm64.

| Platform | Compile | Binary inspection | CLI | Startup | Health | UI | Shutdown |
|---|---|---|---|---|---|---|---|
| darwin/arm64 | PASS | PASS (Mach-O arm64) | PASS | PASS | PASS (`0.1.0`/`rc`) | PASS (200 + JS asset 200) | PASS (clean) |
| darwin/amd64 | PASS | PASS (Mach-O x86_64) | NOT RUN | NOT RUN | NOT RUN | NOT RUN | NOT RUN |
| linux/amd64 | PASS | PASS (ELF x86-64) | NOT RUN | NOT RUN | NOT RUN | NOT RUN | NOT RUN |
| linux/arm64 | PASS | PASS (ELF aarch64) | NOT RUN | NOT RUN | NOT RUN | NOT RUN | NOT RUN |
| windows/amd64 | PASS | PASS (PE32+ x86-64) | NOT RUN | NOT RUN | NOT RUN | NOT RUN | NOT RUN |

Native verification: **macOS arm64 only**. No Linux/Windows/mac-Intel runners are
available in this environment → runtime **NOT VERIFIED** for those targets (documented
limitation carried to R08; cross-compilation is not claimed as runtime PASS).

## Native smoke detail (darwin/arm64, SHA `dfe8411d…`)
- `--version` → `datadeck 0.1.0 (commit f92ce49ee1297a…, built 2026-10-13T00:17:33Z, channel rc)`, exit 0, no key/server.
- Listener `127.0.0.1:8095` (loopback only).
- `GET /api/v1/health` → `{"status":"healthy","version":"0.1.0","channel":"rc"}`.
- `/` → 200 HTML; `/_next/static/chunks/webpack-*.js` → 200.
- Outbound connections during startup: none (only the listening socket).
- SIGTERM → clean shutdown.
- Missing/invalid `ENCRYPTION_KEY` → startup aborts with a sanitized message.
