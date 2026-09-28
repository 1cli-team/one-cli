---
title: one dev
description: 启动全部可开发项目，或只启动一个项目。
---

`one dev` 从 manifest 读取每个项目的开发命令，并用 One CLI 内置 supervisor 运行。

启用 mise 的 workspace 会自动在每个项目的 mise 工具环境中运行开发命令，仍然使用 `one dev` / `one dev web`，无需增加 runtime 参数。日志前缀、项目选择和整组服务的停止行为继续由原有 supervisor 负责。mise 的安装与旧项目启用见 [`one init mise`](/zh/docs/login/#mise-工作区工具配置)。

## 用法

```bash
one dev [project] [--dry-run]
```

## 参数

| 参数 | 说明 |
|---|---|
| 位置参数 `project` | 只启动一个项目；支持 manifest 里的 `name` 或 `relativeDir` |
| `-p, --project <name|path>` | 为旧脚本和 CI 保留的选择参数 |
| `--dry-run` | 只打印将调用的 supervisor 命令，不安装工具或依赖、不访问网络、不写文件 |
| `-o, --output <fmt>` | `json` / `yaml` / `text`（默认按 TTY 检测） |

## 依赖准备

`one dev` 在启动服务前自动准备所选项目的工具与应用依赖，交互和非交互调用行为一致。

- Node：在工作区根目录统一准备依赖。使用 pnpm 时，先通过 pnpm 自身检查工作区的安装状态；手动 `pnpm install` 后，只要依赖与当前工作区一致，就直接复用。需要安装时执行 `pnpm install --no-frozen-lockfile`，自动同步新增项目或依赖变更。pnpm 10.14 之前的版本沿用 One 的安装缓存。其他包管理器仍使用原有锁文件策略。
- `one build` 保留现有的严格安装策略：已有 pnpm 依赖锁文件时使用 `--frozen-lockfile`。构建发现锁文件过期后，需要先安装并审阅锁文件变更。
- Go：独立模块下载固定构建列表并补充 `go.sum`；存在 `go.work` 时由 Go 按实际包依赖解析本地成员和外部依赖，按需维护 `go.work.sum`。准备过程不自动运行 `go mod tidy` 或 `go work sync`。
- 所有准备成功后才启动 supervisor。失败保留底层错误和下载缓存，可修复后重试原命令；取消时停止准备进程。

`go.work.sum` 不替代各模块发布所需的 `go.sum`。模块声明需要修复时，可显式运行 `one exec api -- go mod tidy`。`one exec` 保持直接执行命令，不自动安装应用依赖。

## 运行方式

内置 supervisor 默认启动全部可开发项目，也可用位置参数只启动一个。

```bash
one dev
one dev web
one dev apps/web --dry-run
```

## 日志滚动与窗口缩放

在 Unix 交互式终端中运行多个任务时会进入 TUI。单项目也可通过 `one dev web --ui=tui` 打开。有限构建任务使用 mise 流式输出。

| 操作 | 行为 |
|---|---|
| `↑` / `↓` | 选择项目 |
| 日志区域内的鼠标滚轮 | 滚动当前项目的输出 |
| `PgUp` / `PgDn` | 翻阅日志；全屏子程序的输入模式中交给子程序处理 |
| `Shift+PgUp` / `Shift+PgDn` | 翻阅历史，输入模式下也可使用 |
| `f` / `End` | 在导航模式恢复跟随输出 |
| `Shift+End` | 在导航或输入模式恢复跟随输出 |
| `Enter` / `Ctrl+]` | 进入子进程输入／返回导航 |
| `h` | 隐藏或显示项目列表 |
| `?` | 查看全部快捷键，包括重启和停止 |

每个项目独立保存滚动位置。查看历史时，新输出不会推动当前视图，直到这些历史行超出保留上限。向子进程键入或粘贴内容会返回实时画面。调整终端尺寸会重新换行保留的日志，并同步所有子进程的终端尺寸；全屏子程序自行重绘界面。

每个项目最多显示 3000 行历史，另保留最多 1 MiB 的输出供重新换行使用。达到上限后会丢弃较早的内容。

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `RUN_COMMAND_NOT_FOUND` | 检查原生工具安装或 mise 配置 |
| `ONE_CLI_ERROR` | 按底层依赖错误修复网络、锁文件或模块声明后重试 |
| `SUBPROJECT_NOT_FOUND` | 使用项目 `name` 或 `relativeDir` |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [本地开发编排](/zh/tutorials/dev-local/) — 内置 supervisor 的完整流程
- [`one exec`](/zh/docs/exec/) — 只给单条命令注入环境变量
- [manifest](/zh/docs/manifest/) — 项目列表的来源

`one run dev` 使用相同的 supervisor。开发启动前，本地上游包的有限构建由 mise 调度执行。
