#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ "$(uname -s)" != Darwin ]]; then
  echo "macOS and Xcode are required / 需要 macOS 和 Xcode" >&2
  exit 1
fi
case "${1:-release}" in
  debug) configuration=Debug ;;
  release) configuration=Release ;;
  *) echo "Use debug or release / 请使用 debug 或 release" >&2; exit 1 ;;
esac
identity="${2:--}"
team=""
signing=(ENABLE_HARDENED_RUNTIME=NO OTHER_CODE_SIGN_FLAGS=--timestamp=none)
if [[ "$identity" != - ]]; then
  case "$identity" in
    "Developer ID Application:"*) ;;
    *) echo "Use a Developer ID Application identity / 请使用 Developer ID Application 证书" >&2; exit 1 ;;
  esac
  team_pattern='\(([A-Z0-9]{10})\)$'
  if [[ ! "$identity" =~ $team_pattern ]]; then
    echo "Include the Team ID in the certificate name / 证书名称须包含 Team ID" >&2
    exit 1
  fi
  team="${BASH_REMATCH[1]}"
  signing=(ENABLE_HARDENED_RUNTIME=YES OTHER_CODE_SIGN_FLAGS=--timestamp)
fi

# Xcode embeds package resources and signs nested code as part of the app target.
# Xcode 通过原生应用目标管理包资源与嵌套代码签名。
xcrun xcodebuild -quiet -project App.xcodeproj -scheme one-template-macos \
  -configuration "$configuration" -derivedDataPath .build/xcode \
  -destination "platform=macOS,arch=$(uname -m)" ONLY_ACTIVE_ARCH=YES \
  CODE_SIGN_STYLE=Manual CODE_SIGN_IDENTITY="$identity" DEVELOPMENT_TEAM="$team" \
  SWIFT_TREAT_WARNINGS_AS_ERRORS="${SWIFT_TREAT_WARNINGS_AS_ERRORS:-NO}" \
  "${signing[@]}" build

app="build/one-template-macos.app"
mkdir -p build
rm -rf "$app"
ditto ".build/xcode/Build/Products/$configuration/one-template-macos.app" "$app"
plutil -lint "$app/Contents/Info.plist"
/usr/bin/codesign --verify --deep --strict "$app"
echo "$app"
