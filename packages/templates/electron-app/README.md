# OneTemplateElectron

Electron + React + Vite 桌面应用，由三个顶层 One 项目组成。
Electron + React + Vite desktop application composed of three top-level One projects.

```text
apps/OneTemplateElectron-renderer/     React renderer
services/OneTemplateElectron-main/     Electron main process
packages/OneTemplateElectron-preload/  Preload bridge and IPC contracts
```

三个项目共用工作区根目录的 `pnpm-workspace.yaml` 和 `pnpm-lock.yaml`。
目录与 One 项目名使用用户输入的名称加角色后缀；npm 包名遵循 kebab-case。
组合关系记录在根目录的 `one.manifest.toml`，支持在同一工作区添加多个桌面应用。

The three projects share the workspace root's `pnpm-workspace.yaml` and `pnpm-lock.yaml`.
Directory and One project names append a role suffix to the supplied name; npm package
names use kebab-case. The group is recorded in the root `one.manifest.toml`, allowing
multiple desktop applications in one workspace.

## 开发、构建与打包 / Development, builds, and packaging

从工作区根目录运行 / Run from the workspace root:

```sh
one run OneTemplateElectron:dev
one run OneTemplateElectron:build
one run OneTemplateElectron:test
one run OneTemplateElectron:pack
```

开发任务先构建共享依赖，再启动 Vite、preload watcher 和 Electron。
主进程会等待 Vite 就绪。多个桌面应用同时开发时，为各组设置不同的
`ELECTRON_RENDERER_PORT`（默认 `5173`）；同组 renderer 和 main 使用相同值。

Development builds shared dependencies before starting Vite, the preload watcher,
and Electron. The main process waits for Vite to become ready. For concurrent desktop
apps, configure different `ELECTRON_RENDERER_PORT` values (default `5173`) per group;
use the same value for a group's renderer and main.

各项目在自己的目录构建。`pack` 会先构建整组，再由 main 收集 renderer 与 preload
产物，生成当前平台的本地应用目录。`release` 生成安装包；使用前配置应用信息、
签名与发布目标。开发安装沿用根目录 registry 和安装脚本策略，保留用户的显式设置。

Each project builds in its own directory. `pack` builds the group, then main collects
renderer and preload outputs and creates a local application directory for the current
platform. `release` creates installers; configure app metadata, signing, and the publishing
target first. Installation follows the root registry and build-script policy, preserving
explicit user settings.

## UI / 界面

renderer 使用 shadcn/ui 的 `base-nova`（Base UI）。在 renderer 项目目录运行
`pnpm dlx shadcn@latest add dialog` 添加组件，保留 `components.json`。Toast 也使用 Base UI。

The renderer uses shadcn/ui's `base-nova` (Base UI). Run
`pnpm dlx shadcn@latest add dialog` in the renderer project to add components;
`components.json` remains available, and toasts also use Base UI.

Toast 示例 / Toast example:

```tsx
import { toast } from "@/components/ui/toast";

toast.add({ title: "保存成功 / Saved", type: "success" });
```

## 主进程和 IPC / Main process and IPC

main 使用 Awilix 装配 `createXxx({ dependencies })` 工厂函数，闭包保存内部状态。
`src/container.ts` 显式注册单例；`src/app.ts` 控制启动顺序。
控制器返回 `handle` / `on` 映射，IPC 常量与类型来自 `one-template-electron-preload`。
renderer 仅导入类型和通道常量，通过 `window.electron` 调用桥接 API。

Main uses Awilix to compose `createXxx({ dependencies })` factories with state in
closures. `src/container.ts` registers singletons and `src/app.ts` controls startup.
Controllers return `handle` / `on` maps; IPC constants and types come from
`one-template-electron-preload`. The renderer imports only types and channel constants,
and calls bridge APIs through `window.electron`.

主进程测试模拟 Electron API；打包资源测试使用临时目录。
Main-process tests mock Electron APIs; packaging resource tests use temporary directories.

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
