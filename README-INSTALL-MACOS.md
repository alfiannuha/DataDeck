# Installing DataDeck on macOS

DataDeck is distributed as a free, **unsigned** (ad-hoc signed) Apple Silicon
app. It is **not** notarized by Apple — we do not require a paid Apple Developer
Program membership — so macOS may warn you the first time you open it. This guide
shows how to install and launch it **without using Terminal**.

> Requires an Apple Silicon Mac (M1/M2/M3/…) running macOS 12 or later.

## Install

1. **Download the DMG** — from the releases page download
   `DataDeck-<version>-macos-arm64.dmg`. (A `.zip` is also provided; it contains
   the same app.)
2. **Open the DMG** — double-click it; a Finder window opens showing **DataDeck**
   and an **Applications** shortcut.
3. **Install** — drag **DataDeck** onto the **Applications** shortcut. Eject the
   disk image when done (right-click → Eject).

## First launch

4. **Open it** — from **Applications** (or Launchpad). Double-click **DataDeck**.
   DataDeck starts its local backend, then opens your default browser to the app.

5. **If macOS blocks it** ("Apple could not verify…" / "cannot be opened because
   the developer cannot be verified"), open **System Settings → Privacy &
   Security**, scroll to the notification about **DataDeck**, and click **Open
   Anyway**, then confirm. After that, DataDeck opens normally.

   On some macOS versions you can instead **right-click (or Control-click) the
   app → Open → Open**. Either way, you only need to do this once.

## Why does macOS show this warning?

macOS Gatekeeper verifies that apps are signed with a paid **Developer ID**
certificate and **notarized** by Apple. DataDeck is provided free and is not
enrolled in that program, so it is **not** notarized and macOS cannot
automatically vouch for it — hence the warning. The warning does not mean
malware was found; it means macOS could not verify the publisher.

We do **not** ask you to disable Gatekeeper. We also do **not** silently strip the
quarantine flag from your system; the "Open Anyway" step above is an explicit,
per-app decision you make.

## Optional: verify the download

Each release includes `checksums.txt` with the SHA-256 of the `.dmg` and `.zip`.
You can verify a download in Terminal (optional — not required to install):

```sh
shasum -a 256 DataDeck-<version>-macos-arm64.dmg
# compare the output with the matching line in checksums.txt
```

## Uninstall

Quit DataDeck, then drag **DataDeck** from **Applications** to the Trash.
Application data (connection profiles, history) lives in `~/.datadeck/`; delete
that folder too if you want a clean removal.

## Notes

- The app is **ad-hoc signed**, not Developer ID signed, and **not notarized**.
  These are different things; DataDeck does not claim notarization.
- Data is stored outside the app bundle, in `~/.datadeck/` (database + logs).
  Credentials are encrypted with a key held in your macOS Keychain.
- The backend binds to loopback (`127.0.0.1`) only.
