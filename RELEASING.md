# One CLI 与 One Desktop 发布 / Releasing One CLI and One Desktop

从 **v0.1.22** 开始，CLI 与 Desktop 使用同一个稳定 tag、同一源码提交和同一个 GitHub Release。v0.1.21 及以前的 CLI Release、历史 `desktop-v*` Release 均保持原样。新流程不创建 `desktop-v*` tag，不回填历史附件。

Starting with **v0.1.22**, CLI and Desktop share one stable tag, source commit and GitHub Release. CLI releases through v0.1.21 and historical `desktop-v*` releases remain unchanged. The new publisher never creates desktop tags or backfills historical attachments.

从 **v0.1.23** 开始，Linux amd64 / arm64 同时提供 AppImage 和 `.deb`，统一 Release 增至 **15 个附件**。已公开的 v0.1.22 保留原 **13 个附件**，不补传 `.deb`。

Starting with **v0.1.23**, Linux amd64 / arm64 ship both AppImage and `.deb`, for **15 assets** per unified Release. The published v0.1.22 retains its original **13 assets**; never backfill DEBs.

## 中文

### 发布入口与完成条件

统一入口仍为仓库根目录的 `mise run release`。CLI 在独立干净源码副本中构建；Desktop 在 GitHub Actions 的 **Release One CLI and Desktop** 工作流（`.github/workflows/desktop.yml`）中构建。

1. 本地验证 CLI 的五个平台归档、校验文件、构建信息和冒烟检查。
2. 推送固定源码的稳定 tag，创建同名 **草稿**，上传并下载校验六个 CLI 附件。
3. 显式 dispatch 桌面工作流，冻结源码 SHA、tag 对象、草稿 ID 和 CLI 校验文件摘要。
4. 五个平台桌面构建全部通过，macOS 完成应用与 DMG 的签名、公证和票据校验。
5. 九个桌面附件上传到同一草稿；重新下载并验证全部 **15 个附件**。再次确认 tag、草稿、CLI 摘要及附件未变化后，统一公开并设为 Latest。

本地 `publish` 成功返回表示 **CLI 草稿已准备、流水线已触发**，不是正式发布完成。以 Actions 最终发布 job 成功、Release 已公开且为 Latest 为准。桌面构建或公证失败时，CLI 也保持草稿；原 Latest 不变。

### 工具与凭据

- 本地入口支持 Linux/macOS amd64/arm64，需要 Git、GitHub CLI (`gh`)、mise、bash、tar、unzip；Go、Node、pnpm、Task、Process Compose 来自根 `mise.toml`。
- GoReleaser 固定 **2.18.0**，配置为 `.goreleaser.yaml`。Tauri CI 使用 Rust **1.94.0**。
- 使用现有 `gh auth login` 会话，或通过 `one exec --global --env <环境> --path <目录> --keys GH_TOKEN -- ...` 注入共享凭据。令牌需要仓库内容写权限及 Actions 工作流触发权限；细粒度令牌需要 Contents write、Actions write。不要输出令牌或写入文件。
- CLI 安装脚本、升级器继续按 `one-cli_*` 和 `checksums.txt` 下载。桌面附件使用独立名称和 `desktop-checksums.txt`；两种校验文件不混用。

### 常用命令

先将产品和发布流程的改动提交并合入 `master`；本地 HEAD 必须与远端 `origin/master` 一致。

```sh
mise run release -- plan patch
mise run release:check
mise run release -- build patch --notes /absolute/path/release-notes.md
# 使用上一命令输出的 release.json，准备草稿并触发流水线：
mise run release -- publish --from /absolute/path/release.json
# 或一次构建并启动发布：
mise run release -- publish patch --notes /absolute/path/release-notes.md
```

`plan` 只读远端发布状态并同步 Git 元数据，不创建 tag 或 Release。`build` 在 `.cache/releases/<tag>-<随机后缀>/source` 中构建，只在该副本创建本地 tag。源副本、发布说明、SHA256 和 `release.json` 保留用于检查和重试。`--from` 只能读取本机生成的可信构建记录。

