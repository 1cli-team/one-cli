# One CLI architecture / 架构边界

This file records source boundaries for maintainers and agents. Discover commands from the current executable (`one help --all`) and tasks from workspace configuration (`one run --list -o json`).

此文件仅记录维护者和 AI 所需的代码边界。命令与任务以当前可执行文件和工作区配置为准。

| Path / 路径 | Ownership / 职责 |
| --- | --- |
| `packages/cli` | `one` executable, Cobra commands, terminal input/output / CLI 入口、命令和终端交互 |
| `packages/one-core` | Shared use cases, domain models, integrations and embedded templates/skills / 共享业务、集成、内置模板与技能 |
| `services/desktop-api` | `one-api` executable, authenticated loopback HTTP API / 桌面 HTTP 后端 |
| `services/gateway` | Standalone `one-gateway` transport for task dispatch and Codex App Server / 任务派发与 Codex App Server 通信服务 |
| `apps/desktop-renderer` | React interface / 桌面界面 |
| `services/desktop-main` | Electron host; bundles `one-api` and `one` / 原生能力及后端进程管理 |
| `packages/desktop-preload` | Typed contextBridge API / 类型化原生桥接 |
| `apps/website` | Next.js static homepage; exports to `out/` / Next.js 静态官网 |
| `packages/templates` | Module template sources, including `nextjs-site` and `fumadocs-docs` / 模块模板源 |
| `packages/agent-skills` | Agent guidance embedded in CLI builds / CLI 构建时内置的 AI 指引 |

`go.work` contains four modules: CLI, One Core, Desktop API and Gateway. Go modules inside templates remain independent fixtures.

`go.work` 仅登记 CLI、One Core、Desktop API、Gateway 四个模块；模板中的 Go 模块独立管理。

## Go boundaries / Go 分层

- `one-core/pkg/bootstrap` composes private adapters and exposes dependencies to transports. Transports parse input and render results; reusable workflows belong to application services or feature modules.
- `one-core/pkg/application` owns use cases and execution state. `execution.Scope` holds a standard `context.Context`; `Derive` copies scope fields while retaining cancellation, deadlines and values. Resource owners manage their own cleanup.
- `one-core/pkg/core` owns domain models; `pkg/ports` defines integration contracts; `internal/adapters` implements runtime and storage adapters. Built-in vendor integrations live under `pkg/plugins`; the default composition installs Infisical, Git, and Linear.
- `pkg/platform` provides shared technical capabilities. `internal/resources/bundled` embeds generated templates, registry and skills; regenerate through `one mise run sync-bundled`.

入口通过 bootstrap 组装依赖。可复用业务放在 application / modules，接口契约放在 ports，运行时与存储适配器放在 internal/adapters，厂商集成放在 pkg/plugins。执行上下文使用标准库 context；资源由实际创建者负责清理。

## CLI lifecycle and plugins / CLI 生命周期与插件

The process handles version/identity probes first, initializes One storage, and handles private update workers before constructing the application. `cli.NewApplication` composes shared capabilities and registers plugin commands without opening credentials or making remote requests. Each invocation then resolves output mode and locale, refreshes command translations, and handles help and workspace summaries locally.

进程先处理版本与身份探测，再初始化 One 存储、处理私有更新进程。`cli.NewApplication` 装配共享能力、注册插件命令，构造期间不读取凭据、不发远端请求。每次执行先解析输出格式和语言、刷新命令翻译，再处理帮助和工作区摘要。

For command execution, `packages/cli/internal/lifecycle` runs `Start` for all plugins, then `BeforeCommand`, the command, `AfterCommand`, and cleanup. Start/before run in registration order; after/cleanup unwind in reverse order, including command errors, partial startup failure, and cancellation. Cleanup receives a bounded context independent of invocation cancellation. Help and local summaries do not start plugin resources. The caller owns signal handling and the final error/exit-code envelope.

命令执行顺序为 `Start → BeforeCommand → 命令 → AfterCommand → 清理`。启动与执行前钩子按注册顺序运行，执行后钩子与清理逆序运行；部分启动失败、命令错误和取消均会收尾。清理使用独立且有超时的上下文。帮助和本地摘要不启动插件资源，信号与最终错误、退出码由 CLI 统一处理。

`one-core/pkg/plugins.Plugin` contributes backend descriptors, `environment.Backend`, secret loaders, optional shared-credential loading, Git, and issue-tracking capabilities. `cli.Plugin` adds command registration and optional lifecycle hooks. Registries belong to one application; no `init()` registration or mutable global plugin registry is used. Duplicate plugin identities, backend registrations, global loaders, and command names/aliases fail during composition. `NewApplication()` constructs the CLI without vendor plugins; `builtin.go` installs Infisical, Git, and Linear explicitly.

