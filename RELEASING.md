# One CLI 与 One Desktop 发布 / Releasing One CLI and One Desktop

CLI 沿用本地 `mise run release` 流程；桌面版通过 GitHub Actions 构建和发布，配置见下方「One Desktop」。

CLI releases use the local `mise run release` workflow. Desktop installers are built and published by GitHub Actions; see “One Desktop” below.

## 中文

发布对象是包含模板和 One 技能的 CLI；桌面界面与 Go HTTP 后端单独打包。GitHub Release 位于
`1cli-team/one-cli`，文档网站的部署独立维护。统一入口是根目录的
`mise run release`；流程从已删除的 `.github/workflows/cli.yml` 迁移而来。

### 工具与凭据

- 在 Linux 或 macOS 的 amd64/arm64 主机上运行，需要 Git、GitHub CLI
  (`gh`)、mise、bash、tar、unzip。
- Go、Node、pnpm、Task、Process Compose 使用根 `mise.toml` 的固定版本。
  发布任务单独加载固定的 GoReleaser **2.18.0**，配置为 `.goreleaser.yaml`。
- 使用现有 `gh auth login` 会话，或用户已配置的 Shared credentials。
  共享凭据通过 `one exec --global --env <环境> --path <目录> --keys GH_TOKEN -- ...`
  注入；`GH_TOKEN`/`GITHUB_TOKEN` 需要仓库内容写权限。凭据不写入文件、发布说明或日志。

### 常用命令

从仓库根目录的 `master` 分支执行：

```bash
mise run release -- plan patch
mise run release:check
mise run release -- build patch --notes /absolute/path/release-notes.md
# 上一命令输出已验证的 release.json 路径；下面填入该路径。
mise run release -- publish --from /absolute/path/release.json
```

也可以一次完成构建和发布：

```bash
mise run release -- publish patch --notes /absolute/path/release-notes.md
```

`plan` 只读取远端发布状态并同步本地 Git 元数据，不创建发布 tag 或 Release。
`build` 在 `.cache/releases/<tag>-<随机后缀>/source` 创建独立源码副本，完成
全部检查与打包，只在该副本创建本地 tag。`publish --from` 复用同一组已验证
产物，重新验证后推送 tag、上传草稿并公开。源副本、发布说明、SHA256 和
`release.json` 保留在 `.cache/releases/`，失败时可检查和重试。

### 版本与源码规则

1. 发布前，本地 `master` 的 HEAD 必须与远端 `origin/master` 一致。
   构建只使用指定提交的干净副本；操作工作区尚未提交的改动不会进入发行版。
   准备发布的产品改动必须先合入 master。
2. 版本以远端最高稳定 tag `vX.Y.Z` 为准，默认递增 `patch`；也支持 `minor`
   和 `major`，递增较高位时归零较低位。预发布 tag 不参与稳定版本计算。
   无稳定 tag 时从 `v0.0.0` 计算。
3. 版本来源是 tag，不从 `package.json`、源码默认值或历史 CHANGELOG 标题推断。
   GoReleaser 注入 `main.version`、`updatecheck.buildChannel=release`，以及精确
   源码 SHA 的 `skills.bundledSourceRef`。保留现有 Go module path。
4. 打包前根据上次正式发布以来的实际提交编写中英文发布说明，使用 `--notes`
   传入。历史 `[Unreleased]` 可能包含早已发布的内容，不直接全量复制。
5. 开始构建及推送前均检查远端 master、基础 tag 对象和目标 Release。
   master 前进、tag 移动或出现冲突发布时停止并重新规划。同一时间只运行一次发布。
   `--from` 是本机生成的可信构建记录，请勿使用外部提供的记录文件。
   本地任务使用 `.cache/releases/release.lock` 防止同时运行；进程被强制终止后，
   确认锁文件记录的 PID 已退出再删除残留锁。

### 检查与产物

任务使用 `GOWORK=off go mod download` 准备四个 Go module；禁用 mise 任务产物
缓存，并强制运行 `mise run --jobs 1 --force check`。检查覆盖 React、Rust、Go，并同步
registry、模板和技能。原 `pre-push` 与测试流水线已经删除，当前检查以静态检查、
构建验证和发布包冒烟检查为准。检查结束后受版本控制的源码必须保持干净。

通过 GoReleaser 的 `release --skip=publish --clean` 生成正式版产物，
`CGO_ENABLED=0`、`-trimpath`、`-s -w` 保持原规则。附件必须恰好为以下六个文件：

| 文件 | 平台 |
| --- | --- |
| `one-cli_darwin_amd64.tar.gz` | macOS Intel |
| `one-cli_darwin_arm64.tar.gz` | macOS Apple Silicon |
| `one-cli_linux_amd64.tar.gz` | Linux amd64 |
| `one-cli_linux_arm64.tar.gz` | Linux arm64 |
| `one-cli_windows_amd64.zip` | Windows amd64 |
| `checksums.txt` | 五个归档的 SHA256 |

