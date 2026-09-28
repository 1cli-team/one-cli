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
| `-o, --output <fmt>` | `json` / `yaml` / `text`（默认按 TTY 检测） |

## 交互模式

直接运行 `one create` 会进入终端交互式询问：

1. 目标目录（例如 `./my-app`，也可以填 `.` 表示当前目录）
2. 工作区名称（可留空；留空时使用目标目录的 basename）


脚本、CI、agent 场景用非交互写法：

```bash
one create my-app --yes
```

## 默认能力

`one create` 不再让用户手动多选插件。改为：

**工作区默认（无交互式询问）**

| 能力 | 默认值 | 行为 |
|---|---|---|
| 环境变量 | 未绑定 | 需要时绑定 Infisical，否则继承 shell 环境 |
| 本地开发 | `one dev` | 通过 mise 运行开发任务图 |
| Git 检查 | hk | 生成 `.config/hk.pkl` 并安装本地 Git 启动器 |
| 工具环境 | mise | 自动生成根 `mise.toml`；后续 `one add` 在同一文件登记项目任务 |
| Git 检查 | hk | 创建共享检查配置并安装本地提交钩子；后续 `one add` 增量加入语言检查 |

创建工作区和添加项目时会准备 mise，并自动信任完全由 One 生成的 `mise.toml`，进入新目录无需再单独执行信任命令。已有自定义配置保留 mise 原有的信任检查。本机没有兼容版本时，One 可能下载托管的 mise 程序；项目工具和依赖仍按需安装。信任失败会保留生成文件并给出恢复命令。工具版本与已有 workspace 的启用方式见 [`one init mise`](/zh/docs/login/#mise-工作区工具配置)。

空工作区先保持语言无关：首次添加 Go 模块时创建根 `go.work` 并登记该模块；首次添加 JS/TS 项目时创建根 `package.json` 和 `pnpm-workspace.yaml`。后续项目增量加入，两套配置可以共存。Git hooks 从创建工作区时就由 hk 提供，纯 Go 工作区不生成 Node 配置；JS 工作区也不再依赖 Husky 或 commitlint。工作区不默认安装版本管理工具或生成 Changesets 配置，发布流程由项目按需配置。

提交前默认只检查暂存内容，使用 `one hk fix` 显式修复。用法与自定义方式见 [`one hk`](/zh/docs/hk/)。Git 未安装或已有 hooks 配置发生冲突时，工作区仍会创建，输出的 `warnings` 会提示后续执行 `one init hooks`。

持续集成默认不配置。创建工作区不会写入 `.github/workflows/`；添加项目后如有
需要，可以手动配置调用 `one run ci` 的工作流。

## Infisical 绑定

创建时不访问 Infisical，也不写入 `env` 绑定。需要托管变量时先通过 `one login` 登录，首次 `one env set`、`get` 或 `list` 会初始化绑定；也可在 Dashboard 选择已有项目。未绑定时执行命令使用 shell 环境。

## 输出

```json
{
  "schema": "one-cli/create/v2",
  "project_name": "my-app",
  "created_path": "/abs/path/my-app",
  "created_in_place": false,
  "package_manager": "",
  "secrets_backend": "infisical",
  "dev_enabled": true
}
```

`package_manager` 在空工作区或纯 Go 工作区中为空字符串；含 Node 项目的 preset 会返回实际包管理器名称。

`secrets_backend` 表示支持的来源（`infisical`），不代表已经绑定；`dev_enabled` 是 `true`。


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
one create my-app --yes
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
one dev -p api
```

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `EXISTING_TARGET_NOT_EMPTY` | 换一个空目录，或手动删除目标后重试 |
| `INVALID_NAME` | 名字必须匹配 `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`；空格替换为 `-` |
| `PROJECT_NAME_REQUIRED` | 非交互模式必须把工作区目录作为位置参数传入 |
| `WORKSPACE_NESTED_FORBIDDEN` | 拒绝在已有 workspace 里再 create；换目录或用 `one add` |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## Agent 指引


新工作区生成中英文 `AGENTS.md`，说明 `one run`、`one exec`、pnpm 和 Task 的分工。后续添加项目保留团队修改。
