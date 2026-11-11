# DataDeck for macOS (Apple Silicon)

Build a native `DataDeck.app` that starts the existing Go backend and opens the
existing DataDeck interface in your default browser. No Terminal required for
end users.

## Build

```bash
make macos-app                                # -> release/DataDeck-<version>-macos-arm64.app
make macos-package                            # -> .app + .dmg + .zip + checksums.txt + install guide
# (make macos-dmg is an alias for macos-package)
```

`scripts/build-macos-app.sh` (pass `--package` for the full release set):
1. builds the Next.js static export and copies it into `backend/internal/web/dist/`;
2. compiles the Go backend for `darwin/arm64` (`CGO_ENABLED=0`, version/commit/date/channel injected);
3. compiles the Swift launcher (`desktop/macos/Sources/DataDeckLauncher.swift`);
4. assembles the `.app` with `Info.plist`, `AppIcon.icns`, the launcher and the backend;
5. validates the plist, executables and arm64 architecture, checks the backend `--version`, and runs the launcher `--self-test`;
6. `--package` additionally produces `DataDeck-<version>-macos-arm64.dmg` (via `hdiutil`),
   `DataDeck-<version>-macos-arm64.zip` (via `ditto`, preserving bundle structure/permissions),
   `checksums.txt` (SHA-256 of the dmg + zip) and a copy of `README-INSTALL-MACOS.md`.

It does not touch `scripts/build-release.sh` (cross-platform binaries) or `make build`.

### Release artifacts (free / unsigned distribution)

| File | Notes |
|---|---|
| `DataDeck-<version>-macos-arm64.app` | the bundle (distributed inside the `.zip`/`.dmg`) |
| `DataDeck-<version>-macos-arm64.dmg` | drag-to-Applications installer |
| `DataDeck-<version>-macos-arm64.zip` | `ditto` archive of the bundle |
| `checksums.txt` | SHA-256 of the dmg + zip |
| `README-INSTALL-MACOS.md` | end-user install guide (no Terminal) |

## Bundle layout

```
DataDeck-<version>-macos-arm64.app/Contents/
├── Info.plist            CFBundleIdentifier io.datadeck.desktop, name/executable DataDeck
├── MacOS/DataDeck        Swift AppKit launcher
└── Resources/
    ├── AppIcon.icns
    └── backend/datadeck  Go backend (embedded frontend)
```

The bundle directory is version/platform-suffixed; the app display name and
executable remain **DataDeck**.

## Runtime behavior (launcher lifecycle)

1. Acquire a single-instance lock (`flock` on `~/.datadeck/.instance.lock`); a second
   launch brings the existing instance's URL forward instead of starting a second backend.
2. Create `~/.datadeck/` (`0700`) with `datadeck.db`, `logs/`, `config/`.
3. Retrieve the encryption key from the macOS **Keychain** (service
   `io.datadeck.desktop`); generate 32 random bytes → 64 hex on first launch only.
4. Choose a free **loopback** port (default 8080; scans upward if busy).
5. Start the backend with an explicit environment: `HOST=127.0.0.1`, `PORT`,
   `STORAGE_PATH=~/.datadeck/datadeck.db`, `ENVIRONMENT=production`, `LOG_LEVEL=info`,
   `ENCRYPTION_KEY` (env only — never argv). `ALLOW_REMOTE` is never set.
6. Poll `GET /api/v1/health` until ready (bounded), watching for early exit.
7. Open `http://127.0.0.1:<port>/` in the default browser.
8. On **Quit DataDeck** (or logout), send `SIGTERM` so the Go server exits gracefully
   (`server.Shutdown`), force-kill only after a bounded grace period.

There is no app window; the UI runs in the browser. Use **DataDeck → Quit DataDeck**
(⌘Q) to stop the backend.

## Data & logging

Persistent data lives **outside** the bundle (survives app replacement): `~/.datadeck/`.
`logs/launcher.log` (launcher events) and `logs/backend.log` (backend stdout/stderr,
JSON). Neither ever contains the encryption key, credentials, or raw secrets.

