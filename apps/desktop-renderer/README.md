# One Desktop

## 中文

One Desktop 使用 Electron 主进程、隔离 preload、React 界面与独立 Go 后端。

| 模块     | 路径                       | 职责                                                      |
| -------- | -------------------------- | --------------------------------------------------------- |
| Renderer | `apps/desktop-renderer`    | 现有 React 页面、HashRouter、Axios、SWR、主题与中英文文案 |
| Main     | `services/desktop-main`    | 窗口、托盘、单实例、原生交互、后端生命周期与打包          |
| Preload  | `packages/desktop-preload` | 受限 contextBridge API 与共享类型                         |
| API      | `services/desktop-api`     | 复用 One Core 的本地 Go HTTP API                          |

React 继续通过随机 IPv4 loopback 端口直连 Go。主进程通过私有 stdin/stdout 管道完成握手、身份校验和停止；token 仅通过校验主窗口的 IPC 获取，不进入事件、URL、日志或磁盘。正式页面从 `one-desktop://app` 加载；开发页面固定为 `http://127.0.0.1:5173`。这与通用 Electron 模板的 `loadFile` 不同，以保留可校验的 HTTP Origin。沙箱、contextIsolation 与 webSecurity 始终开启。

关闭窗口隐藏界面并保留后端，托盘“打开 One Desktop”、Dock 或再次启动恢复原窗口。托盘退出与 macOS `Cmd+Q` 先停止写入并关闭后端；活动操作可取消退出。托盘和退出对话框跟随界面语言。React 页面和业务不重新设计；终端任务仍通过 `one run` 执行。

Go 的 `ONE_HOME`、工作区登记和凭据目录保持原样。正式包尽力从旧 WebKit（含 macOS 散列 origin 目录）/WebView2 的副本读取语言、主题、首次启动标记，绝不修改原数据库；新 Electron 偏好优先。无法读取旧缓存时，语言仍从共享 Go preferences 恢复，主题跟随系统。开发与测试的隔离 profile 不导入个人配置。

从仓库根运行：

```sh
one install-deps
one dev
one build-desktop
mise run check
```

`one run desktop:dev` 和 `one run desktop:build` 使用同一桌面编排入口。单独运行 renderer 的 dev/build 仅处理网页，不启动后端。开发编排按顺序构建 preload、renderer、main，再启动 Vite 和 Electron。修改 main/preload 后重启开发任务；React 支持 Vite HMR。

安装包生成到 `services/desktop-main/dist/<target>/`。Go 的 `one-api` 与 `one` 保持同目录，位于应用的 `resources/bin`，macOS 为 `Contents/Resources/bin`。CLI 安装逻辑继续保留未知安装及更高版本。Linux 同时生成 AppImage 和 DEB，macOS 生成 DMG，Windows 生成 NSIS。

正式发布仍使用根 `RELEASING.md` 的 `mise run release`，共用版本、源码 SHA 与原有附件名称。macOS 继续签名、公证应用和 DMG；Windows/Linux 不签名。应用内更新保持未启用。

原生测试：先构建桌面，再运行 `one run test:desktop -p desktop-main`。Linux CI 使用 Xvfb；受限容器可显式设置 `ONE_DESKTOP_TEST_NO_SANDBOX=1`，仅影响测试启动参数，不能作为用户启动方案。所有测试使用临时 ONE_HOME、profile 和 CLI 安装目录。macOS/Windows 的实际安装覆盖、系统托盘和签名公证仍必须在对应平台验收。

## English

The desktop group contains a React renderer, an Electron main process and a typed, isolated preload bridge. The Go API and CLI remain independent bundled binaries sharing One Core. Existing pages, routes, themes, languages and business APIs are retained.

The renderer talks directly to authenticated IPv4 loopback HTTP. A private control pipe handles bootstrap and shutdown. The packaged page uses `one-desktop://app`; development uses `http://127.0.0.1:5173`. Unlike the generic Electron template's loadFile setup, this preserves an explicit origin for the existing API. Sandboxing, context isolation and web security remain enabled.

Closing hides the window; the tray, Dock and a second launch restore it. Explicit quit stops the backend, with a localized confirmation for active writes. ONE_HOME, workspaces and credentials retain their existing locations. Released builds attempt to import only three display preferences from copies of legacy WebKit/WebView2 storage, never modify it, and preserve existing Electron preferences. Unreadable legacy caches fall back to shared Go locale and the system theme. Isolated development/test profiles never import personal preferences.

Use the root commands above. Main/preload changes require restarting development; renderer changes use Vite HMR. Installers live under `services/desktop-main/dist/<target>/`; bundled binaries remain adjacent under resources/bin (Contents/Resources/bin on macOS). The unified release, source/hash checks, artifact names and macOS signing/notarization remain in place. In-app updates stay disabled.

Desktop tests require a prior build and a graphical session or Xvfb. The explicit test-only sandbox override is for restricted containers. Real macOS/Windows installer upgrades, native integration and notarization require acceptance on their respective platforms.
