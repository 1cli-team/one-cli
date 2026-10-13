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
mise run install-deps
mise run dev                 # 两个 Go 程序 + Vite + Tauri
mise run build-desktop       # 构建当前平台桌面包
mise run check               # Go、React、Rust、文档及发布规则检查
```

也可使用 `one run dev -p desktop` / `one run build -p desktop`。`dev:web` 和 `build:web` 仅处理前端；浏览器中打开 Vite 地址不会获得桌面会话。原 `dashboard` 项目名和 Vite API 代理已移除。

macOS 本地构建生成 `src-tauri/target/<target>/release/bundle/macos/One Desktop.app`，以及 `src-tauri/target/<target>/release/bundle/dmg/` 下的 `.dmg` 安装包。`scripts/desktop.mts` 负责先构建同版本、同源码的 `one-api` 与 `one`，再调用 Tauri。CLI 单独构建无需前端或 Rust。

首次启动可安装终端 CLI，也可跳过后在桌面设置中操作。安装检查目标版本和摘要，保留未知安装及更高版本；PATH 状态单独展示。验证安装时使用临时 `ONE_INSTALL_DIR` 和 HOME / XDG 配置，避免替换开发者的 CLI。

## 发布边界

开发包不启用生产更新。正式构建需要稳定的 `RELEASE_VERSION`、干净源码、`ONE_DESKTOP_RELEASE=1`、更新公钥及签名配置。自动安装还受平台恢复验收开关约束；设置开关本身不代表恢复已验证。

当前实现包含签名更新入口和启动健康记录，正式桌面附件汇总、更新 feed 发布、平台签名/公证、故障恢复以及 Windows/Linux 实机验收仍需完成。不要把本地 `.app` 构建视为正式发布或跨平台验收；统一发布流程见根目录 `RELEASING.md`。

## English

The `desktop` project combines React in `src/` with the Tauri host in `src-tauri/`. Rust owns native integration and the bundled backend lifecycle. React accesses authenticated loopback HTTP directly. The app bundles both `one-api` and `one`; shared Go packages live in `packages/one-core/pkg`, with private implementations under `internal`.

Workspace tasks run in the terminal using `one run`. The desktop app has no task runner or log viewer; the bundled `one` is the source for terminal CLI installation.

Install the pinned mise tools, Rust stable with rustfmt, and the Tauri platform prerequisites. Run `mise run dev` for development, `mise run build-desktop` for a native package, and `mise run check` for repository checks. First-run terminal installation is optional and preserves unknown/newer installations. Use isolated installation/configuration directories during verification.

On macOS, the build produces both `bundle/macos/One Desktop.app` and a `.dmg` installer in `bundle/dmg/`, under `src-tauri/target/<target>/release/`.

Development builds do not enable production updates. Signing, publishing the desktop update feed, platform recovery and Windows/Linux runtime acceptance remain release prerequisites. A successful local macOS build does not establish those results.
