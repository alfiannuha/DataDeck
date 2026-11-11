# DataDeck v0.1.0 — R09 Platform Runtime Matrix

Subject: **published** GitHub Release assets (not local builds). Host: macOS 26.6.2 arm64.

| Platform | Download | SHA-256 | Extract | Format/arch | CLI | Startup | Health | UI | Shutdown |
|---|---|---|---|---|---|---|---|---|---|
| darwin/arm64 | PASS | PASS | PASS | Mach-O arm64 | PASS | PASS | PASS | PASS | PASS |
| darwin/amd64 | PASS | PASS | PASS | Mach-O x86_64 | NOT RUN | NOT RUN | NOT RUN | NOT RUN | NOT RUN |
| linux/amd64 | PASS | PASS | PASS | ELF x86-64 | NOT RUN | NOT RUN | NOT RUN | NOT RUN | NOT RUN |
| linux/arm64 | PASS | PASS | PASS | ELF aarch64 | NOT RUN | NOT RUN | NOT RUN | NOT RUN | NOT RUN |
| windows/amd64 | PASS | PASS | PASS | PE32+ x86-64 | NOT RUN | NOT RUN | NOT RUN | NOT RUN | NOT RUN |

## darwin/arm64 (native, executed)
- Extracted `datadeck` from `datadeck_0.1.0_darwin_arm64.tar.gz`.
- `./datadeck --version` → `datadeck 0.1.0 (commit 6d99856…, built 2026-10-13T02:15:03Z, channel release)`; exit 0; no server, no key.
- Start (disposable key/store) → listener `127.0.0.1:8099`; `/health` → `0.1.0`/`release`; `/` 200; `/manifest.json` 200; `/sw.js` 200.
- SIGTERM → clean exit; listener closed.

## Non-mac runtime
Only macOS arm64 was executed natively. darwin/amd64, linux/amd64, linux/arm64,
windows/amd64 are **download + hash + format/arch verified only** → runtime
**NOT VERIFIED** (no native runners). Carried to R10; cross-compilation is not
claimed as runtime PASS.
