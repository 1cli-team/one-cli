---
title: one exec
description: 给任意命令注入项目环境变量，并在解析出的项目目录内执行。
---

`one exec` 解析项目，在配置了 Infisical 时注入变量，然后在项目目录执行命令。

One 不输出拉取到的变量值。子进程的 stdout、stderr、颜色、格式和退出码保持原样。

## 用法

```bash
one exec [-p <name|path>] [--env <env>] -- <cmd> [args...]
```

必须使用 `--` 分隔 One 参数和子命令参数。项目也可以写成位置参数，例如 `one exec web -- pnpm build`。

## 参数

| 参数 | 说明 |
|---|---|
| `-p, --project <name|path>` | 选择项目；不传时从当前目录推导 |
| `--env <env>` | 使用指定环境；项目模式默认固定为 `dev` |
| `--dry-run` | 仅输出目录、原始参数和 runtime，不执行 mise、不读取密钥、不启动命令 |
| `-o, --output <fmt>` | 只影响 One CLI 自己的输出；子进程 stdout/stderr 原样传递，包括其主动打印的变量值 |

## 交互模式

`one exec` 没有交互式向导；它按参数解析项目、环境和子命令。子命令参数放在 `--` 后面。

## mise 工具环境

命令和原有参数保持不变。根目录存在 One 生成的 `mise.toml` 时，`one exec` 自动通过 mise 准备项目工具和环境，再注入 One 环境变量并执行原始命令。新 workspace 自动生成该配置；旧 workspace 未启用时继续使用原有工具。

覆盖顺序为：父进程环境 → mise 环境 → 当前项目的 One 环境变量。项目目录、参数边界、标准 IO 和应用退出码保持原有语义，不需要 `mise activate`。

无需单独安装 mise：One 优先使用兼容的系统版本，否则复用或从官方下载到自己的目录；本地没有可用程序时首次运行需要联网，程序被删后会按需恢复。创建工作区和添加项目时自动信任完整的 One 生成配置；自定义配置继续遵循 mise 规则，审查后通过 `one mise trust` 授权。离线配置见 [One 自动管理 mise](/zh/docs/installation/#one-自动管理-mise)，旧项目启用和版本调整见 [`one init mise`](/zh/docs/login/#本机偏好与工作区工具)。

## 示例

```bash
one exec -- pnpm test
one exec -p web -- pnpm run build
one exec -p apps/web -- pnpm lint
one exec --env staging -- pnpm run e2e
```

子进程总在解析出的项目目录里运行，因此 `npm start`、`pnpm build`、`go test ./...` 会看到项目自己的配置文件。

## PATH 与环境变量

`one exec` 会把环境变量 merge 到子进程环境里，默认覆盖同名 shell 变量。同时它会把下面路径注入 PATH 前面：

```text
<project>/node_modules/.bin
<workspace>/node_modules/.bin
```

这样在 pnpm monorepo 里直接执行 `vite`、`next`、`astro` 等二进制也能解析到。

## Infisical 注入

Manifest 的 `[env.infisical]` 绑定启用 Infisical 注入。未绑定时，命令继承 shell 环境。One CLI 不加载 `.env` 文件，认证或拉取失败会停止执行，不回退到本地文件。通过 `one login` 登录。

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `NOT_ONE_PROJECT` | 在 workspace 内运行，或进入某个项目目录 |
| `SUBPROJECT_NOT_FOUND` | `-p` 改成 manifest 里的项目名或 `path` |
| `RUN_COMMAND_NOT_FOUND` | 确认命令在 PATH、项目 `node_modules/.bin` 或 workspace `node_modules/.bin` 内 |
| `INFISICAL_AUTH_MISSING` | 先 `one login` |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [环境变量注入命令](/zh/tutorials/env-vars/) — 真实使用场景
- [`one env`](/zh/docs/env-vars/) — 管理 Infisical 环境变量
- [`one dev`](/zh/docs/dev/) — 启动全部可开发项目


## 使用全局凭据

```bash
one exec --global --env dev --path /oss --keys OSS_ACCESS_KEY_ID,OSS_ACCESS_KEY_SECRET -- upload-assets
one exec --global --env dev --path /oss --dry-run -- upload-assets
```

环境和目录必须显式指定。只读取该层目录，指定 `--keys` 时只获取所选变量；dry-run 不读取凭据。全局模式不加载项目环境或仓库内的隐式命令路径。命令在当前目录运行。One 不打印注入值；子进程输出保持原样，子进程主动打印的变量值不会被遮盖。
