# One CLI 发布 / Releasing One CLI

## 中文

发布对象是包含 Dashboard、模板和 One 技能的 CLI。GitHub Release 位于
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

任务使用 `GOWORK=off go mod download` 准备两个 Go module；禁用 mise 任务产物
缓存，并强制运行 `mise run --jobs 1 --force check`。检查会重建并同步 Dashboard、
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
中英文帮助、语言切换和模板列表。语言检查使用临时 XDG 目录，保护开发者设置。

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

One CLI releases contain the Dashboard, templates, and One skills. Releases are
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

Download both Go modules with `GOWORK=off`, disable mise task artifact caching,
and force `mise run --jobs 1 --force check` to regenerate and check Dashboard,
registry, templates, and skills. Tracked source must remain clean. The deleted
pre-push/test pipeline is replaced by current static checks, builds, and artifact
smoke checks.

GoReleaser runs `release --skip=publish --clean` with `CGO_ENABLED=0`, `-trimpath`,
and `-s -w`. Exactly six files are published: the five platform archives listed
above and `checksums.txt` containing their SHA256 hashes. Archive names are the
installer/updater compatibility contract. Every archive contains `one` (`one.exe`
on Windows), README, and the mise license. Check all hashes and Go build settings;
run the host binary's version, both help languages, locale switching, and template
list with temporary XDG directories.

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
