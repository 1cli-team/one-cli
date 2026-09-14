---
title: one run
description: 给任意命令注入项目环境变量，并在解析出的项目目录内执行。
---

`one run` 类似 `infisical run` / `dotenv run`：它先解析当前项目，再从 workspace 选择的 env provider 取环境变量，把它们注入子进程，然后执行你传入的命令。

## 用法

```bash
one run [-p <name|path>] [--env-provider dotenv|infisical] [--env <env>] -- <cmd> [args...]
```

必须使用 `--` 分隔 One 参数和子命令参数。项目也可以写成位置参数，例如 `one run web -- pnpm build`。

## 参数

| 参数 | 说明 |
|---|---|
| `-p, --project <name|path>` | 选择项目；不传时从当前目录推导 |
| `--env-provider dotenv|infisical` | 强制使用指定 env provider；默认取 workspace manifest |
| `--env <env>` | 使用指定环境；默认取 manifest 的默认环境 |
| `--dry-run` | 仅输出目录、原始参数和 runtime，不执行 mise、不读取密钥、不启动命令 |
| `-o, --output <fmt>` | 只影响 One CLI 自己的输出；子进程 stdout/stderr 原样透传 |

## 交互模式

`one run` 没有交互式向导；它按参数解析项目、环境和子命令。子命令参数放在 `--` 后面。

## mise 工具环境

命令和原有参数保持不变。根目录存在 One 生成的 `.mise/conf.d/one.toml` 时，`one run` 自动通过 mise 准备项目工具和环境，再注入 One 环境变量并执行原始命令。新 workspace 自动生成该配置；旧 workspace 未启用时继续使用原有工具。

覆盖顺序为：父进程环境 → mise 环境 → 当前项目的 One 环境变量。项目目录、参数边界、标准 IO 和应用退出码保持原有语义，不需要 `mise activate`。

无需单独安装 mise：发布的 One 已内置固定版本，首次运行从自身解压到缓存，无需下载 mise；之后直接复用。配置信任遵循 mise 自身规则；需要显式审批时设置 `MISE_PARANOID=1`，审查配置后通过 `one mise trust` 授权。离线配置见 [One 自动管理 mise](/zh/docs/installation/#one-自动管理-mise)，旧项目启用和版本调整见 [`one configure mise`](/zh/docs/configure/#mise-工作区工具配置)。

## 示例

```bash
one run -- npm test
one run -p web -- npm run build
one run -p apps/web -- pnpm lint
one run --env-provider dotenv -- npm test
one run --env staging -- npm run e2e
```

子进程总在解析出的项目目录里运行，因此 `npm start`、`pnpm build`、`go test ./...` 会看到项目自己的配置文件。

## PATH 与环境变量

`one run` 会把环境变量 merge 到子进程环境里，默认覆盖同名 shell 变量。同时它会把下面路径注入 PATH 前面：

```text
<project>/node_modules/.bin
<workspace>/node_modules/.bin
```

这样在 pnpm / turbo monorepo 里直接执行 `vite`、`next`、`astro` 等二进制也能解析到。

## env provider

| provider | 行为 |
|---|---|
| `dotenv` | 读取项目 `.env` overlay |
| `infisical` | 联网从 Infisical 拉取当前环境变量 |
| 空 | 读取 workspace manifest 记录的 provider |

`--env-provider infisical` 需要先配置 `env/infisical` profile；离线或本地调试可以用 `--env-provider dotenv`。

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `NOT_ONE_PROJECT` | 在 workspace 内运行，或进入某个项目目录 |
| `SUBPROJECT_NOT_FOUND` | `-p` 改成 manifest 里的 `name` 或 `relativeDir` |
| `RUN_COMMAND_NOT_FOUND` | 确认命令在 PATH、项目 `node_modules/.bin` 或 workspace `node_modules/.bin` 内 |
| `ENV_FILE_NOT_FOUND` | 建项目 `.env`，或切到 `--env-provider infisical` |
| `INFISICAL_AUTH_MISSING` | 先 `one configure add env/infisical --profile <name> --use` |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [环境变量注入命令](/zh/tutorials/run-passthrough/) — 真实使用场景
- [`one env`](/zh/docs/env-vars/) — 设置 / 拉取环境变量
- [`one dev`](/zh/docs/dev/) — 启动全部可开发项目
