---
title: 命令总览
description: one 顶层命令、常用子命令、输出模式和 agent 自动化契约速查。
---

`one cli` 是一个单文件二进制。它负责创建 workspace、添加项目、管理环境变量 / endpoint profile、执行本地开发 / 容器 / 部署流程，并为 agent / CI 提供稳定的 JSON 输出。

**适合读这页的人**：刚装好 one cli 想知道有哪些命令；记不清某个 flag 的人；

**读完会**：知道每个公开命令的一句话用途、最小例子、常用子命令，以及该跳到哪一页继续看细节。

## 顶层命令速查

| 命令 | 用途 | 最小例子 |
|---|---|---|
| `one create` | 创建新 workspace | `one create my-app` |
| `one add` | 交互式或从内置模板添加项目 | `one add` |
| `one templates` | 查看可用模板 | `one templates` |
| `one env` | 管理 workspace 的 dotenv / Infisical 环境变量 | `one env list` |
| `one container` | 查看、构建、推送 Dockerfile-driven 镜像 | `one container info` |
| `one dev` | 并行启动所有项目的本地开发进程 | `one dev` |
| `one deploy` | 按 project 派发 kustomize / S3-compatible / Vercel / Cloudflare / EdgeOne 部署 | `one deploy --dry-run` |
| `one run` | 注入项目 `.env` 后执行任意命令 | `one run -- npm test` |
| `one configure` | 配置机器级 endpoint profile | `one configure` |
| `one serve` | 启动本地 Web UI 手工编辑敏感 profile | `one serve` |
| `one skills` | 安装 / 刷新 bundled `one-cli` skill | `one skills install` |

## 创建 workspace

```bash
one create [dir] [--name <name>] [--env-provider dotenv|infisical] [--yes]
```

`[dir]` 是目标目录，不是项目名。默认项目名取 `basename(dir)`；需要不同名字时传 `--name`。直接运行 `one create` 会交互式询问目标目录和可选项目名；不传 `--env-provider` 时默认使用 `dotenv`，需要 Infisical 就显式传 `--env-provider infisical`。

详见 [`one create`](/zh/docs/create/)。

## 添加项目

```bash
one add # 进入交互界面进行选择
one templates # 查看有哪些模板
one add <template-id> --name <project-name> [--deploy-provider <id>] [--yes] # 直接添加某个模板并且选择部署方式
```

第一次用时可以直接 `one add`，按交互式选择器选择模板分类、模板和项目名。需要明确命令时，先跑 `one templates`，把输出里的模板 ID 填到 `one add <template-id>` 位置。

API / SSR 模板通常启用 `container/docker + deploy/kustomize`；
静态前端模板通常启用 S3 兼容部署，支持多个S3 平台；
移动端、库、Electron 模板默认不参与 deploy / container。

详见 [`one add`](/zh/docs/add/)。

## 模板

```bash
one templates
one templates -o json
```

 `one templates` 会列出内置模板。agent / CI 建议使用 `-o json` 读取模板 ID、分类、toolchain 和兼容 backend。

详见 [`one templates`](/zh/docs/templates-cmd/)。

## 环境变量

```bash
one env get <KEY> [--env <env>] [-p <name|path>]
one env set <KEY[=VALUE]> [VALUE] [--env <env>] [-p <name|path>]
one env list [--env <env>] [-p <name|path>]
one env pull [--env <env>] [-p <name|path>] [--force] [--dry-run]
```

`one env` 操作 workspace 当前选择的 env 后端。`dotenv` 读写本地 `.env` overlay；`infisical` 支持远端 get / set / list / pull。`--env` 选择 dev / staging / prod 等环境；`-p / --project` 可按 manifest 里的项目名或相对路径选项目。

详见 [`one env`](/zh/docs/env-vars/)。

## 机器级 profile

```bash
one configure
one configure add
one configure add <pair> --profile <name> [backend flags...] [--use]
one configure list [pair]
one configure current [pair]
one configure show <pair> --profile <name> [--reveal]
one configure use <pair> --profile <name>
one configure remove <pair> --profile <name>
one configure locale [auto|zh-CN|en-US]
```

`configure` 是机器级 endpoint / 凭据入口。无参 `one configure` 或 `one configure add` 会打开交互式向导；非交互脚本应显式传 `<pair>` 和 `--profile`。一次配置到自己的电脑上，后续可以复用

支持的 `<pair>`：

| domain | backend |
|---|---|
| `env` | `infisical` |
| `container` | `docker` |
| `container` | `dockerhub`, `ghcr`, `acr` |
| `deploy` | `aliyun-oss`, `tencent-cos`, `aws-s3`, `minio`, `rustfs`, `r2` |
| `deploy` | `kustomize`, `vercel`, `cloudflare`, `edgeone` |

