# One CLI

AI-native workspace tooling: create projects, run tasks, and manage variables through predictable commands and structured results.

面向 AI 的工作区工具：创建项目、执行任务、管理变量，提供稳定的命令与结构化结果。

[Website / 官网](https://www.1cli.dev/) · [Downloads / 下载](https://github.com/1cli-team/one-cli/releases/latest) · [Agent skills / AI 技能](packages/agent-skills/skills)

## Install / 安装

```sh
curl -fsSL https://1cli.dev/install.sh | bash
```

```powershell
irm https://1cli.dev/install.ps1 | iex
```

One Desktop installers and CLI archives share the same Release. Open One Desktop to manage workspaces, accounts, project variables, and shared credentials.

One Desktop 安装包与 CLI 归档位于同一 Release。工作区、账号、项目变量和共享凭据通过 One Desktop 管理。

## Sign in over SSH / SSH 远程登录

```sh
one login --no-browser
```

Open the displayed URL in your own computer's browser, sign in to Infisical, and paste the browser's CLI login token into the remote terminal's hidden prompt. SSH sessions select this mode automatically, so `one login` also works. No port forwarding is needed. Use `--site-url` for a self-hosted instance. Sign-in expires after 10 minutes and requires an interactive terminal. Credentials are stored in plaintext in `~/.one/config/session.json` (or `$ONE_HOME/config/session.json`), with file mode `0600` on Unix; no system keyring is required. Check the result with `one whoami`, and remove the local session with `one logout`. After upgrading from keyring storage, run `one login` again; existing keyring entries are neither read nor migrated.

在自己电脑的浏览器中打开终端显示的链接，登录 Infisical 后，将浏览器中的 CLI 登录令牌粘贴到远端终端的隐藏输入框。SSH 会话自动选择此模式，直接运行 `one login` 也可以，无需端口转发。自建实例可通过 `--site-url` 指定地址。登录有效期为 10 分钟，需要交互式终端。凭据以明文保存在 `~/.one/config/session.json`（或 `$ONE_HOME/config/session.json`），Unix 文件权限为 `0600`，无需系统钥匙环。完成后通过 `one whoami` 查看登录状态，使用 `one logout` 删除本地会话。从钥匙环存储版本升级后需重新运行 `one login`，旧钥匙环条目不会被读取或迁移。

## Agent entry points / AI 入口

Use the installed CLI as the source of truth. Read the workspace's `AGENTS.md` and `one.manifest.toml` before changing projects.

以实际安装的 CLI 为准；修改项目之前先读取工作区的 `AGENTS.md` 和 `one.manifest.toml`。

```sh
one help --all
one templates -o json
one run --list -o json
```

Use subcommand `--help` to discover arguments. Parse `-o json` results by schema, error code, and context. `one skills`, `one mise`, and `one hk` preserve upstream output. Skills and generated AGENTS guidance are maintained in source, without a separate tutorial site.

通过子命令的 `--help` 发现参数，按 schema、错误码和上下文解析 `-o json` 结果。`one skills`、`one mise`、`one hk` 保留上游输出。技能与生成的 AGENTS 指引由源码维护，不再维护独立教程站。

Templates and official One skills ship with the CLI. New workspaces copy embedded skills and register their GitHub source in the standard Skills lock file. Initialize or migrate older local copies with `one init skills --dry-run` and `one init skills`, then update with `one skills update --project` or `npx skills update --project`. Use `one init skills --install-recommend` to select missing stack skills, choose agents/location/install mode once, and review a batch installation through the upstream Skills API. Failed targets can be retried separately. Add `--dry-run` to preview or `--yes` to use the recommendations and default settings without prompts. Initialization preserves existing files; upstream updates follow repository master independently of CLI releases. See [skill installation and testing](packages/agent-skills/README.md).

模板和 One 官方技能随 CLI 发布。新工作区从内置文件安装技能，并在标准锁文件中登记 GitHub 来源。旧本地副本用 `one init skills --dry-run` 预览、`one init skills` 登记，后续统一用 `one skills update --project` 或 `npx skills update --project` 更新。用 `one init skills --install-recommend` 选择缺失的技术栈技能，统一选择 Agent、位置和安装方式，确认后通过上游 Skills API 批量安装，失败项可单独重试；加 `--dry-run` 预览，加 `--yes` 跳过询问并使用推荐项及默认设置。初始化保留已有文件；上游更新跟随仓库 master，独立于 CLI 发版。详见[技能安装与测试](packages/agent-skills/README.md)。

## Bun, Deno and Python workspaces / Bun、Deno 与 Python 工作区

Bun and Deno projects share the pnpm workspace with Node.js projects. Python projects share a uv workspace and its root `uv.lock` and `.venv`. Each runtime is pinned in `mise.toml`; `one run` prepares dependencies before running tasks.

Bun、Deno 项目与 Node.js 项目共用 pnpm workspace。Python 项目共用 uv workspace 及根目录的 `uv.lock`、`.venv`。运行时版本固定在 `mise.toml` 中，`one run` 在运行任务前准备依赖。

```sh
one add bun-api --name bun-service --yes
one add deno-api --name deno-service --yes
one add python-api --name python-service --yes
one add python-lib --name shared-utils --yes
one run check
one run dev -p python-service
```

Each runtime has an API service and library starter (`bun-lib`, `deno-lib`, `python-lib`). The API starters use the runtime's built-in HTTP server. Bun and Deno expose tasks through `package.json`; Python declares tasks under `[tool.one.tasks.<name>]` in `pyproject.toml`, using `run` argument arrays. One also discovers `[project.scripts]` entry points. After editing task definitions or local package dependencies, run `one init mise` to update task discovery and build ordering. Use its `--bun-version`, `--deno-version`, `--python-version` and `--uv-version` options to set exact runtime versions. `one exec` runs arbitrary commands with the selected project's environment.

每种运行时都提供 API 服务与共享库模板（`bun-lib`、`deno-lib`、`python-lib`），API 模板使用运行时内置 HTTP 服务。Bun、Deno 通过 `package.json` 声明任务，Python 在 `pyproject.toml` 的 `[tool.one.tasks.<name>]` 中通过 `run` 参数数组声明任务，同时支持发现 `[project.scripts]` 命令入口。修改任务或本地包依赖后执行 `one init mise`，更新任务发现和构建顺序。可用 `--bun-version`、`--deno-version`、`--python-version`、`--uv-version` 设置精确版本。`one exec` 可在所选项目环境中执行任意命令。

Use `workspace:*` for local pnpm dependencies. For Python, declare the package in `project.dependencies` and set `package-name = { workspace = true }` under `tool.uv.sources`. Python members must have compatible dependency and Python version requirements because uv resolves the workspace together. Dependency preparation synchronizes all Python members to retain dependencies used by other services. Deno starters use manual `node_modules` management for pnpm; declare npm dependencies in `package.json`.

pnpm 本地依赖使用 `workspace:*`。Python 在 `project.dependencies` 声明包，并在 `tool.uv.sources` 下设置 `package-name = { workspace = true }`。uv 统一解析工作区，Python 成员的依赖和 Python 版本要求需要兼容。依赖准备会同步全部 Python 成员，保留其他服务使用的依赖。Deno 模板通过手动管理 `node_modules` 对接 pnpm，npm 依赖声明在 `package.json` 中。

## Flutter workspaces / Flutter 工作区

Flutter apps and widget libraries use the `flutter` toolchain and native pub workspaces. One pins the Flutter SDK (including Dart), adds members to the root `pubspec.yaml`, and resolves shared dependencies before tasks. Commit the root `pubspec.lock`. Project tasks live in `Taskfile.yml`; local pub dependencies participate in One's task graph. Set a different SDK with `one init mise --flutter-version X.Y.Z`.

Flutter 应用与组件库使用 `flutter` 工具链及原生 pub workspace。One 固定 Flutter SDK（包含 Dart），将成员加入根目录 `pubspec.yaml`，并在执行任务前解析共享依赖。请提交根目录 `pubspec.lock`。项目任务定义在 `Taskfile.yml`，本地 pub 依赖会进入 One 任务图。通过 `one init mise --flutter-version X.Y.Z` 可设置其他 SDK 版本。

```sh
one add flutter-app --name mobile --yes
one add flutter-lib --name shared-ui --yes
one dev -p mobile -- -d chrome
one test -p mobile
one build -p mobile
one run platforms -p mobile -- --platforms android,ios
one build -p mobile -- apk
```

The app starter includes a Web entry point. The explicit `platforms` task uses `flutter create` to generate native runners without overwriting existing files. Android, iOS and desktop builds require their platform SDKs; signing and deployment are separate steps. Widget libraries are consumed as source and expose check, test and format tasks.

应用模板包含 Web 入口。通过 `platforms` 任务显式调用 `flutter create` 生成原生平台工程，不覆盖已有文件。Android、iOS 和桌面构建需要各自的平台 SDK，签名与发布单独执行。组件库以源码形式供应用使用，提供检查、测试和格式化任务。

## Repository / 仓库

Native macOS starters: [SwiftUI window app](packages/templates/swift-macos-app) and [menu-bar utility](packages/templates/swift-menubar-app). Both use Xcode's Swift toolchain with Task-based app/DMG builds and opt-in Developer ID signing and notarization.

原生 macOS 模板：[SwiftUI 窗口应用](packages/templates/swift-macos-app)与[菜单栏工具](packages/templates/swift-menubar-app)。均使用 Xcode 提供的 Swift 工具链，通过 Task 构建应用与 DMG，并支持可选的 Developer ID 签名与公证。

| Path / 路径 | Responsibility / 职责 |
| --- | --- |
| `packages/cli` | Go CLI / 命令行入口 |
| `packages/one-core` | Shared business and platform code / 共享业务与平台能力 |
| `services/desktop-api` | Desktop HTTP API / 桌面 HTTP 后端 |
| `apps/desktop` | React + Tauri desktop / 桌面应用 |
| `apps/website` | Next.js static homepage / 静态官网 |
| `packages/templates` | Project templates, including Fumadocs / 项目模板，包含 Fumadocs |
| `packages/agent-skills` | Maintained agent skills / AI 技能源码 |

Development conventions: [AGENTS.md](AGENTS.md), [CONTRIBUTING.md](CONTRIBUTING.md). Release procedure: [RELEASING.md](RELEASING.md).

开发约定见上述文件，发布统一通过 `mise run release`。

## License / 许可证

[MIT](LICENSE)

Native iOS starter: [SwiftUI iPhone and iPad app](packages/templates/swift-ios-app), with simulator builds, unsigned device archives and opt-in local signed export. Uses the same Swift/Task discovery as the macOS templates; App Store upload is separate.

原生 iOS 模板：[SwiftUI iPhone 与 iPad 应用](packages/templates/swift-ios-app)，支持模拟器构建、未签名真机归档及可选的本机签名导出。复用 macOS 模板的 Swift/Task 发现机制，商店上传单独配置。
