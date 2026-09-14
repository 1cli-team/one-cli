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
| 工具环境 | mise | 自动生成根 `.mise/conf.d/one.toml`；后续 `one add` 自动生成项目配置 |
| Git 检查 | hk | 创建共享检查配置并安装本地提交钩子；后续 `one add` 增量加入语言检查 |

创建和添加项目只生成配置，不下载工具。首次运行时 One 从自身解压内置 mise，用户无需单独安装或下载 mise；正常命令保持不变。工具版本与已有 workspace 的启用方式见 [`one configure mise`](/zh/docs/configure/#mise-工作区工具配置)。

空工作区先保持语言无关：首次添加 Go 模块时创建根 `go.work` 并登记该模块；首次添加 JS/TS 项目时创建根 `package.json` 和 `pnpm-workspace.yaml`。后续项目增量加入，两套配置可以共存。Git hooks 从创建工作区时就由 hk 提供，纯 Go 工作区不生成 Node 配置；JS 工作区也不再依赖 Husky 或 commitlint。工作区不默认安装版本管理工具或生成 Changesets 配置，发布流程由项目按需配置。

提交前默认只检查暂存内容，使用 `one hk fix` 显式修复。用法与自定义方式见 [`one hk`](/zh/docs/hk/)。Git 未安装或已有 hooks 配置发生冲突时，工作区仍会创建，输出的 `warnings` 会提示后续执行 `one configure hooks`。

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
  "package_manager": "",
  "secrets_backend": "dotenv",
  "ci_enabled": false,
  "dev_enabled": true
}
```

`package_manager` 在空工作区或纯 Go 工作区中为空字符串；含 Node 项目的 preset 会返回实际包管理器名称。

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
one dev api
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
