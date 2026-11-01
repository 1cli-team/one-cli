# one-template-macos

A native SwiftUI menu-bar utility with no Dock icon or main window. Quit from its menu.
原生 SwiftUI 菜单栏工具，不显示 Dock 图标或主窗口，可从菜单退出。

## Requirements / 环境要求

- macOS 14+, Xcode 16+ (Swift 6), and Task. No third-party Swift dependencies.
- macOS 14 及以上、Xcode 16 及以上（Swift 6）和 Task；没有第三方 Swift 依赖。
- One pins Task in mise; run `one mise install` from the workspace root before running tasks. Swift and the macOS SDK come from the selected Xcode.
- One 在 mise 中固定 Task 版本；运行任务前，先在工作区根目录执行 `one mise install`。Swift 和 macOS SDK 使用当前选中的 Xcode。
- Check `xcode-select -p` and `xcrun swift --version`. SwiftPM resolves any dependencies you add when building.
- 用上述命令检查 Xcode；后续添加的 SwiftPM 依赖会在构建时解析。

## Tasks / 任务

From the One workspace root / 在 One 工作区根目录：

```sh
one mise install
one run one-template-project:dev
one run one-template-project:check
one run one-template-project:build
one run one-template-project:pack
```

Or run `task dev|check|build|pack` in this project directory.
也可以进入本项目目录运行对应的 Task 任务。

`build` produces `build/one-template-macos.app`; `pack` adds a DMG with an Applications shortcut.
The enclosed app is ad-hoc signed; the DMG is unsigned. These are local development artifacts, not notarized downloads.
Both use the host architecture. Run the bundle (`task dev`), not the bare Swift executable, so macOS reads Info.plist.

`build` 生成应用包，`pack` 额外生成带 Applications 快捷方式的 DMG。
应用采用临时签名，DMG 未签名，两者均未经公证，仅作本地开发使用，架构与构建机器一致。
请通过 `task dev` 运行应用包，让 macOS 正确读取 Info.plist。

## Signing and notarization / 签名与公证

1. Set your own bundle identifier, version and build number in `Info.plist`.
2. Import a **Developer ID Application certificate with its private key** into your macOS keychain.
3. Store notarization credentials interactively with `xcrun notarytool store-credentials` and choose a profile name.
4. Run the release task with the identity and profile name:

1. 在 `Info.plist` 中修改 Bundle ID、版本和构建号。
2. 将 **Developer ID Application 证书及其私钥**导入 macOS 钥匙串。
3. 交互运行 `xcrun notarytool store-credentials`，保存公证凭据并命名。
4. 通过环境变量指定签名身份与凭据名称，运行发布任务：

```sh
SIGNING_IDENTITY='Developer ID Application: Your Name (TEAMID)' \
NOTARY_PROFILE='your-notary-profile' \
task release
```

The task signs with hardened runtime, notarizes and staples the app, packages it, then signs, notarizes and staples the DMG. It verifies Gatekeeper acceptance and **does not upload a GitHub release**. Any failure stops the task. Do not distribute an artifact from a failed run.

任务先启用 Hardened Runtime 签名、公证并装订应用，再打包、签名、公证并装订 DMG，最后验证 Gatekeeper。
**不会自动发布 GitHub Release**；任何步骤失败都会停止，失败的运行产物不可分发。

One's global credentials can supply the identity/profile names at runtime:

One 的全局凭据可以在运行时注入身份和凭据名称：

```sh
one exec --global --env dev --path /macos \
  --keys SIGNING_IDENTITY,NOTARY_PROFILE -- task release
```

Run this in the generated project directory. This only injects environment variables: it does **not** install signing keys or create a notarytool profile. Each signing machine (including CI) still needs the certificate/private key and notarization credentials. For CI, use an ephemeral keychain and your secret store, then remove it after the job. Never commit P12 files, private keys or passwords. Do not enable shell tracing around secrets.

在生成的项目目录执行。此命令仅注入环境变量，**不会**安装签名私钥或建立 notarytool 凭据。
每台签名机器（包括 CI）仍需要证书私钥和公证凭据；CI 应使用临时钥匙串与凭据存储，并在结束时清理。
不要提交 P12、私钥或密码，也不要在处理凭据时开启 shell tracing。

Resources contain English and Simplified Chinese strings. Add application-specific entitlements and permission descriptions only when your features need them.
资源包含英文和简体中文；按实际功能需要再添加 entitlements 和权限说明。