文件名是安装脚本及升级器的兼容协议，保持原样。Unix 归档内的程序名为 `one`，
Windows 为 `one.exe`；每个归档包含 README 和 `third_party/mise/LICENSE`。
自动验证全部归档的校验值及 Go 构建信息，并运行本机平台的 `--version`、
中英文帮助、语言切换和模板列表。本机程序通过真实升级 worker 确认正式发布通道，
并用模拟 npx 记录工作区创建时生成的技能安装命令，确认技能来源提交。冒烟检查
在仓库外的临时目录执行，使用临时 XDG 目录，保护开发者设置。

### 发布及失败恢复

- 先通过全部本地验证，再推送指向固定源码 SHA 的 annotated tag，始终禁止 force push。
- 创建或复用 GitHub Release 草稿，上传本地已验证的六个附件。附件名称必须完全
  对应；重新下载草稿附件，逐一比较 SHA256。所有检查通过后再公开并设为 Latest。
- 最高稳定 tag 在 master 历史上，且 Release 缺失或仍为草稿：继续该 tag 对应的
  **原提交**，不会因为重试而递增版本；master 后续前进也不改变续发源码。
- 当前提交已完整发布：返回已有结果。已公开版本附件不完整或包含意外附件、
  稳定 tag 被标成预发布、无 Release 的最高 tag 不在 master 历史上，均停止检查。
- 上传失败保留草稿及本地构建；优先用相同 `release.json` 重试。已存在草稿中的
  同名附件可以替换，已公开版本不替换附件。禁止删除或移动已公开的 tag。
- 公开后，在临时安装目录通过安装脚本验证 Latest 下载和版本；复制旧发行版到
  临时目录验证 `one upgrade`。不要覆盖开发者现有 `one` 启动器。

## English

One CLI releases contain templates and One skills. The desktop UI and Go HTTP backend are packaged separately. Releases are
published to `1cli-team/one-cli`; the documentation website is deployed separately.
The root `mise run release` task replaces the deleted release workflow.

### Requirements and commands

Use Linux or macOS on amd64/arm64 with Git, `gh`, mise, bash, tar, and unzip.
The source checkout uses the tool versions pinned in `mise.toml`; release tooling
loads GoReleaser **2.18.0** with `.goreleaser.yaml`. Use an existing `gh auth login`
session or user-configured Shared credentials injected by
`one exec --global --env <environment> --path <folder> --keys GH_TOKEN -- ...`.
Tokens need repository contents write access and must never be printed or saved.

```bash
mise run release -- plan patch
mise run release:check
mise run release -- build patch --notes /absolute/path/release-notes.md
mise run release -- publish --from /absolute/path/release.json
# Alternatively, build and publish together:
mise run release -- publish patch --notes /absolute/path/release-notes.md
```

Run from the repository root on `master`. `plan` reads remote release state and
refreshes local Git metadata, without creating a release tag or Release. `build`
creates an independent checkout of the selected commit and a local annotated tag,
then validates and packages it. The verified manifest, source, notes, and hashes
remain under `.cache/releases/<tag>-<random suffix>/`. `publish --from` uploads the
same verified files after rechecking their hashes and remote state.

### Version and source rules

- Local master HEAD must equal remote master when planning. Only committed source
  from a clean independent checkout is built; operator workspace edits are excluded.
  Merge product changes to master before releasing.
- Derive versions from the highest remote stable `vX.Y.Z` tag. Default to `patch`;
  `minor` and `major` reset lower components. Ignore prerelease tags. With no stable
  tags, start from `v0.0.0`.
- Inject the tag version into `main.version`, set `updatecheck.buildChannel=release`,
  and pin `skills.bundledSourceRef` to the exact source SHA. Preserve the existing
  Go module path. Package versions, source defaults, and historical changelog headings
  do not determine the release version.
- Supply reviewed Chinese and English notes based on commits since the previous
  release. Do not copy all historical Unreleased entries without checking them.
- Recheck master, the base tag object, and release state before pushing. Stop on
  concurrent state changes. Run only one release at a time. Only use trusted,
  locally generated build manifests with `--from`. A local lock at
  `.cache/releases/release.lock` prevents concurrent runs. After a forced stop,
  confirm the recorded PID has exited before deleting a stale lock.

### Verification and assets

Download all four Go modules with `GOWORK=off`, disable mise task artifact caching,
and force `mise run --jobs 1 --force check` to check React, Rust, Go, and regenerate
registry, templates, and skills. Tracked source must remain clean. The deleted
pre-push/test pipeline is replaced by current static checks, builds, and artifact
smoke checks.

