#!/usr/bin/env bash
#
# Build a free-to-distribute macOS (Apple Silicon) DataDeck application.
#
# Default: build release/DataDeck.app (native Swift launcher + Go backend with the
# embedded frontend).
# --package: also produce the release artifacts:
#   DataDeck-<version>-macos-arm64.app
#   DataDeck-<version>-macos-arm64.dmg
#   DataDeck-<version>-macos-arm64.zip
#   checksums.txt
#   README-INSTALL-MACOS.md
#
# Signing: the bundle is AD-HOC signed by default (integrity only). This is NOT
# Developer ID signing and the app is NOT notarized. Developer ID signing and
# notarization are optional and enabled only when the environment provides
# credentials; they never run otherwise, so the build always succeeds without an
# Apple Developer Program membership:
#   MACOS_SIGN_IDENTITY   "Developer ID Application: <name> (<TEAMID>)"
#   MACOS_NOTARY_PROFILE  a `notarytool store-credentials` keychain profile name
# No certificate, key, or password is ever read from the repository.
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

PACKAGE=0
case "${1:-}" in
  --package|--dmg) PACKAGE=1 ;;
esac

ARCH="arm64"
RELEASE_DIR="$ROOT/release"
ARTIFACT_BASE="${APP_NAME}-${VERSION}-macos-${ARCH}"
# The bundle keeps the clean app name (this is what users see after installing);
# only the downloadable archives carry the version/platform suffix.
APP="$RELEASE_DIR/$APP_NAME.app"
DMG="$RELEASE_DIR/${ARTIFACT_BASE}.dmg"
ZIP="$RELEASE_DIR/${ARTIFACT_BASE}.zip"
CHECKSUMS="$RELEASE_DIR/checksums.txt"
INSTALL_README_SRC="$ROOT/README-INSTALL-MACOS.md"
INSTALL_README="$RELEASE_DIR/README-INSTALL-MACOS.md"
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

echo "==> Assembling bundle ($(basename "$APP"))"
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

# --- Code signing -------------------------------------------------------------
# Default: ad-hoc signature (integrity only). It is NOT Developer ID signing and
# the app is NOT notarized; macOS Gatekeeper still warns on a downloaded copy.
SIGN_IDENTITY="${MACOS_SIGN_IDENTITY:-}"
if [ -n "$SIGN_IDENTITY" ]; then
  echo "==> Signing with Developer ID: $SIGN_IDENTITY (hardened runtime)"
  codesign --force --deep --options runtime --timestamp --sign "$SIGN_IDENTITY" "$APP"
else
  echo "==> Ad-hoc signing bundle (NOT Developer ID signed, NOT notarized)"
  codesign --force --deep --sign - "$APP" >/dev/null
fi
codesign --verify --deep --strict "$APP"

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

if [ "$PACKAGE" -eq 1 ]; then
  echo "==> Packaging .dmg"
  STAGE="$(mktemp -d)"
  cp -R "$APP" "$STAGE/"
  ln -s /Applications "$STAGE/Applications"
  rm -f "$DMG"
  hdiutil create -volname "$APP_NAME" -srcfolder "$STAGE" -ov -format UDZO "$DMG" >/dev/null
  rm -rf "$STAGE"
  echo "==> Checkpoint: .dmg ready ($(basename "$DMG"))"

  echo "==> Packaging .zip (ditto: preserves bundle structure/permissions/symlinks)"
  rm -f "$ZIP"
  ditto -c -k --sequesterRsrc --keepParent "$APP" "$ZIP"
  echo "==> Checkpoint: .zip ready ($(basename "$ZIP"))"

  if [ -n "${MACOS_NOTARY_PROFILE:-}" ]; then
    if [ -n "$SIGN_IDENTITY" ]; then
      echo "==> Notarizing .dmg (profile: $MACOS_NOTARY_PROFILE)"
      xcrun notarytool submit "$DMG" --keychain-profile "$MACOS_NOTARY_PROFILE" --wait
      xcrun stapler staple "$APP"
      # Re-seal + re-create the DMG so the stapled ticket travels with it.
      codesign --force --deep --options runtime --timestamp --sign "$SIGN_IDENTITY" "$APP"
      STAGE="$(mktemp -d)"; cp -R "$APP" "$STAGE/"; ln -s /Applications "$STAGE/Applications"
      rm -f "$DMG"; hdiutil create -volname "$APP_NAME" -srcfolder "$STAGE" -ov -format UDZO "$DMG" >/dev/null
      rm -rf "$STAGE"
    else
      echo "WARN: MACOS_NOTARY_PROFILE set without MACOS_SIGN_IDENTITY; skipping notarization"
    fi
  else
    echo "==> Not notarized (no MACOS_NOTARY_PROFILE); Gatekeeper will warn on first launch"
  fi

  echo "==> Writing checksums.txt"
  ( cd "$RELEASE_DIR" && shasum -a 256 "$(basename "$DMG")" "$(basename "$ZIP")" > "$(basename "$CHECKSUMS")" )
  cat "$CHECKSUMS"

  if [ -f "$INSTALL_README_SRC" ]; then
    cp "$INSTALL_README_SRC" "$INSTALL_README"
    echo "==> Install guide: $INSTALL_README"
  else
    echo "WARN: $INSTALL_README_SRC not found; release will not include README-INSTALL-MACOS.md"
  fi

  echo "==> Package artifacts ready in $RELEASE_DIR:"
  ls -1 "$RELEASE_DIR" | grep -E "^${APP_NAME}\.app$|^${ARTIFACT_BASE}\.(dmg|zip)$|^checksums\.txt$|^README-INSTALL-MACOS\.md$" || true
fi

echo "==> Done."
