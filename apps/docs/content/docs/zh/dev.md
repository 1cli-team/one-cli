---
title: one dev
description: 通过 mise 运行开发任务及其前置依赖。
---

`one dev` 等同于 `one run dev`，执行 mise 中最终生效的任务图，包括你在 `mise.toml` 中定义的覆盖。

```sh
one dev
one dev -p web -p api
one dev -p apps/web --dry-run -o json
one dev -p web --ui raw
one dev -p web -- --port 4300
```

不传 `-p` 时运行根目录的 `dev` 任务，生成的聚合任务包含有 dev 任务的项目。重复 `-p` 可按名称或相对路径选择项目。任务参数放在 `--` 后；传开发参数时选择一个项目。

## 命令与覆盖

生成的适配任务优先使用 `projects[].dev.command`，未设置时使用项目原生的 `dev` 包脚本或 Taskfile 任务。项目 `mise.toml` 可覆盖任务，根 `mise.toml` 可替换工作区聚合。自定义 mise 命令使用 mise 的环境；生成的适配任务接收 One 冻结的项目环境。

## 依赖准备

`one dev` 在启动服务前自动准备所选项目的工具与应用依赖，交互和非交互调用行为一致。

- Node：在工作区根目录统一准备依赖。使用 pnpm 时，先通过 pnpm 自身检查工作区的安装状态；手动 `pnpm install` 后，只要依赖与当前工作区一致，就直接复用。需要安装时执行 `pnpm install --no-frozen-lockfile`，自动同步新增项目或依赖变更。pnpm 10.14 之前的版本沿用 One 的安装缓存。其他包管理器仍使用原有锁文件策略。
- `one build` 保留现有的严格安装策略：已有 pnpm 依赖锁文件时使用 `--frozen-lockfile`。构建发现锁文件过期后，需要先安装并审阅锁文件变更。
- Go：独立模块下载固定构建列表并补充 `go.sum`；存在 `go.work` 时由 Go 按实际包依赖解析本地成员和外部依赖，按需维护 `go.work.sum`。准备过程不自动运行 `go mod tidy` 或 `go work sync`。
- 所有准备成功后才启动 mise 任务图。失败保留底层错误和下载缓存，可修复后重试原命令；取消时停止准备进程。

`go.work.sum` 不替代各模块发布所需的 `go.sum`。模块声明需要修复时，可显式运行 `one exec api -- go mod tidy`。`one exec` 保持直接执行命令，不自动安装应用依赖。


## 日志、输入与退出

开发任务默认自动分配足够的并发数量。多服务使用 mise 带前缀的日志；指定 `--concurrency` 时需为所有开发服务留出运行位置。上游有限构建完成后才会启动依赖它的服务。

单个服务可使用 `--ui raw` 保留原生终端输入，此时关闭产物缓存。mise 的 raw/interactive 任务独占终端，需要交互的服务请单独运行。多开发服务不支持 raw 输出。原 TUI、交互项目选择器、`--keep-going` 和单服务重启控件已移除。

Ctrl+C 或 SIGTERM 会停止本次调用及其子进程。任务失败会停止本次调用并返回子进程退出码。JSON/YAML 使用 `one-cli/task-result/v1`，子进程日志写到 stderr。`--dry-run` 使用静态 `one-cli/task-plan/v1`，不安装工具、不读取密钥、不写配置。

通用参数、任务配置、缓存与命令重名规则见 [one run](/zh/docs/run/)。
