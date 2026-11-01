#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."

app="build/one-template-macos.app"
dmg="build/one-template-macos.dmg"
[[ -d "$app" ]] || { echo "Run task build first / 请先运行 task build" >&2; exit 1; }
stage="$(mktemp -d "${TMPDIR:-/tmp}/one-macos-dmg.XXXXXX")"
trap 'rm -rf "$stage"' EXIT
ditto "$app" "$stage/one-template-macos.app"
ln -s /Applications "$stage/Applications"
hdiutil create -ov -format UDZO -volname one-template-macos -srcfolder "$stage" "$dmg"
echo "$dmg"
