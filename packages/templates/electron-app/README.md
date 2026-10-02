# OneTemplateElectron

Electron + React + Vite，包含主进程、渲染进程和 preload 三个内部包。
Electron + React + Vite, with separate main, renderer, and preload packages.

## UI / 界面

渲染进程使用 shadcn/ui 的 `base-nova`（Base UI）组件，保留 `apps/ui/components.json`。
在 `apps/ui` 目录执行 `pnpm dlx shadcn@latest add dialog` 可继续添加组件；组合使用 `render`。
Sonner 保留现有通知与主题接口。

The renderer uses shadcn/ui's `base-nova` (Base UI) components and keeps
`apps/ui/components.json`. Run `pnpm dlx shadcn@latest add dialog` from `apps/ui`
to add components, and compose them with `render`. Sonner keeps the existing notification and theme API.

## 主进程 / Main process

主进程使用 Awilix 装配工厂函数。`apps/electron/src/container.ts` 显式注册单例服务，
其他模块通过 `createXxx({ dependencies })` 接收依赖，并用闭包保存窗口等内部状态。
启动顺序由 `src/app.ts` 控制，服务模块无需访问容器。

The main process uses Awilix to compose factory functions. `apps/electron/src/container.ts`
explicitly registers singleton services. Other modules receive dependencies through
`createXxx({ dependencies })` and keep internal state, such as windows, in closures.
`src/app.ts` controls startup order; service modules do not access the container.

控制器返回显式的 `handle` / `on` 映射，IPC 参数与返回值沿用 preload 包的类型契约。
新增控制器时在 `src/controller/index.ts` 导出工厂，并在 `src/container.ts` 中注册和聚合。

Controllers return explicit `handle` / `on` maps. IPC arguments and return values follow
the preload package's type contracts. Export new controller factories from
`src/controller/index.ts`, then register and collect them in `src/container.ts`.

在项目目录运行 `pnpm test` 验证依赖装配、IPC 和窗口行为；测试使用模拟的 Electron API。
Run `pnpm test` from this project to verify dependency composition, IPC, and window behavior
with mocked Electron APIs.

## 工作区 / Workspace

这是一个 One 项目，内部包共享仓库根目录的 `pnpm-workspace.yaml` 和
`pnpm-lock.yaml`。包名使用 `@one-template-electron/` 前缀，支持在一个仓库中添加多个桌面应用。

This is one One project. Its internal packages share the repository's root
`pnpm-workspace.yaml` and `pnpm-lock.yaml`. Package names use the
`@one-template-electron/` scope so multiple desktop apps can coexist.

```text
apps/electron/     Electron main process
apps/ui/           React renderer
packages/preload/  Preload bridge and shared IPC types
```

需要 pnpm 工作区。Node.js 和 pnpm 版本沿用仓库根目录配置。
Requires a pnpm workspace. Node.js and pnpm versions follow the root configuration.

## 开发和构建 / Development and builds

从仓库根目录运行 / Run from the repository root:

```sh
one dev -p OneTemplateElectron
one build -p OneTemplateElectron
```

开发时会按需更新根锁文件；请将其提交 Git。已有锁文件在构建时严格校验。
Development updates the root lockfile when needed; commit it to Git.
Builds validate an existing lockfile without rewriting it.

也可以在本项目目录运行 `pnpm run dev`、`pnpm run build`。
You can also run `pnpm run dev` and `pnpm run build` from this project directory.

先构建 preload，再运行主进程和 UI。多个桌面应用同时开发时，分别在各项目环境中设置
`ELECTRON_RENDERER_PORT`（默认 `5173`），主进程和 Vite 会使用相同端口。

Preload builds first, followed by the main process and UI. When developing multiple
desktop apps at once, set a different `ELECTRON_RENDERER_PORT` in each project's
environment (default `5173`). The main process and Vite use the same port.

## 依赖 / Dependencies

安装依赖使用仓库根目录配置，包括 registry、镜像和安装脚本策略。
模板不会携带独立锁文件或覆盖根目录 `.npmrc`。Electron 需要允许运行安装脚本；
已有策略中的显式禁用会保留，需要在根目录自行调整后重新安装。

Dependency installation uses the root configuration, including the registry,
mirrors, and build-script policy. The template does not carry a separate lockfile
or override the root `.npmrc`. Electron needs its installation script enabled;
existing explicit denials are preserved and must be adjusted at the root before reinstalling.

新增内部包时，在本项目 `package.json` 的 `workspaces` 和根 `pnpm-workspace.yaml`
中登记路径。

When adding an internal package, register its path in this project's `package.json`
`workspaces` and the root `pnpm-workspace.yaml`.

## Ubuntu 沙箱启动错误 / Ubuntu sandbox startup errors

如果启动时报 `The SUID sandbox helper binary was found, but is not configured correctly`，
先检查系统的 AppArmor 用户命名空间限制。构建成功并不代表系统已允许 Electron 创建沙箱。

If startup reports `The SUID sandbox helper binary was found, but is not configured correctly`,
check the system's AppArmor user-namespace restrictions. A successful build does not
mean that the operating system permits Electron to create its sandbox.

在本项目目录生成当前 Electron 的规则 / Generate a profile for the installed Electron
from this project directory:

```sh
node apps/electron/script/sandbox-profile.mjs > /tmp/one-electron.apparmor
cat /tmp/one-electron.apparmor
apparmor_parser --skip-kernel-load --skip-cache /tmp/one-electron.apparmor
```

规则只匹配当前 Electron 可执行文件的完整路径。管理员审核后，可安装到
`/etc/apparmor.d/` 并使用 `apparmor_parser -r` 加载。此操作会持续允许该可执行文件使用
用户命名空间，需管理员权限。升级 Electron 或移动工作区后，需要重新生成规则。

The profile matches only the full path of the installed Electron executable. After
review, an administrator can install it under `/etc/apparmor.d/` and load it with
`apparmor_parser -r`. This persistently grants that executable access to user namespaces
and requires administrator privileges. Regenerate the profile after upgrading Electron
or moving the workspace.

模板保持 Chromium 沙箱开启。`--no-sandbox` 只适合临时测试，会关闭所有进程的沙箱，
不要写入正式启动或发布配置。

The template keeps Chromium sandboxing enabled. `--no-sandbox` is for temporary testing
and disables sandboxing for all processes; keep it out of normal startup and release configuration.

参考 / References:

- [Ubuntu: user namespace restrictions](https://discourse.ubuntu.com/t/ubuntu-24-04-lts-noble-numbat-release-notes/39890)
- [Electron: process sandboxing](https://www.electronjs.org/docs/latest/tutorial/sandbox)