工作流触发失败时，草稿和已验证 CLI 附件仍保留，可用原 `release.json` 重试。手动重跑：Actions → **Release One CLI and Desktop** → Run workflow，分支 `master`，`release_tag` 填同一个稳定 tag：

```sh
gh workflow run desktop.yml --repo 1cli-team/one-cli --ref master -f release_tag=vX.Y.Z
```

只有准备好六个 CLI 附件的 v0.1.22+ 草稿才可开始桌面构建。流水线不监听 Release published 事件，也不会因最终公开 Release 而再次启动。`mise run desktop-release -- plan vX.Y.Z` 是只读预检；CI 的 `publish` 子命令必须携带 plan 输出的 `ONE_RELEASE_SOURCE`、`ONE_RELEASE_TAG_OBJECT`、`ONE_RELEASE_ID`、`ONE_RELEASE_CLI_CHECKSUMS_SHA256`，禁止绕过固定状态直接发布。

### 版本、源码与检查

- 版本按远端最高稳定 `vX.Y.Z` tag 递增，默认 patch，可选 minor/major；忽略预发布 tag。不根据 package.json、源码默认值或历史 CHANGELOG 猜测版本。
- 固定已提交的 master 源码。工作区未提交修改不会进入构建；重试始终使用原 tag 与原提交，即使 master 已前进。目标源码必须已包含统一发布协议。
- 所有检查、构建、上传前校验源码和远端状态；并发变更、tag 对象变化、意外附件或更高稳定 tag 会阻止发布。禁止 force push。
- 发布说明用 `--notes` 提供，按上次正式发布以来的实际变更审阅中英文内容；不要直接复制整个历史 Unreleased。新草稿追加 CLI / Desktop 下载与签名说明；重试保留已有说明。
- 本地 `.cache/releases/release.lock` 防止同时运行；残留锁只有确认所记录 PID 已退出后才能删除。Actions 使用统一发布 concurrency group 串行运行，失败重试不取消正在进行的构建。
- 四个 Go module 使用 `GOWORK=off go mod download` 准备依赖；禁用 mise 任务缓存并执行 `mise run --jobs 1 --force check`。覆盖 React、Rust、Go、官网、AI 指引与发布规则。生成资源后受版本控制的源码必须仍干净。
- GoReleaser 执行 `release --skip=publish --clean`，保留 `CGO_ENABLED=0`、`-trimpath`、`-s -w`，注入版本、release channel 和精确技能来源 SHA，保留 Go module path。
- 自动验证五个归档的 SHA256、Go 构建信息、源码与平台；验证本机 CLI 版本、中英文帮助、语言切换、模板列表、真实 upgrade worker 的正式通道，以及模拟 npx 记录中的技能来源提交。冒烟在仓库外临时目录和临时 XDG 配置中执行，不替换开发者的 `one`。

### 附件协议

v0.1.23 及以后的 Release 必须恰好包含以下 **15 个附件**，缺件、重复、其他版本文件和意外文件都会阻止公开。v0.1.22 按原 13 个附件识别；更早的 CLI Release 按原六个附件识别，不补传历史附件。

