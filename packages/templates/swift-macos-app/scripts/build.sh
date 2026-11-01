#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ "$(uname -s)" != Darwin ]]; then
  echo "macOS and Xcode are required / 需要 macOS 和 Xcode" >&2
  exit 1
fi
configuration="${1:-release}"
case "$configuration" in
  debug|release) ;;
  *) echo "Use debug or release / 请使用 debug 或 release" >&2; exit 1 ;;
esac

app="build/one-template-macos.app"
xcrun swift build -c "$configuration"
binary_dir="$(xcrun swift build -c "$configuration" --show-bin-path)"
rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$binary_dir/one-template-macos" "$app/Contents/MacOS/"
cp Info.plist "$app/Contents/Info.plist"
# App resources must live inside Contents/Resources for a valid code signature.
# Bundle.appResources resolves these before falling back to SwiftPM's build tree.
for bundle in "$binary_dir"/*.bundle; do
  [[ -d "$bundle" ]] || continue
  cp -R "$bundle" "$app/Contents/Resources/"
done
plutil -lint "$app/Contents/Info.plist"
/usr/bin/codesign --force --sign - "$app"
/usr/bin/codesign --verify --strict "$app"
echo "$app"
