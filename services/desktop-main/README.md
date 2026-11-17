# One Desktop main process

See [the desktop guide](../../apps/desktop-renderer/README.md) for architecture, development, packaging and platform acceptance.

主进程通过 Awilix 函数工厂管理窗口、后端和 IPC 生命周期；业务保留在 Go。开发、构建和发布编排统一位于根 `scripts/desktop/`，依赖根 `mise.toml` 和 workspace lockfile。

macOS 托盘使用透明单色的 `assets/trayTemplate.png`（16×16，72 dpi）和 `assets/trayTemplate@2x.png`（32×32，144 dpi），由根 `assets/icon-mono.svg` 渲染而来。保留文件名与两种分辨率，让 Electron 自动加载 Retina 图像，并由 macOS 适配菜单栏颜色。Windows 托盘使用 `assets/icon.ico`；Linux 使用 `assets/icon.png`。这些资源随 `assets/**/*` 原样打包，应用窗口及 Dock 图标独立配置。

The macOS tray uses transparent monochrome `trayTemplate.png` and `trayTemplate@2x.png`, rendered from the repository's `assets/icon-mono.svg` at 16×16 / 72 dpi and 32×32 / 144 dpi. Keep their filenames and both resolutions so Electron loads the Retina representation and macOS applies menu bar colors. Windows uses `icon.ico`; Linux uses `icon.png`. The existing `assets/**/*` packaging rule preserves these files. Window and Dock icons remain independently configured.

应用菜单由 Electron 自动生成默认菜单。托盘菜单和退出确认使用 One 的语言设置，默认菜单的退出操作通过现有 `before-quit` 流程停止后端。

Electron automatically supplies the default application menu. One localizes the tray menu and quit confirmation, and default menu quit actions pass through the existing `before-quit` backend shutdown flow.