| 文件                                     | 用途                                       |
| ---------------------------------------- | ------------------------------------------ |
| `one-cli_darwin_amd64.tar.gz`            | CLI macOS Intel                            |
| `one-cli_darwin_arm64.tar.gz`            | CLI macOS Apple Silicon                    |
| `one-cli_linux_amd64.tar.gz`             | CLI Linux amd64                            |
| `one-cli_linux_arm64.tar.gz`             | CLI Linux arm64                            |
| `one-cli_windows_amd64.zip`              | CLI Windows amd64                          |
| `checksums.txt`                          | 五个 CLI 归档的 SHA256                     |
| `one-desktop_X.Y.Z_darwin_arm64.dmg`     | Desktop macOS Apple Silicon                |
| `one-desktop_X.Y.Z_darwin_amd64.dmg`     | Desktop macOS Intel                        |
| `one-desktop_X.Y.Z_windows_amd64.exe`    | Desktop Windows x64，NSIS                  |
| `one-desktop_X.Y.Z_linux_amd64.AppImage` | Desktop Linux x64                          |
| `one-desktop_X.Y.Z_linux_arm64.AppImage` | Desktop Linux arm64                        |
| `one-desktop_X.Y.Z_linux_amd64.deb`      | Desktop Ubuntu/Debian x64                  |
| `one-desktop_X.Y.Z_linux_arm64.deb`      | Desktop Ubuntu/Debian arm64                |
| `desktop-release.json`                   | 桌面源码、版本、平台、公证状态和安装包摘要 |
| `desktop-checksums.txt`                  | 七个桌面安装包及桌面发布记录的 SHA256      |

CLI 归档包含 `one` / `one.exe`、`one-api` / `one-api.exe`、README、`third_party/mise/LICENSE`。安装与升级使用不可变的服务程序副本，保留运行中的本地服务。旧版单文件升级器遗漏服务程序时，新 CLI 会从同版本发布包校验并补齐到用户私有目录。CLI 文件名是安装与升级协议，保持不变。Desktop 内置同版本、同源码的 `one-api` 和 `one`。

Linux 构建一次生成 AppImage 与 DEB，每个架构的构建记录使用 `artifacts` 数组记录两种格式。发布器仍支持 v0.1.22 的单个 `artifact` 记录。DEB 通过系统包管理器安装 GTK/WebKitGTK 依赖，不内置 AppImage 的整套运行库；安装时使用 `sudo apt install ./one-desktop_X.Y.Z_linux_amd64.deb`（ARM64 使用对应文件）。

### Apple 签名与公证