`one-core/pkg/plugins.Plugin` 提供后端描述、`environment.Backend`、变量加载器、共享凭据加载器以及 Git 和 Issue 管理能力；`cli.Plugin` 增加命令注册和可选生命周期钩子。注册表属于单个应用，不使用 `init()` 自动注册或全局可变插件表。重复注册在装配时失败。`NewApplication()` 可构造不含厂商插件的 CLI，默认 `builtin.go` 显式安装 Infisical、Git 和 Linear。

Infisical owns its remote client, session storage, workspace binding, shared credentials, and CLI `login`/`logout`/`whoami`/`env` commands. Shared code is in `one-core/pkg/plugins/infisical`, private vendor code in its `internal/remote`, and Cobra adapters in `cli/internal/plugins/infisical`. The generic environment service dispatches through an injected backend; task and arbitrary-command execution use injected loaders. Existing `[env.infisical]` manifest fields, session files, protocol values, and CLI commands remain compatible. This is a compiled-in extension boundary, not dynamic Go `.so` loading or a separately installed executable.

Infisical 插件拥有远端客户端、会话存储、工作区绑定、共享凭据和账号/环境命令。通用环境服务通过注入的后端派发操作，任务和任意命令通过注入的加载器取变量。现有 `[env.infisical]`、会话文件、协议值和 CLI 命令保持兼容。插件随程序编译，不采用 Go `.so` 动态加载或独立安装的可执行文件。

Linear owns OAuth PKCE, token refresh, GraphQL access, and write-operation recovery in `one-core/pkg/plugins/linear`. The `modules/issuetracking` service resolves local project scopes and stores bindings. The CLI adapter and Desktop HTTP handlers use the same service. Tokens remain in the local plugin store and never enter renderer responses. Desktop uses its existing authenticated HTTP boundary. Setup and recovery behavior are documented in [the Linear plugin README](packages/one-core/pkg/plugins/linear/README.md).

Linear 插件负责 OAuth PKCE、令牌刷新、GraphQL 请求与写入结果恢复。`modules/issuetracking` 负责本地项目范围和绑定存储，CLI 与 Desktop HTTP 共用同一服务。令牌只保存在本机插件存储，renderer 通过现有认证 HTTP 边界调用业务接口。

## Desktop boundary / 桌面边界

React calls the Go HTTP API directly. Electron owns native capabilities, backend startup, authentication handshake and child-process lifetime. The API is separate from the CLI command tree. Desktop initialization, bundling and updates are coordinated by `scripts/desktop.mts` and `services/desktop-main`.

React 直接访问 Go HTTP；Electron 主进程负责原生能力、后端启动、认证握手和子进程生命周期。`one create` 与 Desktop 的“打开工作区”维护本机工作区登记，敏感值按需从凭据存储读取。

The `SERVE_*` codes still used by the Desktop API are stable wire values. They do not imply a built-in CLI command. Generic task shorthands remain available for user-defined tasks.

Desktop API 仍使用的 `SERVE_*` 是稳定协议值；用户自定义任务继续支持通用简称。

## Gateway boundary / Gateway 边界

One CLI owns code host configuration, PR operations, worktrees, issue workflow
state and Linear writeback. Desktop API uses One Core directly for local code
host settings. For remote execution, Desktop sends the bound project and Issue
ID through Gateway, which forwards requests and status to the execution
machine's One CLI and opens its Codex App Server connection.

One CLI 管理代码库配置、PR、worktree、需求流程状态和 Linear 回写。桌面 API
直接通过 One Core 管理本机代码库。远程执行时，Desktop 经 Gateway 向执行机器
的 One CLI 转发已绑定的项目与 Issue，并通过 Gateway 建立 Codex App Server
连接。部署说明见 [Gateway README](services/gateway/README.md)。

## Source of truth / 事实来源

Use `one.manifest.toml`, `mise.toml`, template registry, command help and current source for details. Follow `AGENTS.md` for changes and `RELEASING.md` for releases. The website contains no documentation runtime; the CLI's Fumadocs template remains available.

细节以 manifest、任务配置、模板 registry、命令帮助和实际源码为准；修改遵循 AGENTS，发布遵循 RELEASING。官网不再承载文档运行时，CLI 的 Fumadocs 模板继续保留。
