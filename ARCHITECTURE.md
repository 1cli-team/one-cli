# One CLI architecture / 架构边界

This file records source boundaries for maintainers and agents. Discover commands from the current executable (`one help --all`) and tasks from workspace configuration (`one run --list -o json`).

此文件仅记录维护者和 AI 所需的代码边界。命令与任务以当前可执行文件和工作区配置为准。

| Path / 路径 | Ownership / 职责 |
| --- | --- |
| `packages/cli` | `one` executable, Cobra commands, terminal input/output / CLI 入口、命令和终端交互 |
| `packages/one-core` | Shared use cases, domain models, integrations and embedded templates/skills / 共享业务、集成、内置模板与技能 |
| `services/desktop-api` | `one-api` executable, authenticated loopback HTTP API / 桌面 HTTP 后端 |
| `apps/desktop` | React UI and Tauri host; bundles `one-api` and `one` / 桌面界面、原生能力及后端进程管理 |
| `apps/website` | Next.js static homepage; exports to `out/` / Next.js 静态官网 |
| `packages/templates` | Project template sources, including `nextjs-site` and `fumadocs-docs` / 项目模板源 |
| `packages/agent-skills` | Agent guidance embedded in CLI builds / CLI 构建时内置的 AI 指引 |

`go.work` contains three modules: CLI, One Core and Desktop API. Go projects inside templates remain independent fixtures.

`go.work` 仅登记 CLI、One Core、Desktop API 三个模块；模板中的 Go 项目独立管理。

## Go boundaries / Go 分层

- `one-core/pkg/bootstrap` composes private adapters and exposes dependencies to transports. Transports parse input and render results; reusable workflows belong to application services or feature modules.
- `one-core/pkg/application` owns use cases and execution state. `execution.Scope` holds a standard `context.Context`; `Derive` copies scope fields while retaining cancellation, deadlines and values. Resource owners manage their own cleanup.
- `one-core/pkg/core` owns domain models; `pkg/ports` defines integration contracts; `internal/adapters` implements them. The active environment backend is Infisical.
- `pkg/platform` provides shared technical capabilities. `internal/resources/bundled` embeds generated templates, registry and skills; regenerate through `one mise run sync-bundled`.

入口通过 bootstrap 组装依赖。可复用业务放在 application / modules，接口契约放在 ports，具体集成放在 internal/adapters。执行上下文使用标准库 context；资源由实际创建者负责清理。

## Desktop boundary / 桌面边界

React calls the Go HTTP API directly. Rust owns native capabilities, backend startup, authentication handshake and child-process lifetime. The API is separate from the CLI command tree. Desktop initialization, bundling and updates are coordinated by `scripts/desktop.mts` and `apps/desktop/src-tauri`.

React 直接访问 Go HTTP；Rust 负责原生能力、后端启动、认证握手和子进程生命周期。`one create` 与 Desktop 的“打开工作区”维护本机工作区登记，敏感值按需从凭据存储读取。

The `SERVE_*` codes still used by the Desktop API are stable wire values. They do not imply a built-in CLI command. Generic task shorthands remain available for user-defined tasks.

Desktop API 仍使用的 `SERVE_*` 是稳定协议值；用户自定义任务继续支持通用简称。

## Source of truth / 事实来源

Use `one.manifest.toml`, `mise.toml`, template registry, command help and current source for details. Follow `AGENTS.md` for changes and `RELEASING.md` for releases. The website contains no documentation runtime; the CLI's Fumadocs template remains available.

细节以 manifest、任务配置、模板 registry、命令帮助和实际源码为准；修改遵循 AGENTS，发布遵循 RELEASING。官网不再承载文档运行时，CLI 的 Fumadocs 模板继续保留。
