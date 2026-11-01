#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
: "${IOS_TEAM_ID:?Set IOS_TEAM_ID / 请设置开发团队 ID}"
: "${IOS_EXPORT_OPTIONS:?Set IOS_EXPORT_OPTIONS to an export plist / 请指定导出配置 plist}"
# Inspect before any build or signing. This task exports locally, never uploads.
# 构建签名前校验配置；此任务只在本机导出，不上传。
plutil -lint "$IOS_EXPORT_OPTIONS"
destination=$(/usr/libexec/PlistBuddy -c 'Print :destination' "$IOS_EXPORT_OPTIONS" 2>/dev/null || true)
method=$(/usr/libexec/PlistBuddy -c 'Print :method' "$IOS_EXPORT_OPTIONS")
team=$(/usr/libexec/PlistBuddy -c 'Print :teamID' "$IOS_EXPORT_OPTIONS")
if [[ -n "$destination" && "$destination" != export ]]; then
  echo "Only local export is allowed / 仅允许本机导出" >&2; exit 1
fi
if [[ "$team" != "$IOS_TEAM_ID" ]]; then
  echo "Export team must match IOS_TEAM_ID / 导出团队须与 IOS_TEAM_ID 一致" >&2; exit 1
fi
case "$method" in
  app-store-connect|release-testing|debugging) ;;
  *) echo "Unsupported iOS export method / 不支持的 iOS 导出方式" >&2; exit 1 ;;
esac
if [[ -e build/export ]]; then
  echo "Export exists; move it before retrying / 导出目录已存在，请移走后重试" >&2; exit 1
fi
bash scripts/pack.sh --signed
xcrun xcodebuild -exportArchive -archivePath build/one-template-ios.xcarchive \
  -exportPath build/export -exportOptionsPlist "$IOS_EXPORT_OPTIONS"
