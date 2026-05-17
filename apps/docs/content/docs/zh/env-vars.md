---
title: one env
description: 多环境环境变量 — set / get / list / pull 子命令的完整参考。
---

`one env` 管 monorepo 多环境变量。两种后端：

- **dotenv**（默认）：本地文件系统。每个项目持有 `.env` + `.env.<env>` + `.env.local` + `.env.<env>.local` 的 overlay。
- **infisical**：[Infisical](https://infisical.com/)。同一个工作区共享一个 Infisical project，环境（dev / staging / prod / ...）是 project 内分区。

完整工作流和心智模型见 [环境变量指南](/zh/tutorials/env-vars/)。

## 环境模型

`manifest.environments.names` 是工作区的环境列表（`one create` 默认 `["dev","staging","prod"]`），`manifest.environments.default` 是不传 `--env` 时的回退值（默认 `dev`）。

`--env` 解析链：

```text
--env flag → manifest.environments.default → environments.names[0]
```

`one env set FOO bar --env qa` 可以创建新环境。TTY 模式会确认，非 TTY / `--yes` 会直接追加到 `manifest.environments.names`。`get` / `list` / `pull` 是只读语义，未知环境直接报 `ENV_UNKNOWN_ENVIRONMENT`。

## 用法

```bash
one env set  <KEY[=VALUE]> [VALUE] [--env <env>] [-p <name|path>] [--yes]
one env get  <KEY>                 [--env <env>] [-p <name|path>]
one env list                       [--env <env>] [-p <name|path>]
one env pull                       [--env <env>] [-p <name|path>] [--force] [--dry-run]
```

`-p / --project` 接受 manifest 里的项目名或相对路径：

```bash
one env set FOO=bar -p web
one env set FOO=bar -p apps/web
one env set FOO=bar              # cwd 在项目里时自动识别
one env pull -p api              # 只拉 api 项目
one env pull --env staging       # 拉所有项目的 staging 环境变量
```

通用输出 flag 是 `-o / --output`，取值 `json` / `yaml` / `text`。

> 当前没有 `one env init` 子命令。Infisical project binding 由 `one create --env-provider infisical` 自动尝试；如果 create 时 profile、网络或权限还没准备好，首次 `set/get/list/pull` 会再尝试 lazy auto-bind。

机器级 Infisical 凭据通过 [`one configure add env/infisical`](/zh/docs/cli-overview/#one-configure) 配，不进入 manifest。

## 交互模式

`one env` 没有完整向导，但 `one env set` 在 TTY 下有两类确认：

- 写入一个不在 `manifest.environments.names` 里的新环境时，会确认是否把该环境加入 manifest。
- 覆盖已有不同值时，会确认是否覆盖。

脚本、CI、agent 用 `--yes` 跳过确认；`get` / `list` / `pull` 是显式参数命令，不会打开交互式向导。

## dotenv overlay

dotenv 后端读取顺序：

```text
<project>/.env
<project>/.env.<env>
<project>/.env.local
<project>/.env.<env>.local
```

后面的文件覆盖前面的文件。`one env set` 写 `.env.<env>`；`.local` 文件只读，由开发者自己维护。

`one create` 默认写入 `.gitignore`：

```text
.env
.env.*
!.env.example
```

## set

写入单个 key。两种参数形式都支持：

```bash
one env set DATABASE_URL "postgres://localhost/dev" --env dev -p api
one env set JWT_SECRET=dev-only-secret --env dev -p api --yes
```

输出 schema：`one-cli/env-set/v1`

```json
{
  "schema": "one-cli/env-set/v1",
  "env": "dev",
  "key": "DATABASE_URL",
  "action": "created"
}
```

`action` 可能是 `created` / `updated` / `unchanged`。已有不同值时需要 `--yes` 确认覆盖。

## get

读取单个 key：

```bash
one env get DATABASE_URL --env dev -p api
DB_URL=$(one env get DATABASE_URL --env dev -p api -o json | jq -r .value)
```

输出 schema：`one-cli/env-get/v1`

```json
{
  "schema": "one-cli/env-get/v1",
  "env": "dev",
  "key": "DATABASE_URL",
  "value": "postgres://..."
}
```

## list

列出 key 名，不显示值：

```bash
one env list --env dev -p api
```

输出 schema：`one-cli/env-list/v1`

```json
{
  "schema": "one-cli/env-list/v1",
  "env": "dev",
  "keys": ["DATABASE_URL", "JWT_SECRET"]
}
```

## pull

把 Infisical 的环境变量拉到本地 `.env`：

```bash
one env pull --env dev
one env pull --env dev -p api --dry-run
one env pull --env dev --force
```

默认不传 `-p` 时遍历 `manifest.projects[]`。每个项目按 `projects[].domains.env.path` 或 `relativeDir` 映射到 Infisical folder，并把 root → ancestors → self 的继承链合并后写入该项目目录的 `.env`。

输出 schema：`one-cli/env-pull/v1`

```json
{
  "schema": "one-cli/env-pull/v1",
  "env": "dev",
  "dry_run": false,
  "written_count": 1,
  "skipped_count": 0,
  "per_subproject": [
    {
      "name": "api",
      "relative_dir": "services/api",
      "infisical_path": "/services/api",
      "env_file_path": "/abs/.../services/api/.env",
      "status": "written",
      "keys_written": ["DATABASE_URL", "JWT_SECRET"]
    }
  ]
}
```

## manifest 配置

Workspace 级 env 后端写在 `one.manifest.json#domains.env`，环境列表写在顶层 `environments`：

```json
{
  "environments": {
    "names": ["dev", "staging", "prod"],
    "default": "dev"
  },
  "domains": {
    "env": {
      "kind": "infisical",
      "profile": "work",
      "config": {
        "projectId": "...",
        "projectName": "my-workspace",
        "rootPath": "/"
      }
    }
  }
}
```

项目级 path 覆盖写在 `projects[].domains.env`：

```json
{
  "projects": [
    {
      "name": "charge",
      "relativeDir": "services/charge",
      "domains": {
        "env": {
          "path": "/teams/payments/charge",
          "inherits": true
        }
      }
    }
  ]
}
```

值本身永远不进 manifest；manifest 只记录 backend、profile、folder path 和 key 名。

## 凭据安全

`one configure add env/infisical` 写 `~/.config/one/config.json` 与 `~/.config/one/credentials.json`（mode 0600）。不要把 client id / client secret 写进仓库；CI 用 secret store 注入。

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `INFISICAL_NOT_CONFIGURED` | 确认工作区用了 `--env-provider infisical`，并有 default `env/infisical` profile |
| `INFISICAL_AUTH_MISSING` | 重新跑 `one configure add env/infisical --profile work ... --use` |
| `INFISICAL_AUTH_FAILED` | Infisical 后台重新生成 client secret |
| `INFISICAL_PROJECT_NAME_TAKEN` | 修改 `domains.env.config.projectName` 后重跑 env 命令触发 lazy bind |
| `INFISICAL_PROJECT_CREATE_FORBIDDEN` | 给 machine identity 加 admin 角色，或手动建项目后填 `domains.env.config.projectId` |
| `ENV_PULL_CONFLICT` | 本地 `.env` 已存在且不同；确认后加 `--force` |
| `ENV_KEY_NOT_FOUND` | 核对 path / env / 拼写 |
| `ENV_INVALID_KEY` | KEY 必须匹配 `^[A-Za-z_][A-Za-z0-9_]*$` |
| `ENV_SET_OVERWRITE_REQUIRED` | 已存在不同值；加 `--yes` 确认 |
| `ENV_UNKNOWN_ENVIRONMENT` | `set` 可创建；读命令需要先在 `manifest.environments.names` 声明 |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [环境变量指南](/zh/tutorials/env-vars/) — 心智模型 + 完整工作流
- [`one create`](/zh/docs/create/) — 起骨架时用 `--env-provider infisical` 接 Infisical
