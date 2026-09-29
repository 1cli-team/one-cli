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
| `dir` | 目标目录（位置参数）。传 `.` 在当前目录就地创建，自动使用当前文件夹名；支持新目录、空目录或只有有效 Git 元数据的仓库 |
| `-n, --name <name>` | 高级选项：覆盖自动使用的文件夹名称 |
| `-y, --yes` | 非交互模式：使用默认值；必须显式传 `dir` |
| `-o, --output <fmt>` | `json` / `yaml` / `text`（默认按 TTY 检测） |

## 交互模式

直接运行 `one create` 只询问目标目录，例如 `./my-app`，也可以填 `.` 表示当前目录。工作区名称自动使用最后一级文件夹名，不再单独询问；名称不合要求时会直接在目录输入处提示。


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

创建工作区和添加项目时会准备 mise，并自动信任完全由 One 生成的 `mise.toml`，进入新目录无需再单独执行信任命令。已有自定义配置保留 mise 原有的信任检查。本机没有兼容版本时，One 可能下载托管的 mise 程序；项目工具和依赖仍按需安装。信任失败会保留生成文件并给出恢复命令。工具版本与已有 workspace 的启用方式见 [`one init mise`](/zh/docs/login/#本机偏好与工作区工具)。

空工作区先保持语言无关：首次添加 Go 模块时创建根 `go.work` 并登记该模块；首次添加 JS/TS 项目时创建根 `package.json` 和 `pnpm-workspace.yaml`。后续项目增量加入，两套配置可以共存。Git hooks 从创建工作区时就由 hk 提供，纯 Go 工作区不生成 Node 配置；JS 工作区也不再依赖 Husky 或 commitlint。工作区不默认安装版本管理工具或生成 Changesets 配置，发布流程由项目按需配置。

提交前默认只检查暂存内容，使用 `one hk fix` 显式修复。用法与自定义方式见 [`one hk`](/zh/docs/hk/)。Git 未安装或已有 hooks 配置发生冲突时，工作区仍会创建，输出的 `warnings` 会提示后续执行 `one init hooks`。

持续集成默认不配置。创建工作区不会写入 `.github/workflows/`；添加项目后如有
需要，可以手动配置调用 `one run ci` 的工作流。

## Infisical 绑定

创建时不访问 Infisical，也不写入 `env` 绑定。需要托管变量时先通过 `one login` 登录，首次保存变量（`one env set` 或 Dashboard 保存）时才初始化绑定。查看、刷新、读取和删除不会创建远程项目。重名时会自动追加短后缀，并显示实际创建的远程项目名称；本地工作区名称保持原样。后续写入按保存的远程项目 ID 定位。未绑定时执行命令使用 shell 环境。

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

`one create` 创建空工作区，因此 `package_manager` 为空字符串；使用 `one add` 添加 Node 项目后才会配置包管理器。

`secrets_backend` 表示支持的来源（`infisical`），不代表已经绑定；`dev_enabled` 是 `true`。


## 示例

### 交互（人类）

```bash
one create
# 只需填写目标目录
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

### 在已有空 Git 仓库中创建

克隆空仓库或执行 `git init` 后，运行 `one create .` 即可。One 会保留分支、远程地址、历史和 Git 配置。已有自定义 hooks 时保留原内容并提示；linked worktree 的共享 hooks 跳过自动安装。含 README、源码等文件或未提交删除的仓库仍会提示冲突。

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
| `EXISTING_TARGET_NOT_EMPTY` | 根据提示检查已有文件，选择空目录或空 Git 仓库 |
| `INVALID_NAME` | 名字必须匹配 `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`；空格替换为 `-` |
| `PROJECT_NAME_REQUIRED` | 非交互模式必须把工作区目录作为位置参数传入 |
| `WORKSPACE_NESTED_FORBIDDEN` | 拒绝在已有 workspace 里再 create；换目录或用 `one add` |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## Agent 指引


新工作区只生成一份单语言 `AGENTS.md`，跟随 One CLI 当前生效语言。使用 `one locale zh-CN` 或 `one locale en-US` 设置偏好；`auto` 跟随终端语言，无法识别时使用英文。后续切换语言或运行 `one add` 都不会重写已有文件。

源码模板直接使用 Markdown，位于 `packages/cli/internal/modules/creation/templates/` 下的 `AGENTS.zh-CN.md` 和 `AGENTS.en-US.md`。修改后重新构建 CLI，即可应用到以后创建的工作区。

指引要求 Agent 通过 `one run` 执行测试及项目命令，将复杂 TypeScript 脚本放在 `scripts/*.mts` 并登记为任务，统一通过 One 管理环境变量。Docker 镜像上传账号等发布、运维凭据放在全局变量中；缺少时提示用户到 Dashboard 的“共享凭据”页面配置，然后由任务内部通过 `one exec --global` 注入。
