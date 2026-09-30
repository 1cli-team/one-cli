---
title: one env
description: 使用 set、unset、list 管理 Infisical 变量，并在命令执行时直接注入。
---

Infisical 是 One CLI 唯一管理的环境变量来源。变量在执行命令时拉取并注入子进程，One CLI 不读取、写入或导出 `.env` 文件。

## 绑定工作区

新工作区没有 Infisical 绑定，无需登录即可运行。先执行 `one login`，首次使用 `one env set` 保存变量时会初始化绑定。Dashboard 也仅在首次点击保存时初始化；打开页面、刷新和取消编辑不会创建项目。

绑定保存在 `one.manifest.toml` 中的 `[env.infisical]` 表：

```toml
[env.infisical]
siteUrl = "https://app.infisical.com"
projectId = "your-project-id"
environments = ["dev", "staging", "prod"]
```

Manifest 只记录绑定信息和环境 slug。值保存在 Infisical，登录使用系统 keyring 中的浏览器会话。

## 命令

```bash
one env
one env set DATABASE_URL -p api
one env set API_URL=https://example.com -p web --env dev --yes
one env set SIGNING_PRIVATE_KEY -p api --env dev --stdin < /secure/private-key.pem
one env list -p web --env dev
```

多行 PEM 和其他秘密可以通过 `--stdin` 读取，无需把内容放进命令参数。项目变量和 `--global` 共享凭据都支持该方式，保留内部换行，只去掉一个结尾换行符。输入上限为 1 MiB，不能同时提供参数值。覆盖已有变量仍需确认或显式指定 `--yes`。

终端中的 `set KEY` 会隐藏输入值；脚本显式传入值。`--yes` 确认覆盖已有值或新增环境名。`list` 只返回变量名。CLI 不提供明文读取命令；通过 `one exec` 将值注入子进程。`-o json` 或 `-o yaml` 输出结构化结果，保留 `one-cli/env-set/v1`、`one-cli/env-list/v1` 协议。

`-p / --project` 接受项目名或工作区相对路径。不传时按当前目录推断项目；在工作区根目录操作共享变量，交互式 `set` 会提供作用域选择。

## 环境与目录

`env.infisical.environments` 声明远程环境 slug。默认固定使用 `dev`，通过 `--env` 选择其他已声明环境。新绑定默认声明 `dev`、`staging`、`prod`，对应 Infisical 的环境标识，而不是界面展示名称。声明名称不会自动创建远程环境。

`set` 经确认且远程写入成功后，可以在本地登记新环境名；对应环境必须已存在于 Infisical。`list` 对未声明名称返回 `ENV_UNKNOWN_ENVIRONMENT`。`list`、读取与 `unset` 只使用已有绑定，未绑定时提示先保存第一个变量。

共享目录固定为 `/`。项目目录从 `path` 推导：`services/api` 对应 `/services/api`。变量依次合并 `/`、`/services` 和 `/services/api`，更具体的目录优先。并行任务各自接收所属项目的变量，不提供项目级目录、继承、变量名清单或停用设置。

## 带变量执行

```bash
one exec -p api --env dev -- go run ./cmd/server
one run dev --env dev
```

配置 `[env.infisical]` 后，`one run` 和 `one exec` 按项目拉取变量，覆盖同名 shell 变量。未绑定时继承 shell 环境；认证或拉取失败会停止执行。One 不输出拉取到的变量值；子进程的 stdout、stderr、ANSI 色彩和格式保持原样。

## 全局共享凭据

共享凭据独立于工作区。通过 `one env bind --global` 选择存储项目，使用 `one env list --global --env dev --path /` 查看名称，使用 `one exec --global --env dev --path /folder -- command` 注入明确作用域。详见[登录与共享凭据](/zh/docs/login/)。

## 常见错误

| 错误码 | 恢复方法 |
|---|---|
| `INFISICAL_NOT_CONFIGURED` | 登录后保存第一个变量，One 会自动创建并绑定存储项目 |
| `INFISICAL_AUTH_MISSING` / `INFISICAL_AUTH_FAILED` | 执行 `one login` 并检查绑定项目的访问权限 |
| `INFISICAL_PROJECT_NAME_TAKEN` | 在 Dashboard 选择已有项目，或使用不同的工作区名称 |
| `INFISICAL_PROJECT_CREATE_FORBIDDEN` | 在 Dashboard 选择已有且可访问的项目 |
| `ENV_KEY_NOT_FOUND` | 检查变量名、环境和目录 |
| `ENV_INVALID_KEY` | 变量名须匹配 `^[A-Za-z_][A-Za-z0-9_]*$` |
| `ENV_SET_OVERWRITE_REQUIRED` | 确认替换后添加 `--yes` |
| `ENV_UNKNOWN_ENVIRONMENT` | 通过 `set` 登记新环境，或选择已有名称 |

配置结构参阅 [Manifest v2](/zh/docs/manifest/)，初次配置参阅[操作教程](/zh/tutorials/env-vars/)。

## 同名工作区

远程项目默认使用工作区名称；名称冲突时自动添加短后缀，并显示实际名称。不同工作区不会仅因同名而共用变量，写入目标由 `env.infisical.projectId` 确定。复制或克隆包含绑定的配置会继续使用同一远程项目。创建后变量保存失败时，绑定会保留，重试复用该项目。
