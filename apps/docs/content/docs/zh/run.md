---
title: one run
description: 给任意命令注入项目环境变量，并在解析出的项目目录内执行。
---

`one run` 类似 `infisical run` / `dotenv run`：它先解析当前项目，再从 workspace 选择的 env provider 取环境变量，把它们注入子进程，然后执行你传入的命令。

## 用法

```bash
one run [-p <name|path>] [--env-provider dotenv|infisical] [--env <env>] -- <cmd> [args...]
```

也可以省略 `--`，但脚本里建议保留，避免把子命令 flag 误解析成 One CLI flag。

## 参数

| 参数 | 说明 |
|---|---|
| `-p, --project <name|path>` | 选择项目；不传时从当前目录推导 |
| `--env-provider dotenv|infisical` | 强制使用指定 env provider；默认取 workspace manifest |
| `--env <env>` | 使用指定环境；默认取 manifest 的默认环境 |
| `-o, --output <fmt>` | 只影响 One CLI 自己的输出；子进程 stdout/stderr 原样透传 |

## 交互模式

`one run` 没有交互式向导；它只按参数解析项目、环境和子命令。因为后面的命令可能带自己的 flag，脚本里建议保留 `--` 分隔符。

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
- [`one dev`](/zh/docs/dev/) — 启动整个 Procfile.dev
