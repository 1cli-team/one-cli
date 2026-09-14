---
title: one dev
description: 启动全部可开发项目，或只启动一个项目。
---

`one dev` 从 manifest 读取每个项目的开发命令，并用 One CLI 内置 supervisor 运行。

启用 mise 的 workspace 会自动在每个项目的 mise 工具环境中运行开发命令，仍然使用 `one dev` / `one dev web`，无需增加 runtime 参数。日志前缀、项目选择和整组服务的停止行为继续由原有 supervisor 负责。mise 的安装与旧项目启用见 [`one configure mise`](/zh/docs/configure/#mise-工作区工具配置)。

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

- Node：在工作区根目录安装一次。已有锁文件时执行冻结安装，锁文件与项目声明不一致会失败；新工作区首次安装生成锁文件。项目声明、工具版本改变或依赖目录被清理后会重新准备。
- Go：独立模块下载固定构建列表并补充 `go.sum`；存在 `go.work` 时由 Go 按实际包依赖解析本地成员和外部依赖，按需维护 `go.work.sum`。准备过程不自动运行 `go mod tidy` 或 `go work sync`。
- 所有准备成功后才启动 supervisor。失败保留底层错误和下载缓存，可修复后重试原命令；取消时停止准备进程。

`go.work.sum` 不替代各模块发布所需的 `go.sum`。模块声明需要修复时，可显式运行 `one run api -- go mod tidy`。`one run` 保持直接执行命令，不自动安装应用依赖。

## 运行方式

内置 supervisor 默认启动全部可开发项目，也可用位置参数只启动一个。

```bash
one dev
one dev web
one dev apps/web --dry-run
```

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `RUN_COMMAND_NOT_FOUND` | 检查原生工具安装或 mise 配置 |
| `ONE_CLI_ERROR` | 按底层依赖错误修复网络、锁文件或模块声明后重试 |
| `SUBPROJECT_NOT_FOUND` | 使用项目 `name` 或 `relativeDir` |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [本地开发编排](/zh/tutorials/dev-local/) — 内置 supervisor 的完整流程
- [`one run`](/zh/docs/run/) — 只给单条命令注入环境变量
- [manifest](/zh/docs/manifest/) — 项目列表的来源
