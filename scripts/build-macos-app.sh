#!/usr/bin/env bash
#
# Build a macOS Apple Silicon application bundle for DataDeck.
#
# Produces release/DataDeck.app (macOS .app with a native Swift launcher and the
# Go backend + embedded frontend). Pass --dmg to also produce a .dmg.
#
# The bundle is unsigned for local development; see desktop/macos/README.md for
# Gatekeeper notes and the signing/notarization path for public releases.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP_NAME="DataDeck"
BUNDLE_ID="io.datadeck.desktop"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
if [ -n "${SOURCE_DATE_EPOCH:-}" ]; then
  BUILD_DATE="$(date -u -r "$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)"
else
  BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi

MAKE_DMG=0
[ "${1:-}" = "--dmg" ] && MAKE_DMG=1

ARCH="arm64"
RELEASE_DIR="$ROOT/release"
APP="$RELEASE_DIR/$APP_NAME.app"
LAUNCHER_SRC="$ROOT/desktop/macos/Sources/DataDeckLauncher.swift"
DIST="$ROOT/backend/internal/web/dist"

command -v swiftc >/dev/null 2>&1 || { echo "ERROR: swiftc not found (install Xcode Command Line Tools)"; exit 1; }
command -v iconutil >/dev/null 2>&1 || { echo "ERROR: iconutil not found"; exit 1; }

echo "==> DataDeck macOS app: version=$VERSION commit=$COMMIT arch=$ARCH"

echo "==> Building static frontend export"
( cd "$ROOT/frontend" && npm ci && npm run build:static )
[ -f "$ROOT/frontend/out/index.html" ] || { echo "ERROR: frontend/out/index.html missing"; exit 1; }

echo "==> Refreshing embedded assets ($DIST)"
rm -rf "$DIST"; mkdir -p "$DIST"
cp -R "$ROOT/frontend/out/." "$DIST/"
# Preserve the tracked placeholder content so the Go tree stays clean between builds.
printf '%s\n' \
  "# Placeholder so //go:embed all:dist always matches." \
  "# scripts/build-embedded.sh replaces this directory with frontend/out." \
  > "$DIST/.gitkeep"
find "$DIST" -name '*.map' -type f -delete 2>/dev/null || true

echo "==> Compiling Go backend (darwin/$ARCH, CGO-free)"
mkdir -p "$RELEASE_DIR"
LDFLAGS="-s -w \
  -X github.com/datadeck/datadeck/backend/internal/version.Version=$VERSION \
  -X github.com/datadeck/datadeck/backend/internal/version.Commit=$COMMIT \
  -X github.com/datadeck/datadeck/backend/internal/version.BuildDate=$BUILD_DATE \
  -X github.com/datadeck/datadeck/backend/internal/version.Channel=release"
( cd "$ROOT/backend" && CGO_ENABLED=0 GOOS=darwin GOARCH="$ARCH" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$RELEASE_DIR/.datadeck-backend" ./cmd/server )

echo "==> Compiling Swift launcher"
swiftc -swift-version 5 -O -parse-as-library \
  -framework AppKit -framework Security \
  -o "$RELEASE_DIR/.DataDeckLauncher" "$LAUNCHER_SRC"

echo "==> Generating application icon"
ICON_SRC="$ROOT/frontend/public/icons/icon-512x512.png"
ICONSET="$RELEASE_DIR/.AppIcon.iconset"
if [ -f "$ICON_SRC" ]; then
  rm -rf "$ICONSET"; mkdir -p "$ICONSET"
  mk() { sips -z "$2" "$2" "$ICON_SRC" --out "$ICONSET/$1" >/dev/null; }
  mk icon_16x16.png 16
  mk icon_16x16@2x.png 32
  mk icon_32x32.png 32
  mk icon_32x32@2x.png 64
  mk icon_128x128.png 128
  mk icon_128x128@2x.png 256
  mk icon_256x256.png 256
  mk icon_256x256@2x.png 512
  mk icon_512x512.png 512
  mk icon_512x512@2x.png 1024
  APPICNS="$RELEASE_DIR/.AppIcon.icns"
  iconutil -c icns "$ICONSET" -o "$APPICNS"
else
  echo "WARN: no icon source at $ICON_SRC; bundle will use the default icon"
  APPICNS=""
fi

echo "==> Assembling bundle"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources/backend"
cp "$RELEASE_DIR/.DataDeckLauncher" "$APP/Contents/MacOS/$APP_NAME"
cp "$RELEASE_DIR/.datadeck-backend" "$APP/Contents/Resources/backend/datadeck"
chmod +x "$APP/Contents/MacOS/$APP_NAME" "$APP/Contents/Resources/backend/datadeck"
[ -n "$APPICNS" ] && cp "$APPICNS" "$APP/Contents/Resources/AppIcon.icns"

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>$APP_NAME</string>
	<key>CFBundleDisplayName</key><string>$APP_NAME</string>
	<key>CFBundleExecutable</key><string>$APP_NAME</string>
	<key>CFBundleIdentifier</key><string>$BUNDLE_ID</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>$VERSION</string>
	<key>CFBundleVersion</key><string>$VERSION</string>
	<key>CFBundleIconFile</key><string>AppIcon</string>
	<key>LSMinimumSystemVersion</key><string>12.0</string>
	<key>LSApplicationCategoryType</key><string>public.app-category.developer-tools</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>NSSupportsAutomaticGraphicsSwitching</key><true/>
</dict>
</plist>
PLIST

echo "==> Validating bundle"
plutil -lint "$APP/Contents/Info.plist" >/dev/null
[ -x "$APP/Contents/MacOS/$APP_NAME" ] || { echo "ERROR: launcher not executable"; exit 1; }
[ -x "$APP/Contents/Resources/backend/datadeck" ] || { echo "ERROR: backend not executable"; exit 1; }
file "$APP/Contents/MacOS/$APP_NAME" | grep -q "arm64" || { echo "ERROR: launcher not arm64"; exit 1; }
file "$APP/Contents/Resources/backend/datadeck" | grep -q "arm64" || { echo "ERROR: backend not arm64"; exit 1; }

BACKEND_VERSION="$("$APP/Contents/Resources/backend/datadeck" --version)"
echo "    backend: $BACKEND_VERSION"
echo "$BACKEND_VERSION" | grep -q "$VERSION" || { echo "ERROR: backend version mismatch"; exit 1; }

echo "==> Launcher self-test"
"$APP/Contents/MacOS/$APP_NAME" --self-test

rm -rf "$RELEASE_DIR/.datadeck-backend" "$RELEASE_DIR/.DataDeckLauncher" "$ICONSET" "$APPICNS"

echo "==> Bundle ready: $APP"

if [ "$MAKE_DMG" -eq 1 ]; then
  echo "==> Packaging .dmg"
  STAGE="$(mktemp -d)"
  cp -R "$APP" "$STAGE/"
  ln -s /Applications "$STAGE/Applications"
  DMG="$RELEASE_DIR/${APP_NAME}-${VERSION}.dmg"
  rm -f "$DMG"
  hdiutil create -volname "$APP_NAME" -srcfolder "$STAGE" -ov -format UDZO "$DMG" >/dev/null
  rm -rf "$STAGE"
  echo "==> DMG ready: $DMG"
fi

echo "==> Done."
