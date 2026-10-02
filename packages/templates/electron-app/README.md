# OneTemplateElectron

Electron · React · TypeScript · shadcn/ui (Base UI) · Tailwind CSS · Axios · SWR · Zustand · Awilix.

## 中文

- `apps/one-template-electron-renderer`：React UI；`services/one-template-electron-main`：主进程；`packages/one-template-electron-preload`：隔离桥接。
- 生成后使用用户项目名，例如 `desktop-renderer`、`desktop-main`、`desktop-preload`。
- `one run <项目名>:dev` 开发；`one run <项目名>:build` 构建。主进程的 `pack` / `release` 生成本地制品，不自动发布。
- Awilix 只在 composition root 使用，业务通过函数工厂接收显式依赖。
- preload 只暴露 `getAppInfo()`；主进程校验请求所属窗口、主 frame、URL 和参数。
- 生产界面通过 `loadFile` 加载，Vite 使用相对资源路径，路由使用 HashRouter。
- SWR 管理远程数据与应用信息，Zustand 保存 UI 主题。初始项目没有文件操作、自动更新或业务功能。
- Ubuntu 开发启动时检查沙箱权限并自动识别 X11/Wayland 桌面；按提示安装 AppArmor profile，保留 Electron 沙箱。可运行主进程 `sandbox:profile` 生成配置。

## English

- `apps/one-template-electron-renderer`: React UI; `services/one-template-electron-main`: main process; `packages/one-template-electron-preload`: isolated bridge.
- Generated member names use the project name, e.g. `desktop-renderer`, `desktop-main`, `desktop-preload`.
- Develop with `one run <project>:dev`; build with `one run <project>:build`. Main's `pack` / `release` produce local artifacts without publishing.
- Awilix is accessed only in the composition root. Function factories receive explicit dependencies.
- Preload exposes only `getAppInfo()`. Main validates the window, main frame, URL, and arguments.
- Production loads the renderer with `loadFile`, relative Vite asset URLs, and HashRouter.
- SWR owns remote data and app information; Zustand stores the UI theme. No file operations, automatic updates, or business features are included.
- On Ubuntu, development checks sandbox permissions and detects an X11/Wayland desktop. Follow the AppArmor profile instructions while keeping Electron's sandbox enabled. Main's `sandbox:profile` generates the profile.

## Ubuntu 首次启动与桌面识别 / Ubuntu first launch and desktop detection

`dev` 在启动前读取 `/etc/os-release`，仅在 `ID=ubuntu` 时检查 Ubuntu 沙箱权限。
如 AppArmor 阻止当前 Electron 使用用户命名空间，会自动生成专用规则文件，
并显示两条 `sudo` 安装与加载命令。请先审阅文件，再按提示执行并重新运行开发任务。
脚本不会自动提权或修改系统策略。权限已配置时直接继续；升级 Electron 或移动
工作区导致可执行文件路径变化时，重新检查并引导配置。已正确安装 SUID 沙箱的环境
也可直接通过检查。

Before launch, `dev` reads `/etc/os-release` and checks Ubuntu sandbox permissions only
when `ID=ubuntu`. If AppArmor blocks the installed Electron binary's user namespaces,
it generates a dedicated profile file and prints two `sudo` commands to install and
load it. Review the file, follow the instructions, and rerun the development task.
The script does not elevate privileges or change system policy. Configured permissions
pass automatically; a changed binary path after an Electron upgrade or workspace move
triggers another check and setup guidance. A correctly installed SUID sandbox also passes.

仍可手动输出当前 Electron 的规则 / You can also print the current Electron profile manually:

```sh
one run OneTemplateElectron-main:sandbox:profile
```

Linux 开发启动保留已有 `DISPLAY`（包括 SSH X11 转发和 Xvfb）。仅配置
`WAYLAND_DISPLAY` 时，会补齐当前用户的运行目录和 Wayland 会话类型；显示变量缺失时，
先查询当前用户桌面会话，再检查活跃的 Wayland / X11 socket。自动识别 X11 时还需要
当前用户的认证文件。发现多个候选且会话无法确定时，提示显式设置显示变量。
通过 SSH 使用本机桌面时，窗口显示在该机器的桌面上。无桌面环境会给出明确提示，
请使用图形桌面终端、SSH X11 转发，或在 CI 中使用 Xvfb。

Linux development preserves an existing `DISPLAY`, including SSH X11 forwarding and
Xvfb. With only `WAYLAND_DISPLAY` set, it fills the current user's runtime directory
and Wayland session type. When display variables are missing, it queries the current
user's desktop session, then looks for live Wayland / X11 sockets. X11 discovery also
requires the current user's authentication file. Ambiguous displays require an explicit
display variable. When using a local desktop from SSH, windows appear on that machine's
desktop. A headless environment receives guidance to use a graphical terminal, SSH X11
forwarding, or Xvfb in CI.

macOS 和 Windows 跳过 Ubuntu 权限与 Linux 桌面检查。模板保留 Chromium 沙箱。
启动提示沿用 One 的语言偏好；`auto` 按 `LC_ALL`、`LC_MESSAGES`、`LANG` 判断。

macOS and Windows skip Ubuntu permission and Linux desktop checks. Chromium sandboxing
remains enabled. Startup guidance follows One's language preference; `auto` uses
`LC_ALL`, `LC_MESSAGES`, then `LANG`.

参考 / References:

- [Ubuntu user namespace restrictions](https://discourse.ubuntu.com/t/ubuntu-24-04-lts-noble-numbat-release-notes/39890)
- [Electron process sandboxing](https://www.electronjs.org/docs/latest/tutorial/sandbox)
