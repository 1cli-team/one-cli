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
| `one ci` | 查看或管理可选的持续集成 | `one ci` |
| `one run` | 注入项目 `.env` 后执行任意命令 | `one run -- npm test` |
| `one configure` | 配置机器级 endpoint profile | `one configure` |
| `one serve` | 启动本地 Workspace、Project 与 Profile Dashboard | `one serve` |

## 创建 workspace

```bash
one create [dir] [--name <name>] [--env-provider dotenv|infisical] [--yes]
```

`[dir]` 是目标目录，工作区名称默认取 `basename(dir)`。create 只创建空工作区，默认使用本地 dotenv 和 `one dev`；不配置 CI、不问项目、不问部署。

详见 [`one create`](/zh/docs/create/)。

## 添加项目

```bash
one add # 进入交互界面进行选择
one templates # 查看有哪些模板
one add <template-id> --name <project-name> [--yes] # 直接添加某个技术栈
```

直接 `one add` 会按目录分成应用、服务、共享库三类，再询问技术栈和项目名；文档站归在应用中。它不配置 CI，也不询问部署。普通 add 保持部署未配置，直到 `one deploy <project>`；`--deploy-provider` 只作为高级自动化选项保留。

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

## 本机连接

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
one configure open
```

`configure` 管理本机连接和偏好设置。没有连接时，无参调用进入建立连接向导；已有连接时显示简洁概览。`show` / `use` / `remove` 可在终端选择，脚本仍可显式传服务 ID 和 `--profile`。密钥只保存在本机，不写入工作区或 Git。

支持的 `<pair>`：

| domain | backend |
|---|---|
| `env` | `infisical` |
| `container` | `docker` |
| `container` | `dockerhub`, `ghcr`, `acr` |
| `deploy` | `aliyun-oss`, `tencent-cos`, `aws-s3`, `minio`, `rustfs`, `r2` |
| `deploy` | `kustomize`, `vercel`, `cloudflare`, `edgeone` |

本地 `.env` 文件不需要本机连接。
本机连接写到 `~/.config/one/config.json` 和 `~/.config/one/credentials.json`。环境感知的 Workspace/Project 选择只保存名字，位于 `~/.config/one/profile-bindings.json`。敏感字段默认掩码，只有 `show --reveal` 会显示明文；这些文件都不会改动 `one.manifest.json`。
添加 token 时推荐使用 `one configure open`，避免把密钥交给 AI agent。

## 交互模式速查

| 命令 | 交互模式 |
|---|---|
| `one create` | 有；无参时询问目标目录和可选工作区名称 |
| `one add` | 有；无参时选择项目类型、技术栈和项目名 |
| `one configure` | 有；无参或 `one configure add` 进入本机连接向导 |
| `one env set` | 有；隐藏输入值、选择作用域、确认覆盖；脚本显式传值 |
| `one container build` | 半交互；TTY 下缺少构建版本时可选择版本，CI 用 `--build-version` |
| `one deploy` | 首次部署询问项目、目标类别/服务和本机连接；脚本传 `--provider` / `--profile` |
| `one dev` | Node 依赖缺失时询问是否安装，否则直接启动 |
| `one ci disable` | 删除生成的工作流前先确认；拒绝时成功退出 |
| `one templates` / `one run` | 无交互式向导；通过参数控制行为 |
| `one serve` | 不是终端向导；它打开本地 Dashboard 管理 Workspace、Project 与本机连接 |

## 本地 Web UI

```bash
one serve [--host 127.0.0.1] [--port 0] [--open=false]
```

启动仅绑定 loopback 的本地 HTTP 服务，用浏览器手工编辑 `env / deploy / container` Profile、选择环境感知的本机绑定，并在发布前审阅类型化的 Workspace Backend 或 Project 配置草稿及 revision 校验。Workspace 源码与非白名单 Manifest 字段保持只读。这个入口会处理 API key、kubeconfig path、registry token 等敏感字段，设计上是给人类使用，不给 AI agent 直接读写凭据。

详见 [`one serve`](/zh/docs/serve/)。

## 容器

```bash
one container info
one container build [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
one container push  [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
```

`one container` 读取每个项目的 Dockerfile 和 manifest 里的 container 配置。裸 `build` 默认本地构建 `<workload>:<version>`；传 `--profile` 或解析到机器本地 registry 绑定/default 时，会使用 registry-qualified tag 并执行登录。`push` 需要 registry Profile，必要时会把本地镜像 retag 后推送。

## 本地开发

```bash
one dev [project] [--dry-run]
```

读取项目的 dev 命令并用内置 supervisor 并行启动。位置参数只启动一个项目；`--project` 为旧脚本保留。Node 依赖缺失时可确认安装并继续。

## 部署

```bash
one deploy [project] [--provider <target>] [--profile <connection>] [--dry-run]
```

首次部署只展示当前仓库已经实现、且与技术栈兼容的部署目标，然后询问本机连接。选择“稍后配置”会成功退出且不修改工作区；后续部署复用项目已保存的目标。

`--env <name>` 一次性覆盖目标环境；`--dry-run` 打印 docker / kubectl / s3 / platform CLI 计划，不触碰远端。

## 持续集成

```bash
one ci
one ci enable [project]
one ci sync [project]
one ci disable [project]
```

持续集成是可选能力，`one create` 和 `one add` 都不会自动添加。当前版本生成
GitHub Actions 工作流。不传 `[project]` 时处理全部项目（`sync` 只更新已经启用
持续集成的项目）。

详见 [持续集成](/zh/docs/ci/)。

## 注入环境变量后运行

```bash
one run [-p <name|path>] [--env-provider dotenv|infisical] [--env <env>] -- <command> [args...]
```

子进程总是在解析出的项目目录里执行。默认从 workspace manifest 读取 env provider，也可以用 `--env-provider` 强制走 dotenv 或 Infisical。

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
one help --all
one <command> --help
```

`one --help` 只展示六个日常核心任务；`one help --all` 展示完整命令；具体 flag 以 `one <command> --help` 为准。
