---
title: one create
description: 起一个新的 one 工作区根骨架。
---

`one create` 起工作区骨架并安装 skills。默认模式只创建工作区，不加首个项目；需要项目时继续使用 `one add` 追加模板。

## 用法

```bash
one create [dir] [options]
```

## 参数

| 参数 | 说明 |
|---|---|
| `dir` | 目标目录（位置参数）。传 `.` 在当前目录就地创建（用 `basename(cwd)` 当名字）；目标目录必须不存在或为空 |
| `-n, --name <name>` | 项目名（默认 `basename(dir)`） |
| `-y, --yes` | 非交互模式：使用默认值；必须显式传 `dir` |
| `--env-provider <dotenv\|infisical>` | env 后端选择；默认 `dotenv`，需要 Infisical 时显式传 `infisical` |
| `-o, --output <fmt>` | `json` / `yaml` / `text`（默认按 TTY 检测） |

## 交互模式

直接运行 `one create` 会进入终端交互式询问：

1. 目标目录（例如 `./my-app`，也可以填 `.` 表示当前目录）
2. 项目名（可留空；留空时使用目标目录的 basename）

`one create` 不会在交互模式里询问 deploy / container，也不会再询问是否切换 Infisical。默认 env 后端是 `env/dotenv`；如果要在创建时使用 Infisical，请显式传 `--env-provider infisical`。

脚本、CI、agent 场景用非交互写法：

```bash
one create my-app --yes
one create my-app --yes --env-provider infisical
```

## 自动启用的插件

`one create` 不再让用户手动多选插件。改为：

**工作区默认（无交互式询问，自动启用）**

| Domain | 默认插件 | 行为 |
|---|---|---|
| `env` | `env/dotenv` | 读写 `.env` 系列文件（可通过 `--env-provider infisical` 或后续 `one env switch infisical` 切换到 Infisical） |
| `ci` | `ci/github-actions` | 写 `.github/workflows/` |
| `dev` | `dev/process` | 把每个项目的 dev 命令写到 `projects[].domains.dev.command`，`one dev` 用内置 supervisor 跑 |

**Deploy / Container：模板驱动，不在 create 时落盘**

由 `one add <template>` 按模板的 `defaults` 自动启用：

| 模板 | 自动启用的 backend |
|---|---|
| go-api / nestjs-api / nextjs-app | `container=docker` + `deploy=kustomize` |
| react-spa / astro-site / starlight-docs | `deploy=aws-s3` |
| expo-mobile / ts-library / go-lib / electron-app | 不参与 deploy / container |

## --env-provider 语义

`--env-provider <dotenv|infisical>` 显式指定 env 后端：

```bash
one create my-app -y --env-provider infisical
```

使用 Infisical 前建议先配置机器级 profile：

```bash
one configure add env/infisical --profile work \
  --client-id $INFISICAL_UNIVERSAL_AUTH_CLIENT_ID \
  --client-secret $INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET \
  --use
```

`one create --env-provider infisical` 会尽量自动绑定 / 创建 Infisical project；如果当时 profile、网络或权限没准备好，工作区仍会创建成功，首次 `one env set/get/list/pull` 会再尝试一次 lazy auto-bind。

## 输出

```json
{
  "schema": "one-cli/create/v2",
  "project_name": "my-app",
  "created_path": "/abs/path/my-app",
  "created_in_place": false,
  "package_manager": "pnpm",
  "secrets_backend": "dotenv",
  "ci_enabled": true,
  "dev_enabled": true,
  "skills": {
    "status": "completed",
    "installed_to": ["/Users/example/.claude/skills"],
    "skill_count": 2
  }
}
```

`secrets_backend` 是 env 域 backend 名（`dotenv` / `infisical`）；`ci_enabled` /
`dev_enabled` 永远为 `true`（CI workflow 始终同步，dev 命令始终随 `one add` 写入 manifest；保留字段是为了 wire format 兼容）。container / deploy 的 backend 由模板驱动，写在 `projects[].domains.{container,deploy}` 中。

`skills.status` 可能是：

- `"completed"`：已安装 skills。
- `"failed"`：工作区仍然成功创建，但 skills 没装上，跑 `one skills install` 重试。

## 示例

### 交互（人类）

```bash
one create
# 引导填写目标目录 + 可选项目名
```

### 非交互（CI / 脚本）

```bash
one create my-app --yes
```

### 切换到 Infisical 作为 secrets 后端

```bash
one create my-app --yes --env-provider infisical
```

### 在当前目录就地创建

```bash
mkdir my-app && cd my-app
one create . --yes
```

### 起骨架 + 加首个项目

```bash
one create my-app --yes
cd my-app
one add nestjs-api --name api --yes
pnpm install
```

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `EXISTING_TARGET_NOT_EMPTY` | 换一个空目录，或手动删除目标后重试 |
| `INVALID_NAME` | 名字必须匹配 `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`；空格替换为 `-` |
| `PROJECT_NAME_REQUIRED` | 非交互模式必须传位置参数 |
| `BACKEND_ID_UNKNOWN` | `--env-provider` 值无效（合法值：dotenv / infisical） |
| `WORKSPACE_NESTED_FORBIDDEN` | 拒绝在已有 workspace 里再 create；换目录或用 `one add` |
| `SKILLS_INSTALL_FAILED` | 检查 `~/.claude/skills` 写入权限；或手工跑 `one skills install` |

完整码表：[错误码大全](/zh/docs/error-codes/)。
