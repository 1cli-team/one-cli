# One Desktop

React 与 Tauri 位于同一个项目中，使用 `src/` + `src-tauri/` 布局。原 `apps/dashboard` 项目已重命名为 `desktop`。

```text
apps/desktop/
  src/                  React 界面、路由和 HTTP 客户端
  public/               前端静态资源
  package.json          桌面与前端开发任务
  vite.config.ts
  src-tauri/
    src/                Rust 窗口、进程生命周期和更新入口
    capabilities/       原生权限
    icons/
    Cargo.toml
    Cargo.lock
    tauri.conf.json
    binaries/           生成的 one-api 和 one，不提交
    bundle-manifest.json 生成的版本、源码和二进制摘要
```

Rust 从应用包启动 `services/desktop-api` 编译出的 `one-api`；后者复用 `packages/one-core/pkg`。React 通过带 Bearer 认证的随机 loopback HTTP 端口访问 Go。会话凭据通过私有管道及受限 Tauri IPC 交付，不写进 URL 或日志。桌面版不提供任务运行和日志界面；工作区任务在终端使用 `one run` 执行，包内 `one` 用于终端 CLI 安装。

## 开发和构建

先安装仓库固定的 mise 工具，以及 Rust stable、rustfmt 和 [Tauri 平台依赖](https://v2.tauri.app/start/prerequisites/)。当前 Cargo 最低 Rust 版本为 1.90。从仓库根运行：

```sh
one install-deps
one dev                 # 两个 Go 程序 + Vite + Tauri
one build-desktop       # 构建当前平台桌面包
one check               # Go、React、Rust、文档及发布规则检查
```

也可使用 `one run dev -p desktop` / `one run build -p desktop`。`dev:web` 和 `build:web` 仅处理前端；浏览器中打开 Vite 地址不会获得桌面会话。原 `dashboard` 项目名和 Vite API 代理已移除。

macOS 本地构建生成 `src-tauri/target/<target>/release/bundle/macos/One Desktop.app`，以及 `src-tauri/target/<target>/release/bundle/dmg/` 下的 `.dmg` 安装包。`scripts/desktop.mts` 负责先构建同版本、同源码的 `one-api` 与 `one`，再调用 Tauri。CLI 单独构建无需前端或 Rust。

首次启动可安装终端 CLI，也可跳过后在桌面设置中操作。安装检查目标版本和摘要，保留未知安装及更高版本；PATH 状态单独展示。验证安装时使用临时 `ONE_INSTALL_DIR` 和 HOME / XDG 配置，避免替换开发者的 CLI。

## 发布边界

开发包不启用生产更新。正式构建需要稳定的 `RELEASE_VERSION`、干净源码和 `ONE_DESKTOP_RELEASE=1`。macOS 额外需要根目录 `RELEASING.md` 中列出的五项 Apple 凭据；缺少凭据会失败。更新公钥和私钥可成对配置，但它们独立于 Apple 签名，本次安装包流水线不启用应用内自动更新。自动安装仍受平台恢复验收开关约束。

GitHub Actions 会在 CLI 正式 Release 发布后，从同一 tag 源码构建五个平台安装包；macOS 完成应用和 DMG 签名、公证及票据校验，Windows/Linux 不签名。所有产物上传并下载校验通过后公开 `desktop-vX.Y.Z` Release，保留 CLI Latest。支持手动补跑同一 CLI tag。配置、命令与恢复规则见根目录 `RELEASING.md`。

更新 feed、故障恢复以及 Windows/Linux 实机验收仍需完成。本地构建结果与 GitHub Actions、Apple 公证的实际执行结果分别验证。

## English

The `desktop` project combines React in `src/` with the Tauri host in `src-tauri/`. Rust owns native integration and the bundled backend lifecycle. React accesses authenticated loopback HTTP directly. The app bundles both `one-api` and `one`; shared Go packages live in `packages/one-core/pkg`, with private implementations under `internal`.

Workspace tasks run in the terminal using `one run`. The desktop app has no task runner or log viewer; the bundled `one` is the source for terminal CLI installation.

Install the pinned mise tools, Rust with rustfmt, and the Tauri platform prerequisites. Run `one dev` for development, `one build-desktop` for a native package, and `one check` for repository checks. CI pins Rust 1.94.0. First-run terminal installation is optional and preserves unknown/newer installations. Use isolated installation/configuration directories during verification.

On macOS, the build produces both `bundle/macos/One Desktop.app` and a `.dmg` installer in `bundle/dmg/`, under `src-tauri/target/<target>/release/`.

Release builds require a stable version, clean source and `ONE_DESKTOP_RELEASE=1`; macOS also requires the five Apple credentials documented in `RELEASING.md`. Updater keys are optional as a pair and separate from Apple credentials. GitHub Actions builds all five targets from the published CLI tag, signs/notarizes/staples macOS apps and DMGs, verifies all downloaded attachments, then publishes `desktop-vX.Y.Z` without changing CLI Latest. Windows/Linux installers are unsigned. Manual retries reuse the same CLI tag.

This installer workflow does not enable in-app updates. The update feed, platform recovery and Windows/Linux runtime acceptance remain separate work. Local build results do not establish a successful remote pipeline or Apple notarization.