`env/dotenv` 是 workspace 的本地 `.env` 后端，不需要机器级 profile。
profile 写到 `~/.config/one/config.json` 和 `~/.config/one/credentials.json`。敏感字段默认掩码，只有 `show --reveal` 会显示明文。
添加的时候推荐使用 one serve 进行 token 的配置，防止 token 给 ai 以后泄漏

## 交互模式速查

| 命令 | 交互模式 |
|---|---|
| `one create` | 有；无参时询问目标目录和可选项目名 |
| `one add` | 有；无参时选择模板分类、模板、项目名，必要时选择 deploy 后端 |
| `one configure` | 有；无参或 `one configure add` 进入 profile 配置向导 |
| `one skills install` | 有；无参时多选要安装到哪些 agent |
| `one env set` | 半交互；遇到未知环境或覆盖已有值时会确认，脚本用 `--yes` |
| `one container build` | 半交互；TTY 下缺少构建版本时可选择版本，CI 用 `--build-version` |
| `one deploy` | 半交互；kustomize 缺少构建版本或 Cloudflare 缺 profile 时可能询问，CI 用显式参数 |
| `one templates` / `one dev` / `one run` | 无交互式向导；通过参数控制行为 |
| `one serve` | 不是终端向导；它打开本地 Web UI 让人手工编辑敏感 profile |

## 本地 Web UI

```bash
one serve [--host 127.0.0.1] [--port 0] [--open=false]
```

启动仅绑定 loopback 的本地 HTTP 服务，用浏览器手工编辑 `env / deploy / container` profile。这个入口会处理 API key、kubeconfig path、registry token 等敏感字段，设计上是给人类使用，不给 AI agent 直接读写凭据。

详见 [`one serve`](/zh/docs/serve/)。

## 容器

```bash
one container info
one container build [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
one container push  [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
```

`one container` 读取每个项目的 Dockerfile 和 manifest 里的 container 配置。裸 `build` 默认本地构建 `<workload>:<version>`；传 `--profile` 或 manifest pin 了 registry profile 时，会使用 registry-qualified tag 并执行登录。`push` 需要 registry profile，必要时会把本地镜像 retag 后推送。

## 本地开发

```bash
one dev [-p <name|path>] [--dry-run]
```

读取 `one.manifest.json` 里每个项目的 `domains.dev.command`，用内置 supervisor 并行启动所有 dev 进程（无需安装第三方 runner）。`-p / --project` 只启动一个项目；`--dry-run` 只打印每个项目的命令。

## 部署

```bash
one deploy [-p <name|path>] [--profile <name>] [--env <env>] [--env-provider dotenv|infisical] [--build-version <version>] [--dry-run]
```

`deploy` 按 project 派发到 manifest 声明的 deploy backend。后端 / SSR 项目通常走 `kustomize`；静态前端可走 S3 兼容后端（Aliyun OSS / Tencent COS / AWS S3 / MinIO / RustFS / R2）；前端托管可走 Vercel / Cloudflare / EdgeOne。

`--env <name>` 一次性覆盖目标环境；`--dry-run` 打印 docker / kubectl / s3 / platform CLI 计划，不触碰远端。

## 注入环境变量后运行

```bash
one run [-p <name|path>] [--env-provider dotenv|infisical] [--env <env>] -- <command> [args...]
```

子进程总是在解析出的项目目录里执行。默认从 workspace manifest 读取 env provider，也可以用 `--env-provider` 强制走 dotenv 或 Infisical。

## Agent skills

```bash
one skills install # 通过交互选择要给哪些ai安装skills
one skills install --yes
one skills install --agent claude-code # 给指定 ai安装 skills
```

安装 / 刷新 bundled `one-cli` skill 到本机检测到的 coding agent。当前 bundled skill：

| skill | 用途 |
|---|---|
| `one-cli` | 新建 workspace、追加模板项目、补依赖、查命令 / JSON / 错误码 |

详见 [安装 Skill 到 Agent](/zh/tutorials/skills-install/)。

## 输出模式

每个命令都支持同一组通用输出参数：

| 触发条件 | 模式 |
|---|---|
| `-o json` 或 `--output json` | 强制 JSON，2-space pretty-print |
| `-o yaml` 或 `--output yaml` | 强制 YAML，与 JSON 同 schema |
| `-o text` 或 `--output text` | 强制人类格式 |
| 默认 + pipe / 非 TTY | JSON |
| 默认 + TTY | 彩色人类格式 |

直接打 `one templates` 会看到终端友好的输出；
agent / CI 通过 pipe 读取时默认拿 JSON。
脚本里仍建议显式写 `-o json`，避免执行环境变化影响解析。

## 元命令

```bash
one --version
one --help
one <command> --help
```

`one --help` 只展示顶层命令；具体 flag 以 `one <command> --help` 为准。想看错误码请读 [错误码大全](/zh/docs/error-codes/)。
