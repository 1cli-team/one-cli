#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Require an explicit booted simulator; never boot or erase the user's devices.
# 明确指定已启动的模拟器，不自动启动或抹除设备。
: "${IOS_SIMULATOR_UDID:?Set IOS_SIMULATOR_UDID to a booted simulator / 请指定已启动模拟器的 UDID}"
if ! xcrun simctl list devices booted | grep -F "($IOS_SIMULATOR_UDID)" >/dev/null; then
  echo "Select an already booted simulator UDID / 请选择已启动模拟器的 UDID" >&2; exit 1
fi
bash scripts/build.sh debug
app=build/one-template-ios.app
bundle_id=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app/Info.plist")
xcrun simctl bootstatus "$IOS_SIMULATOR_UDID"
xcrun simctl install "$IOS_SIMULATOR_UDID" "$app"
cleanup() { xcrun simctl terminate "$IOS_SIMULATOR_UDID" "$bundle_id" >/dev/null 2>&1 || true; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
xcrun simctl launch --console-pty --terminate-running-process "$IOS_SIMULATOR_UDID" "$bundle_id" "$@" &
wait "$!"
