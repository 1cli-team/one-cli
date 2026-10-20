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

## Repository / 仓库

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
