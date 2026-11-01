#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Never force SKIP_INSTALL on dependencies: that creates a generic archive.
# 不对依赖强制设置 SKIP_INSTALL，避免生成不可分发的通用归档。
archive=build/one-template-ios.xcarchive
if [[ -e "$archive" ]]; then
  echo "Archive exists; move it before retrying / 归档已存在，请移走后重试" >&2; exit 1
fi
signing=(CODE_SIGNING_ALLOWED=NO)
if [[ "${1:-}" == --signed ]]; then
  : "${IOS_TEAM_ID:?Set IOS_TEAM_ID / 请设置开发团队 ID}"
  signing=(CODE_SIGN_STYLE=Automatic DEVELOPMENT_TEAM="$IOS_TEAM_ID")
elif [[ $# != 0 ]]; then
  echo "Use no arguments or --signed / 请不传参数或使用 --signed" >&2; exit 1
fi
xcrun xcodebuild -quiet -project App.xcodeproj -scheme one-template-ios \
  -configuration Release -destination 'generic/platform=iOS' \
  -derivedDataPath .build/device -archivePath "$archive" "${signing[@]}" archive
/usr/libexec/PlistBuddy -c 'Print :ApplicationProperties:ApplicationPath' "$archive/Info.plist"
echo "$archive"