GoReleaser runs `release --skip=publish --clean` with `CGO_ENABLED=0`, `-trimpath`,
and `-s -w`. Exactly six files are published: the five platform archives listed
above and `checksums.txt` containing their SHA256 hashes. Archive names are the
installer/updater compatibility contract. Every archive contains `one` (`one.exe`
on Windows), README, and the mise license. Check all hashes and Go build settings;
run the host binary's version, both help languages, locale switching, and template
list. Check the release channel through the real upgrade worker and record the
skill-install command with a simulated npx to verify its source commit. Smoke
checks use a temporary directory outside the workspace and temporary XDG directories.

### Publishing and recovery

Push the annotated tag only after validation, never with force. Create or reuse
a draft, upload the six validated assets, download them again, and compare every
SHA256 with the build manifest. Publish and mark Latest only after these checks.

A highest stable tag on master with a missing/draft Release resumes its original
commit without bumping again, even if master has advanced. A complete published
release for the current commit is idempotent. Stop if published assets are
incomplete/unexpected, a stable tag is marked prerelease, or an orphan highest tag
is off master. Keep failed drafts and build records, retry with the same manifest,
and replace duplicate assets only in drafts. Never move/delete published tags or
replace published assets. After publication, verify latest installation and
upgrade from an older release in temporary directories, preserving local launchers.

## One Desktop

### 中文：首次配置

工作流：`.github/workflows/desktop.yml`（**Release One Desktop**）。构建脚本复用
`scripts/desktop.mts`，根目录日常构建仍为 `one build-desktop`。

