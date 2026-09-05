---
title: one create
description: 起一个新的 one 工作区根骨架。
---

`one create` 只创建空工作区：不问项目、不问部署，也不修改本机 AI 工具配置。需要项目时使用 `one add`。

## 用法

```bash
one create [dir] [options]
```

## 参数

| 参数 | 说明 |
|---|---|
| `dir` | 目标目录（位置参数）。传 `.` 在当前目录就地创建（用 `basename(cwd)` 当名字）；目标目录必须不存在或为空 |
| `-n, --name <name>` | 工作区名称（默认 `basename(dir)`） |
| `-y, --yes` | 非交互模式：使用默认值；必须显式传 `dir` |
| `--env-provider <dotenv\|infisical>` | env 后端选择；默认 `dotenv`，需要 Infisical 时显式传 `infisical` |
| `-o, --output <fmt>` | `json` / `yaml` / `text`（默认按 TTY 检测） |

## 交互模式

直接运行 `one create` 会进入终端交互式询问：

1. 目标目录（例如 `./my-app`，也可以填 `.` 表示当前目录）
2. 工作区名称（可留空；留空时使用目标目录的 basename）

`one create` 不会在交互模式里询问 deploy / container，也不会再询问是否切换 Infisical。默认 env 后端是 `env/dotenv`；如果要在创建时使用 Infisical，请显式传 `--env-provider infisical`。

脚本、CI、agent 场景用非交互写法：

```bash
one create my-app --yes
one create my-app --yes --env-provider infisical
```

## 默认能力

`one create` 不再让用户手动多选插件。改为：

**工作区默认（无交互式询问）**

| 能力 | 默认值 | 行为 |
|---|---|---|
| 环境变量 | 本地 `.env` 文件 | 可通过 `--env-provider infisical` 或后续 `one env switch infisical` 切换到 Infisical |
| 本地开发 | `one dev` | 通过内置进程管理器运行各项目的开发命令 |

持续集成默认不配置。创建工作区不会写入 `.github/workflows/`；添加项目后如有
需要，再显式运行 `one ci enable <project>`。

**部署决策延后**

create 不写部署或镜像配置，普通 `one add` 也保持未配置。第一次运行
`one deploy <project>` 时，才选择兼容的部署目标和本机连接。

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
  "ci_enabled": false,
  "dev_enabled": true
}
```

`secrets_backend` 是 env 域 backend 名（`dotenv` / `infisical`）；`ci_enabled`
为兼容 wire format 继续保留，默认是 `false`，`dev_enabled` 是 `true`。部署配置会在首次部署时写入。


## 示例

### 交互（人类）

```bash
one create
# 引导填写目标目录 + 可选工作区名称
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
| `PROJECT_NAME_REQUIRED` | 非交互模式必须把工作区目录作为位置参数传入 |
| `BACKEND_ID_UNKNOWN` | `--env-provider` 值无效（合法值：dotenv / infisical） |
| `WORKSPACE_NESTED_FORBIDDEN` | 拒绝在已有 workspace 里再 create；换目录或用 `one add` |

完整码表：[错误码大全](/zh/docs/error-codes/)。
