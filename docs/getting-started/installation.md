# Installation

DataDeck v0.1.0 ships as a **single self-contained binary** (Go backend + embedded
static frontend). No database server or Node.js runtime is required to run it —
you only need access to the databases you want to connect to.

## Supported platforms

| OS | Architecture | Artifact name |
|---|---|---|
| macOS | arm64 (Apple Silicon) | `datadeck_0.1.0_darwin_arm64` |
| macOS | amd64 (Intel) | `datadeck_0.1.0_darwin_amd64` |
| Linux | amd64 | `datadeck_0.1.0_linux_amd64` |
| Linux | arm64 | `datadeck_0.1.0_linux_arm64` |
| Windows | amd64 | `datadeck_0.1.0_windows_amd64.exe` |

> **Artifacts are not yet published.** Download links and checksums are produced
> by a later release step. Until then, build from source:
> [../development/building.md](../development/building.md).

## Requirements

- A supported OS/architecture above.
- A reachable PostgreSQL, MySQL, or SQLite target (optional at first launch).
- An `ENCRYPTION_KEY` (see [configuration](configuration.md)) — **required** at
  startup; DataDeck aborts without a valid key.

## macOS

```bash
chmod +x datadeck_0.1.0_darwin_arm64
export ENCRYPTION_KEY="$(openssl rand -hex 32)"   # keep this safe
./datadeck_0.1.0_darwin_arm64
```

If macOS Gatekeeper blocks an unsigned binary, use **System Settings → Privacy &
Security → "Open Anyway"** (do not disable OS protections globally). Open
`http://127.0.0.1:8080`.

## Linux

```bash
chmod +x datadeck_0.1.0_linux_amd64
export ENCRYPTION_KEY="$(openssl rand -hex 32)"
./datadeck_0.1.0_linux_amd64
```

There is **no systemd unit or distro package** in v0.1.0; run the binary directly.
Open `http://127.0.0.1:8080`.

## Windows

```powershell
$env:ENCRYPTION_KEY = "<your 64-hex key>"
.\datadeck_0.1.0_windows_amd64.exe
```

There is **no MSI/installer** in v0.1.0. Run it from a terminal (a console window
shows logs). Windows Defender/SmartScreen may warn about an unsigned binary;
choose "More info → Run anyway" if you trust the source. Open
`http://127.0.0.1:8080`.

## First launch

1. The daemon starts and listens on `127.0.0.1:8080` (configurable — see
   [configuration](configuration.md)).
2. Open the URL in your browser.
3. You land in an **empty workspace**; no query tab is created automatically.
4. Create a connection profile and browse.

## Shutdown

Stop the process (Ctrl+C in the terminal, or close the console window). DataDeck
has no background service; closing the process stops the server.

## Configuration persistence

Local app state (connection profiles, query history, saved queries) is stored in
an embedded SQLite file at `STORAGE_PATH` (default `./data/datadeck.db`). See
[configuration](configuration.md).

## Common installation problems

- **"ENCRYPTION_KEY is required"** — set a valid 32-byte / 64-hex key.
- **Port already in use** — set `PORT` to a free port.
- **Browser cannot connect** — confirm the daemon is running and you opened the
  exact host/port it logs; DataDeck binds loopback only by default.
- **macOS/Windows security prompt** — see the notes above.