## Install

1. `make macos-package` (or `make macos-app` for the bundle only).
2. Open `release/DataDeck-<version>-macos-arm64.dmg` and drag the app into **Applications**.
3. Launch from Finder/Launchpad/Dock. End-user instructions (including the first-launch
   Gatekeeper steps) live in [`README-INSTALL-MACOS.md`](../../README-INSTALL-MACOS.md).

Dev validation without installing: `open release/DataDeck-<version>-macos-arm64.app`.

## Gatekeeper (ad-hoc signed, not notarized)

The bundle is **ad-hoc signed but not notarized** (no Apple Developer ID), so a
downloaded copy is blocked by Gatekeeper. On macOS 15 (Sequoia) the message is
*“Apple could not verify "DataDeck" is free of malware…”* and the old
right-click → Open bypass no longer works. Use one of:

- **Terminal (recommended):**
  ```bash
  xattr -dr com.apple.quarantine /Applications/DataDeck.app
  ```
  (or whatever path you installed it to). Removing the quarantine attribute
  skips Gatekeeper assessment; the ad-hoc signature satisfies the kernel.
- **System Settings → Privacy & Security →** scroll to the *“DataDeck” was
  blocked* entry → **Open Anyway** → confirm.

Do not disable macOS security globally. This build is **not** notarized and is
not ready for unattended public distribution.

### Optional Developer ID signing / notarization

The default build is **ad-hoc signed** (integrity only) and **not notarized** —
this is *not* Developer ID signing and carries no Apple identity. A clean,
warning-free public build requires a paid Apple Developer account (Developer ID
Application certificate + notary credentials). The build script already supports
it; enable it by exporting:

```bash
export MACOS_SIGN_IDENTITY="Developer ID Application: <name> (<TEAMID>)"
export MACOS_NOTARY_PROFILE="<notarytool keychain profile>"
make macos-package
```

When `MACOS_SIGN_IDENTITY` is unset the script ad-hoc signs; when
`MACOS_NOTARY_PROFILE` is unset it skips notarization. In CI the same two values
come from repository secrets (`MACOS_SIGN_IDENTITY`, `MACOS_NOTARY_PROFILE`) and
are **optional** — the workflow succeeds without them. Never commit the
certificate, key, or notary password.

## Encryption key migration (important)

The packaged app uses a **Keychain-held** key and `~/.datadeck/datadeck.db`. If you
previously ran DataDeck with a different `ENCRYPTION_KEY` and/or `STORAGE_PATH`
(e.g. a dev run with a repo `.env` and `./data/datadeck.db`), the packaged app does
**not** adopt them:

- It never reads or modifies your repo `.env`.
- A **new** Keychain key cannot decrypt credentials encrypted with the old key.
- To reuse existing encrypted data, keep the original key: set `ENCRYPTION_KEY` for a
  dev run, or perform an explicit re-encryption/migration. Never assume the new key
  decrypts old ciphertext.

## Tests

- `DataDeck.app/Contents/MacOS/DataDeck --self-test` — pure checks: default port,
  `~/.datadeck` paths, backend environment construction (loopback, no `ALLOW_REMOTE`,
  key via env), hex encoding, generated key is 64 hex chars. (Keychain read/create and
  network are exercised by the running app, not the self-test.)
- The build script validates the plist, executables, architecture, backend version, and
  runs the self-test.

## Known limitations

- Apple Silicon (`arm64`) only; no universal/Intel slice.
- Ad-hoc signed but not notarized (see above).
- No `.app` auto-update; replacing the bundle preserves `~/.datadeck/`.
- If the launcher is force-quit, the backend receives no SIGTERM; it exits when its
  parent/stdout closes and the next launch re-acquires the lock. Storage is a single
  SQLite file, so a second backend would fail fast on lock rather than corrupt data.
