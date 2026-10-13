# DataDeck v0.1.0 — R06 Artifact Security Report

## Secret scan (artifacts)
`strings -a` scan of all five binaries + SBOM/JSON for: the `ENCRYPTION_KEY`
placeholder, PEM private keys, AWS keys, inline passwords, and DSNs with userinfo.

| Pattern | Matches | Triage |
|---|---|---|
| `.env.example` placeholder key `0123…cdef` | 0 | — |
| `BEGIN … PRIVATE KEY` | 0 | — |
| `AKIA…` | 0 | — |
| `postgres://` | 1 | **false positive**: driver option-name table (`sslrootcert`/`sslpassword`/`verify-full`), not a credential DSN |
| `password=…` | 3 | **false positive**: code/JSON fragments (`password=""`, regex) from drivers |

**No credential or key is embedded.** `ENCRYPTION_KEY` is supplied at runtime only.
A real credential leak would be a BLOCKER; none was found.

## Archive integrity
- All archives verified intact (`tar -tzf` / `unzip -t` OK).
- Contents: binary only; **no path traversal, no absolute paths, no symlinks, no
  `.env`/`.db`/metadata**, no node_modules, no git metadata.
- SHA-256 for every artifact generated and re-verified (10/10 OK).

## Dependency posture (carried)
No dependency changed in R04/R05/R06. govulncheck = 0 reachable; npm advisories are
build-time/optional (MEDIUM, see `r05/dependency-reassessment.md`). SBOMs list them.
