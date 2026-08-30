---
title: one dev
description: 启动全部可开发项目，或只启动一个项目。
---

`one dev` 从 manifest 读取每个项目的开发命令，并用 One CLI 内置 supervisor 运行。

## 用法

```bash
one dev [project] [--dry-run]
```

## 参数

| 参数 | 说明 |
|---|---|
| 位置参数 `project` | 只启动一个项目；支持 manifest 里的 `name` 或 `relativeDir` |
| `-p, --project <name|path>` | 为旧脚本和 CI 保留的选择参数 |
| `--dry-run` | 只打印将调用的 supervisor 命令，不启动进程 |
| `-o, --output <fmt>` | `json` / `yaml` / `text`（默认按 TTY 检测） |

## 交互模式

选中的 Node 项目缺少依赖时，终端会询问是否运行检测到的包管理器安装命令。确认后安装并继续，拒绝则成功退出；非交互调用返回 `DEPENDENCIES_NOT_INSTALLED` 和准确安装命令。

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
| `DEPENDENCIES_NOT_INSTALLED` | 执行 remediation 给出的安装命令，再重试 |
| `SUBPROJECT_NOT_FOUND` | 使用项目 `name` 或 `relativeDir` |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [本地开发编排](/zh/tutorials/dev-local/) — 内置 supervisor 的完整流程
- [`one run`](/zh/docs/run/) — 只给单条命令注入环境变量
- [manifest](/zh/docs/manifest/) — 项目列表的来源
