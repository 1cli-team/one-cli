#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ "$(uname -s)" != Darwin ]]; then
  echo "macOS and Xcode are required / 需要 macOS 和 Xcode" >&2; exit 1
fi
case "${1:-release}" in
  debug) configuration=Debug ;;
  release) configuration=Release ;;
  *) echo "Use debug or release / 请使用 debug 或 release" >&2; exit 1 ;;
esac
xcrun xcodebuild -quiet -project App.xcodeproj -scheme one-template-ios \
  -configuration "$configuration" -destination 'generic/platform=iOS Simulator' \
  -derivedDataPath .build/simulator CODE_SIGNING_ALLOWED=NO \
  SWIFT_TREAT_WARNINGS_AS_ERRORS="${SWIFT_TREAT_WARNINGS_AS_ERRORS:-NO}" build
mkdir -p build
app=build/one-template-ios.app
rm -rf "$app"
ditto ".build/simulator/Build/Products/$configuration-iphonesimulator/one-template-ios.app" "$app"
plutil -lint "$app/Info.plist"
echo "$app"
