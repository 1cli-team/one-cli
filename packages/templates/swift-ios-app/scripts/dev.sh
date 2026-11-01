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
# Coordinate the same app/device across checkouts with an inherited file lock.
# 通过继承的文件锁，跨工作区保护同一设备中的同一应用。
umask 077
lock_root="${TMPDIR:-/tmp}/one-ios-dev-$(id -u)"
mkdir -p "$lock_root"
lock_key=$(printf '%s\n%s\n' "$IOS_SIMULATOR_UDID" "$bundle_id" | shasum -a 256 | cut -d ' ' -f 1)
lock_file="$lock_root/$lock_key.lock"
exec 9>"$lock_file"
if ! /usr/bin/lockf -s -t 0 9; then
  echo "This app already has a dev session on this simulator; stop it first / 此应用在该模拟器中已有开发会话，请先停止它" >&2
  exit 1
fi
# Keep the file: the OS releases its lock when the last descriptor closes.
# 保留文件，由操作系统在最后一个文件描述符关闭时释放锁。
xcrun simctl bootstatus "$IOS_SIMULATOR_UDID"
xcrun simctl install "$IOS_SIMULATOR_UDID" "$app"
# Forward signals to this launch; never terminate a newer instance by Bundle ID.
# 向本次启动转发信号，不按 Bundle ID 终止可能已更新的实例。
exec xcrun simctl launch --console-pty --terminate-running-process "$IOS_SIMULATOR_UDID" "$bundle_id" "$@"