在 [仓库 Actions Secrets](https://github.com/1cli-team/one-cli/settings/secrets/actions)
配置以下五项。也可以先保存到 One 共享凭据，再通过 `one exec --global --env <环境>
--path <目录> --keys <这五个变量名> -- <同步命令>` 注入，使用 `gh secret set` 的标准输入同步；
不要把凭据作为命令行参数、源码、发布说明或日志输出。

| Secret | 内容 |
| --- | --- |
| `APPLE_TEAM_ID` | Apple Developer Team ID |
| `APPLE_ID` | Apple Developer 账户邮箱 |
| `APPLE_PASSWORD` | Apple 应用专用密码，不是账户登录密码 |
| `APPLE_CERTIFICATE_BASE64` | **Developer ID Application** 的 `.p12` 文件 base64，必须包含对应私钥 |
| `APPLE_CERTIFICATE_PASSWORD` | 导出该 `.p12` 时设置的密码 |

签名身份从证书自动解析，并校验 Team ID，无需第六项 Secret。Apple 签名/公证的要求见
[Tauri macOS 签名文档](https://v2.tauri.app/distribute/sign/macos/)。证书在 macOS runner
临时目录以受限权限解码，导入临时钥匙串后立即删除 `.p12`；构建前向 Apple 验证公证凭据。
任务结束或失败时恢复原钥匙串
列表并删除临时钥匙串。Secret 只注入 macOS 打包步骤。

GitHub 自带 `GITHUB_TOKEN` 用于上传，不需要另配个人 Token。构建 job 只有 `contents: read`；
只有最终发布 job 获得 `contents: write`。仓库规则需允许该工作流创建 `desktop-v*` tag 和 Release。

### 中文：触发与产物

1. 将桌面代码和工作流合入 `master`，按上文既有 CLI 流程发布新的稳定 `vX.Y.Z`。
   桌面流水线订阅 CLI Release 的 `published` 事件，使用该 tag 的固定源码 SHA。
   尚未包含桌面流水线的旧 tag 会被拒绝，不能拿旧版本号构建新源码。
2. 手动补跑：GitHub Actions → **Release One Desktop** → **Run workflow**，分支选
   `master`，`cli_tag` 填已正式发布的 CLI tag。也可执行：

   ```sh
   gh workflow run desktop.yml --repo 1cli-team/one-cli --ref master -f cli_tag=vX.Y.Z
   ```

3. 五个平台从同一提交构建。macOS arm64 job 先执行 `mise run --jobs 1 --force check`；
   所有平台都编译 React、Rust、Go。工具版本来自 `mise.toml`，Rust 固定为 1.94.0。
4. macOS 先对 `one-api`、`one` 执行 Developer ID 签名，再计算 manifest 的 SHA256。
   Tauri 通过 `bundle.macOS.files` 复制已签名 sidecar，避免二次签名改变摘要；随后签名、
   公证并 staple `.app`。脚本核对包内 sidecar 摘要、Developer ID、Team ID、强化运行时、
   Gatekeeper 与 `.app` 票据，再单独公证并 staple 最终 DMG。
5. 收齐五个平台的安装包后才创建/复用 `desktop-vX.Y.Z` 草稿。上传后重新下载全部附件并
   比较 SHA256，通过后公开。桌面 Release 使用 `--latest=false`，CLI Latest 与六附件协议保持兼容。

| 安装包文件名 | 架构 | 签名/公证 |
| --- | --- | --- |
| `one-desktop_X.Y.Z_darwin_arm64.dmg` | macOS Apple Silicon | Developer ID + 公证 + staple |
| `one-desktop_X.Y.Z_darwin_amd64.dmg` | macOS Intel | Developer ID + 公证 + staple |
| `one-desktop_X.Y.Z_windows_amd64.exe` | Windows x64，NSIS | 未签名 |
| `one-desktop_X.Y.Z_linux_amd64.AppImage` | Linux x64 | 未签名 |
| `one-desktop_X.Y.Z_linux_arm64.AppImage` | Linux arm64 | 未签名 |

Release 另含 `desktop-release.json`（源码、版本、平台和安装包摘要）与
`desktop-checksums.txt`（五个安装包及发布记录的摘要）。Actions 原始产物保留 14 天。
Linux 使用 Ubuntu 22.04 构建，以该环境的系统库为兼容基线。

失败后重跑同一 workflow/CLI tag。任何平台或公证失败均不公开；已创建的草稿保留，同名
草稿附件可在重试中替换。已公开版本只校验并返回已有结果，不移动 tag、不覆盖附件。
修复代码应发布新的 CLI 版本，再构建对应桌面版本。需要只读检查时，从根目录运行
`mise run desktop-release -- plan vX.Y.Z`；CI 的附件汇总发布入口是
`mise run desktop-release -- publish vX.Y.Z <下载产物目录>`。

Apple 的五项凭据与 Tauri updater 密钥是两套机制。本流程发布可手动安装的正式包，
不配置 updater 密钥、不发布更新 feed，也不打开尚未完成平台恢复验收的自动安装开关。
本地构建通过不代表 GitHub 五个平台执行或 Apple 公证已通过；以工作流结果为准。

### English: configuration and operation

The **Release One Desktop** workflow builds native installers for macOS arm64/x64,
Windows x64, and Linux arm64/x64. Configure the five repository Secrets listed above:
Apple Team ID, Apple ID email, an **app-specific password**, a base64-encoded
**Developer ID Application p12 including its private key**, and its export password.
The signing identity is derived and checked against the Team ID. One shared credentials
can inject these values into a synchronization command that sends each value to
`gh secret set` via stdin; never print values or put them in command arguments.

Only macOS packaging receives Apple credentials. It imports the certificate into a
temporary keychain, removes the decoded p12, validates notarization credentials with Apple before building, and restores the original keychain list
and deletes the temporary keychain on completion/failure. Build jobs have read-only
repository access; only the publisher receives contents-write access through the
automatic GitHub token. Repository rules must allow `desktop-v*` tags and Releases.

Merge the workflow and desktop source into master, then publish a stable CLI Release
through the existing CLI release entrypoint. The published event starts desktop
packaging from the exact CLI tag's commit. Tags predating the desktop workflow are
rejected. To retry, dispatch `desktop.yml` from master with `cli_tag=vX.Y.Z` using
the command above. The macOS arm64 job runs the existing full repository checks;
each platform builds React, Rust, and Go with pinned tools and Rust 1.94.0.

Sign Go sidecars before generating their manifest hashes. Tauri copies them unchanged
using `bundle.macOS.files`, then signs, notarizes and staples the app. Verify sidecar
hashes, Developer ID/Team ID, hardened runtime, Gatekeeper and the stapled app ticket.
Submit the final signed DMG for notarization too, then staple and verify it.
Windows NSIS and Linux AppImage installers are unsigned; Linux is built on Ubuntu 22.04.

After all five builds succeed, create or reuse a `desktop-vX.Y.Z` draft. Upload the
five installers plus `desktop-release.json` and `desktop-checksums.txt`, download
every attachment again, and compare hashes before publishing. Keep `--latest=false`
so CLI Latest and its six-file installer contract remain valid. Actions artifacts
are retained for 14 days. Failed drafts can be resumed for the same source; published
tags/assets are immutable. Code fixes require a new CLI version and matching desktop
release. `mise run desktop-release -- plan vX.Y.Z` performs read-only preflight;
`publish vX.Y.Z <artifact-directory>` is the CI aggregation entrypoint.

Apple credentials are independent from updater signing keys. This workflow publishes
manually installable releases; it does not enable in-app updates or publish an updater
feed. Platform recovery and Windows/Linux runtime acceptance remain separate work.
Local build results do not establish a successful CI run or Apple notarization.
