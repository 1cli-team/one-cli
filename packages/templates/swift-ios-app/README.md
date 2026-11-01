# one-template-project

Native SwiftUI iPhone/iPad starter (iOS 17+, Xcode 16+). Uses Xcode's toolchain and One's existing Swift/Task discovery. No Node, SwiftPM executable wrapper, or third-party packages.

原生 SwiftUI iPhone/iPad 模板（iOS 17+、Xcode 16+），复用 Xcode 工具链和 One 的 Swift/Task 发现机制。无需 Node、SwiftPM 可执行程序包装或第三方依赖。

## Tasks / 任务

From the workspace root / 在工作区根目录运行：

```sh
one run build -p one-template-project
one run check -p one-template-project
one run pack -p one-template-project
```

`build` creates `build/one-template-ios.app` for Simulator. `pack` creates an **unsigned device archive**, `build/one-template-ios.xcarchive`, for archive validation. Neither requires signing credentials; neither is a distributable IPA. Existing archives/exports are never overwritten; move them before repeating these tasks.

`build` 生成模拟器应用；`pack` 生成**未签名真机归档**，用于检查归档流程。两者均无需签名凭据，也不等于可分发 IPA。已有归档及导出不会被覆盖，重试前请移走。

## Simulator / 模拟器

Select and boot an iPhone or iPad in Xcode's Simulator first. Find its UDID with `xcrun simctl list devices booted`, then:

先在 Xcode Simulator 中启动 iPhone 或 iPad，通过 `xcrun simctl list devices booted` 查询 UDID，然后运行：

```sh
IOS_SIMULATOR_UDID=<udid> one run dev -p one-template-project
```

The task installs/launches this app, streams its console, forwards arguments and terminates it on task stop. It does not boot, erase or shut down simulators. Use `SIMCTL_CHILD_<NAME>` to forward environment variables to the app. `open App.xcodeproj` is also supported from this project directory.

任务安装并启动此应用，转发参数、输出控制台日志，并在任务停止时终止应用；不会启动、抹除或关闭模拟器。应用环境变量使用 `SIMCTL_CHILD_<NAME>` 转发。也可在项目目录运行 `open App.xcodeproj`。

## Distribution / 分发

Before distributing, replace the example Bundle ID in `App.xcodeproj`, configure your Apple team/App ID, add application icons, update version/build in `Info.plist`, and configure entitlements, privacy descriptions and provisioning profiles for your capabilities. Register the app in App Store Connect for TestFlight/App Store delivery.

正式分发前，替换 `App.xcodeproj` 中的示例 Bundle ID，配置 Apple 团队及 App ID、应用图标、`Info.plist` 中的版本和构建号，以及对应功能的权限、隐私说明和描述文件。TestFlight/App Store 分发还需创建 App Store Connect 应用记录。

The optional `release` task requires valid signing materials already configured in Xcode/the keychain and a matching `ExportOptions.plist` from Xcode. It creates a signed device archive, then exports locally. It does not upload, notarize, register Apple resources or submit for review. macOS Developer ID certificates cannot sign an iOS distribution.

可选的 `release` 任务需要 Xcode/钥匙串中已配置的有效签名材料，以及从 Xcode 取得的匹配导出配置。它生成签名真机归档后仅在本机导出，不上传、公证、注册 Apple 资源或提审。macOS Developer ID 证书不适用于 iOS 分发。

```sh
IOS_TEAM_ID=<team-id> IOS_EXPORT_OPTIONS=/absolute/path/ExportOptions.plist \
  one run release -p one-template-project
```

Export options must specify a matching `teamID`, a local `destination` (`export` or omitted), and `method` (`app-store-connect`, `release-testing` or `debugging`). Export/signing failures preserve artifacts for diagnosis. Keep credentials out of source control; One environment injection may be configured separately. Signed export and App Store delivery require validation with your own account.

导出配置须包含匹配的 `teamID`、本地 `destination`（`export` 或省略）和 `method`（`app-store-connect`、`release-testing` 或 `debugging`）。失败时保留产物供排查。凭据不要提交到源码；可另行配置 One 环境变量注入。签名导出及商店分发需使用自己的账号验证。
