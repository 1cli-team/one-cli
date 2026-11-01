#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
bash scripts/build.sh debug
# Keep the app in One's process tree; preserve its environment, logs and status.
# 应用由 One 的进程树管理，保留环境变量、日志与退出状态。
exec build/one-template-macos.app/Contents/MacOS/one-template-macos "$@"
