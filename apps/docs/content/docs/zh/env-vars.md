---
title: one env
description: 使用 set、get、list 管理 Infisical 变量，并在命令执行时直接注入。
---

Infisical 是 One CLI 唯一管理的环境变量来源。变量在执行命令时拉取并注入子进程，One CLI 不读取、写入或导出 `.env` 文件。

## 绑定工作区

新工作区没有 Infisical 绑定，无需登录即可运行。先执行 `one login`，首次使用 `one env set`、`one env get` 或 `one env list` 时会初始化绑定。也可以在 Dashboard 的工作区设置中选择已有 Infisical 项目。

绑定保存在 `one.manifest.json` 顶层的 `env`：

```json
{
  "env": {
    "siteUrl": "https://app.infisical.com",
    "projectId": "your-project-id",
    "projectName": "my-workspace",
    "rootPath": "/"
  },
  "environments": {
    "names": ["dev", "preview", "prod"],
    "default": "dev"
  }
}
```

Manifest 只记录绑定信息、目录路径和变量名。值保存在 Infisical，登录使用系统 keyring 中的浏览器会话。

## 命令

```bash
one env
one env set DATABASE_URL -p api
one env set API_URL=https://example.com -p web --env dev --yes
one env get API_URL -p web --env dev --reveal
one env list -p web --env dev
```

终端中的 `set KEY` 会隐藏输入值；脚本显式传入值。`--yes` 确认覆盖已有值或新增环境名。`get` 必须带 `--reveal`；`list` 只返回变量名。`-o json` 或 `-o yaml` 输出结构化结果，保留 `one-cli/env-set/v1`、`one-cli/env-get/v1`、`one-cli/env-list/v1` 协议。

`-p / --project` 接受项目名或工作区相对路径。不传时按当前目录推断项目；在工作区根目录操作共享变量，交互式 `set` 会提供作用域选择。

## 环境与目录

`environments.names` 声明可用环境，默认使用 `environments.default`，缺省时取第一个名称。新工作区声明 `dev`、`preview`、`prod`，默认 `dev`。`--env` 可覆盖选择。

`set` 可以经确认登记新环境。`get` 和 `list` 对未声明名称返回 `ENV_UNKNOWN_ENVIRONMENT`；这些命令仍可能初始化尚未存在的 Infisical 绑定。

项目默认使用 `relativeDir` 对应的 Infisical 目录，也可通过 `projects[].env` 覆盖：

```json
{
  "name": "api",
  "relativeDir": "services/api",
  "env": {
    "path": "/teams/payments/api",
    "inherits": true,
    "keys": ["DATABASE_URL"]
  }
}
```

默认启用继承，变量按根目录、祖先目录、项目目录合并，越靠近项目的值优先。`inherits: false` 只读项目目录；`disabled: true` 停用该项目的 Infisical 注入。

## 带变量执行

```bash
one exec -p api --env dev -- go run ./cmd/server
one run dev --env dev
```

配置 `env` 后，生成的任务适配器与 `one exec` 会拉取 Infisical 变量，并覆盖同名 shell 变量。未绑定或项目停用注入时，继承 shell 环境。认证或拉取失败会停止执行，不回退到本地文件。自定义 mise 任务使用 mise 的环境。

## 全局共享凭据

共享凭据独立于工作区。通过 `one env bind --global` 选择存储项目，使用 `one env list --global --env dev --path /` 查看名称，使用 `one exec --global --env dev --path /folder -- command` 注入明确作用域。详见[登录与共享凭据](/zh/docs/login/)。

## 常见错误

| 错误码 | 恢复方法 |
|---|---|
| `INFISICAL_NOT_CONFIGURED` | 在 Dashboard 绑定 Infisical 项目，或登录后通过 env 命令初始化 |
| `INFISICAL_AUTH_MISSING` / `INFISICAL_AUTH_FAILED` | 执行 `one login` 并检查绑定项目的访问权限 |
| `INFISICAL_PROJECT_NAME_TAKEN` | 设置不同的 `env.projectName`，或显式绑定已有项目 |
| `INFISICAL_PROJECT_CREATE_FORBIDDEN` | 在 Dashboard 选择已有且可访问的项目 |
| `ENV_KEY_NOT_FOUND` | 检查变量名、环境和目录 |
| `ENV_INVALID_KEY` | 变量名须匹配 `^[A-Za-z_][A-Za-z0-9_]*$` |
| `ENV_SET_OVERWRITE_REQUIRED` | 确认替换后添加 `--yes` |
| `ENV_UNKNOWN_ENVIRONMENT` | 通过 `set` 登记新环境，或选择已有名称 |

旧工作区参阅 [Manifest 迁移](/zh/docs/manifest/)，初次配置参阅[操作教程](/zh/tutorials/env-vars/)。
