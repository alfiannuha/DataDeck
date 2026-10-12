# Building

## Single binary (recommended)

`make build` runs `scripts/build-embedded.sh`, which installs frontend deps,
produces the static export, embeds it into the Go binary, and builds a
**CGO-free** executable at `backend/bin/datadeck`:

```bash
make build
# -> backend/bin/datadeck
```

Manual equivalent:

```bash
cd frontend
npm ci
DATADECK_STATIC_EXPORT=true NEXT_PUBLIC_API_URL= npm run build:static   # -> frontend/out

cd ../
rm -rf backend/internal/web/dist && mkdir -p backend/internal/web/dist
cp -R frontend/out/. backend/internal/web/dist/

cd backend
CGO_ENABLED=0 go build -o bin/datadeck ./cmd/server
```

Verify:

```bash
./backend/bin/datadeck --version     # datadeck 0.1.0 (commit …, built …, channel …)
```

> The embedded binary must include `frontend/out`. The repo ignores
> `backend/internal/web/dist/*` except a `.gitkeep` placeholder, so always run the
> static export before the Go build.

## Development builds

```bash
cd backend && go build ./...          # no embedded frontend (dev API only)
cd frontend && npm run build          # Next.js production build (not embedded)
```

## Cross-compilation

The backend is pure Go (`CGO_ENABLED=0`), so it cross-compiles to every supported
target:

```bash
cd backend
for t in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64; do
  os=${t%/*}; arch=${t#*/}; ext=""; [ "$os" = windows ] && ext=".exe"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -o ../release/datadeck_0.1.0_${os}_${arch}${ext} ./cmd/server
done
```

Cross-compilation is **build verification only** — it is not native runtime testing.

## Release builds

```bash
scripts/build-release.sh        # cross-compiles all targets with -trimpath and
                                # version/channel ldflags into release/
scripts/release-dry-run.sh      # local dry run: archives, checksums, SBOM (no publish)
```

- Release build injects `Version`, `Commit`, `BuildDate` (RFC3339 UTC,
  `SOURCE_DATE_EPOCH`-aware) and `Channel=release`.
- Artifact naming: `datadeck_<version>_<os>_<arch>[.exe]`.
- **Publishing is a separate, later step.** These scripts do not tag or push.

## Docker

```bash
docker build -f docker/Dockerfile --build-arg VERSION=0.1.0 -t datadeck:0.1.0 .
```

The image runs on Alpine, publishes port `8080`, and requires `ENCRYPTION_KEY` at
runtime (never baked in). Container publishing is out of scope for the binary
release.

## Version metadata

`scripts/check-version.sh` validates that `VERSION`, the backend default
(`internal/version.Version`), the frontend `package.json`/lockfile, the frontend
build-info fallback, and the Dockerfile `ARG VERSION` all agree.