在 [仓库 Actions Secrets](https://github.com/1cli-team/one-cli/settings/secrets/actions) 中配置：

| Secret                       | 内容                                               |
| ---------------------------- | -------------------------------------------------- |
| `APPLE_TEAM_ID`              | Apple Developer Team ID                            |
| `APPLE_ID`                   | Apple Developer 账户邮箱                           |
| `APPLE_PASSWORD`             | Apple 应用专用密码，不是账户登录密码               |
| `APPLE_CERTIFICATE_BASE64`   | 含私钥的 Developer ID Application `.p12` 的 base64 |
| `APPLE_CERTIFICATE_PASSWORD` | `.p12` 导出密码                                    |

可先保存到 One 共享凭据，通过 `one exec --global` 精确注入这五项，并经 `gh secret set` 标准输入同步；不要把值放在命令参数、源码、发布说明或日志中。签名身份从证书解析并匹配 Team ID，无需第六项 Secret。要求见 [Tauri macOS 签名文档](https://v2.tauri.app/distribute/sign/macos/)。

只有 macOS 构建步骤获得 Apple 凭据。证书以受限权限解码，导入临时钥匙串后删除 `.p12`；构建前验证公证凭据，成功或失败均恢复原钥匙串列表并清理临时钥匙串。

Go sidecar 先签名，再生成摘要；Tauri 通过 `bundle.macOS.files` 复制签名文件，避免重复签名改变摘要。应用完成签名、公证和 staple 后，验证 sidecar 摘要、Developer ID、Team ID、强化运行时、Gatekeeper 和票据；最终 DMG 也独立公证、staple 并验证。

时间戳服务与票据网络错误按 5、15、30 秒有限重试；签名无效、公证拒绝等错误立即失败。Tauri 公证成功但应用缺少票据时，按 5、15 秒最多重新打包两次，应用和 DMG 一起重新生成并校验，不能只修复外部应用。重试耗尽不发布。

Windows/Linux 不签名。Linux 以 Ubuntu 22.04 系统库为兼容基线。Actions 原始桌面产物保留 14 天。构建 job 使用 contents read；plan 为读取草稿获得 contents write，但只做读取；最终发布 job 使用 contents write 上传附件和公开。CI 不创建 tag，稳定 tag 由本地入口推送。

### 失败恢复与发布后验证

- 新 tag 尚无 Release 或仍为草稿：续发原 tag 和源码，不另增版本。
- CLI 草稿附件一旦上传即冻结，重试只补缺失文件。已存在的文件必须与原构建记录一致，不覆盖；不完整或冲突文件会停止并提示检查。优先使用原 `release.json`。
- 桌面构建或公证失败：保持同一草稿和原 Latest，重跑失败 job / 同一 workflow。最终上传失败时，可直接重跑发布 job，复用同一 run 的构建产物；若重跑完整流程，桌面草稿附件可被重新验证的同版本产物替换。
- workflow plan 冻结源码 SHA、tag 对象、Release ID 和 CLI checksums.txt 的 SHA256。发布前后检查附件 ID、名称、大小、摘要和上传状态，防止下载校验期间附件被替换或删除。
- 已公开新版本只校验并返回，不重新设置历史版本为 Latest，不覆盖附件。新入口不写 v0.1.21 及更早版本，也不写历史 `desktop-v*` 发布。公开版本缺件或异常时停止。
- 修改源码需要新版本；不能移动已有 tag 来修复构建。发布后在临时安装目录验证 Latest 安装和旧版 `one upgrade`，不要覆盖开发者的启动器。
- Apple 凭据与 Tauri updater 密钥独立。当前流程只发布手动安装包，不启用应用内自动更新、不生成 updater feed。Windows/Linux 实机运行和自动更新恢复验收另行完成。

本地检查通过不代表五个平台 Actions 或 Apple 公证成功；实际发布状态以远端执行结果为准。

## English

### Entrypoint and completion

Use `mise run release` from the repository root. The CLI is built in an isolated clean checkout. The **Release One CLI and Desktop** workflow (`.github/workflows/desktop.yml`) builds desktop installers and completes publication.

1. Validate the five CLI archives, checksums, build metadata and smoke checks locally.
2. Push the fixed stable tag, create its draft, upload and download-verify the six CLI assets.
3. Explicitly dispatch CI, freezing the source SHA, Git tag object, draft Release ID and CLI checksum manifest hash.
4. Build all five desktop targets; sign, notarize, staple and verify both macOS apps and DMGs.
5. Upload nine desktop assets to the same draft. Download and verify all **15 assets**, recheck frozen state and attachment identities, then publish once as Latest.

A successful local `publish` means **draft staged and workflow dispatched**, not publication complete. Wait for the final CI job and a public Latest Release. Desktop failures leave CLI assets in the draft and preserve the previous Latest. The workflow does not subscribe to Release published events.

### Commands and source rules

Use the commands above from master with HEAD matching origin/master. `plan` reads remote state and refreshes Git metadata; `build` uses `.cache/releases/<tag>-<random>/source`, with only a local tag in that copy. `publish --from` reuses the original trusted `release.json`. The cache retains source, notes and hashes for recovery. Uncommitted changes never enter a release.

Derive versions from the highest remote stable vX.Y.Z tag, defaulting to patch; minor/major reset lower fields. Ignore prereleases and package.json defaults. Resume interrupted tags at their original commit even after master advances. Do not force-push or move tags. Provide reviewed bilingual notes with `--notes`; existing draft notes are preserved on retry. Local locks and workflow concurrency serialize release attempts.

The next stable version after v0.1.21 uses the unified protocol. Published historical CLI releases still require exactly the original six assets; v0.1.22 requires its original 13 assets, while v0.1.23+ requires all 15 files in the table above. No new desktop-v* tags are created and historical releases are never backfilled.

Linux builds generate AppImage and DEB together, with an `artifacts` array in each architecture's build record. The publisher also accepts the legacy single `artifact` record from v0.1.22. DEBs depend on system GTK/WebKitGTK packages instead of bundling the AppImage runtime libraries. Install with `sudo apt install ./one-desktop_X.Y.Z_linux_amd64.deb`, or the matching ARM64 file.

Use Linux/macOS amd64/arm64 with Git, gh, mise, bash, tar and unzip. Pin tools through mise.toml, GoReleaser **2.18.0**, and CI Rust **1.94.0**. Local authentication needs content-write and workflow-dispatch access (Contents write and Actions write for fine-grained tokens). Reuse gh authentication or inject GH_TOKEN through One shared credentials; never print/save credential values.

Download all four Go modules with GOWORK=off and run `mise run --jobs 1 --force check` with task caching disabled. Check React, Rust, Go, website, agent guidance and release rules; tracked source must remain clean. GoReleaser uses `release --skip=publish --clean`, CGO_ENABLED=0, -trimpath, -s -w, and injects version, release channel and exact skill-source SHA. Verify archive hashes and Go source/platform metadata, host version, bilingual help, locale switching, templates, the real upgrade worker's release channel and simulated npx skill-source arguments. Smoke checks use temporary directories and XDG configuration.

### CI, signing and recovery

After staging CLI assets, the local entrypoint dispatches desktop.yml on master with `release_tag=vX.Y.Z`. The same command above supports retries. Only v0.1.22+ tags containing the unified publisher and a complete CLI draft are eligible. `mise run desktop-release -- plan vX.Y.Z` is read-only. The CI publish command requires the frozen ONE_RELEASE_SOURCE, ONE_RELEASE_TAG_OBJECT, ONE_RELEASE_ID and ONE_RELEASE_CLI_CHECKSUMS_SHA256 outputs.

Configure the five Apple Secrets in the table above: Team ID, Apple ID, app-specific password, Developer ID Application p12 including its private key (base64), and p12 export password. Derive signing identity from the certificate and validate its Team ID. Shared credentials may be synchronized using gh secret set stdin; never put values in arguments or logs.

Only macOS build steps receive Apple secrets. Use a temporary keychain, remove decoded p12 files after import, validate notarization credentials before building, and restore/clean keychains on success or failure. Sign sidecars before hashing; copy them using bundle.macOS.files, then sign/notarize/staple the app. Verify its sidecar hashes, identity, hardened runtime, Gatekeeper and ticket. Separately notarize/staple/verify the DMG. Timestamp/ticket network failures retry after 5/15/30 seconds; missing app tickets retry the entire app+DMG packaging up to twice after 5/15 seconds. Invalid signatures and rejected notarization fail immediately.

Windows NSIS and Linux AppImage/DEB installers are unsigned. Linux uses Ubuntu 22.04 as its system-library baseline. Build jobs have contents read; preflight has contents write to see drafts but performs only reads; the publisher has contents write to upload and publish. CI creates no Git tags. Actions desktop artifacts are retained for 14 days.

CLI draft bytes are immutable once uploaded: retries only upload missing files and compare existing assets with the original build. Conflicting or incomplete existing files stop for inspection. Desktop drafts can reuse validated artifacts; rerun a failed publish job to reuse its run's installers. A full retry may replace only unpublished desktop assets. Tag movement, draft recreation, newer stable tags, CLI checksum changes or attachment replacement/deletion during verification stop publication.

Completed public releases are validated and returned without writes. Code fixes require a new version. Never replace published assets, move published tags, mutate historical releases or make an older version Latest. After publication, verify installation and upgrade from an older CLI in temporary installation directories. Existing CLI archive names and checksums.txt remain compatible with installers and one upgrade; desktop-checksums.txt covers only desktop files.

Apple signing is separate from updater signing. This workflow publishes manual installers and does not enable in-app updates or produce an updater feed. Local verification does not establish real CI/notarization success; platform runtime and updater recovery acceptance remain separate.
