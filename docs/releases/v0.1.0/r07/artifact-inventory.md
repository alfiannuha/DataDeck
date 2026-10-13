# DataDeck v0.1.0 — R07 Artifact Inventory & Integrity

Source `f92ce49`; all values independently recomputed from artifact bytes.

## Binaries
| File | Size | SHA-256 (independently computed) | Format / arch | checksums | provenance |
|---|---|---|---|---|---|
| `datadeck_0.1.0_darwin_arm64` | 24,217,202 | `dfe8411d…f3ecc` | Mach-O arm64 | ✓ | ✓ |
| `datadeck_0.1.0_darwin_amd64` | 25,638,640 | `d0a5a387…ccf93` | Mach-O x86_64 | ✓ | ✓ |
| `datadeck_0.1.0_linux_amd64` | 25,133,218 | `44939859…1d374` | ELF x86-64 (static) | ✓ | ✓ |
| `datadeck_0.1.0_linux_arm64` | 23,658,658 | `62513343…51722` | ELF aarch64 (static) | ✓ | ✓ |
| `datadeck_0.1.0_windows_amd64.exe` | 25,721,344 | `5e2f1cfc…f91b9` | PE32+ x86-64 | ✓ | ✓ |

## Packages
| File | Size | SHA-256 | checksums | provenance |
|---|---|---|---|---|
| `datadeck_0.1.0_darwin_arm64.tar.gz` | 8,776,189 | `83027620…85d868` | ✓ | ✓ |
| `datadeck_0.1.0_darwin_amd64.tar.gz` | 9,356,940 | `6b6cdda1…e14abf` | ✓ | ✓ |
| `datadeck_0.1.0_linux_amd64.tar.gz` | 9,221,700 | `b55713f8…91219af` | ✓ | ✓ |
| `datadeck_0.1.0_linux_arm64.tar.gz` | 8,449,992 | `4f0a8572…69bba2` | ✓ | ✓ |
| `datadeck_0.1.0_windows_amd64.zip` | 9,401,016 | `c8e73bfc…4dd9a2` | ✓ | ✓ |

- **10/10 checksums** match `checksums/SHA256SUMS` **and** `provenance.json`.
- No missing/duplicate/zero-byte/stale/unexpected artifacts; 5 binaries + 5 packages.
- Full values: `release/rc/v0.1.0/checksums/SHA256SUMS`.

## Archive integrity & extraction safety
- `tar -tzf` / `unzip -t` all OK; single entry each; **no absolute paths, no `../`
  traversal, no symlinks/devices**; executable bit preserved (`-rwxr-xr-x`).
- **Archive→binary identity**: extracted hashes == standalone binary hashes for all 5.
