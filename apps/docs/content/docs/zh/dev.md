---
title: one dev
description: 用 Procfile.dev 启动本地开发进程。
---

`one dev` 读取 workspace 根目录的 `Procfile.dev`，用本机可用的 supervisor 启动开发进程。`one create` 和 `one add` 会同步 `Procfile.dev`，所以 workspace 默认具备本地开发编排入口。

## 用法

```bash
one dev [-p <name|path>] [--dry-run]
```

## 参数

| 参数 | 说明 |
|---|---|
| `-p, --project <name|path>` | 只启动一个项目；支持 manifest 里的 `name` 或 `relativeDir` |
| `--dry-run` | 只打印将调用的 supervisor 命令，不启动进程 |
| `-o, --output <fmt>` | `json` / `yaml` / `text`（默认按 TTY 检测） |

## 交互模式

`one dev` 没有交互式向导；它按 manifest 和 `Procfile.dev` 直接启动进程。需要先确认会跑什么时，用 `--dry-run`。

## 运行方式

CLI 会按本机 PATH 查找 Procfile supervisor。支持的 runner 包括 overmind、hivemind、foreman、honcho；全部缺失时返回 `DEV_NO_SUPERVISOR`。

```bash
one dev
one dev -p web
one dev -p apps/web --dry-run
```

## Procfile.dev

`Procfile.dev` 的每一行对应一个本地进程：

```text
api: pnpm --dir services/api dev
web: pnpm --dir apps/web dev
```

如果新增项目后文件缺失或旧了，重新跑相关 `one add` 或检查模板同步。不要手工把它当成业务配置源；真正的项目列表仍以 `one.manifest.json` 为准。

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `DEV_NO_SUPERVISOR` | 安装 overmind / hivemind / foreman / honcho 中任意一个 |
| `DEV_PROCFILE_MISSING` | 重新跑一次 `one add` 触发 Procfile 同步，或检查 workspace 是否完整 |
| `DEV_PROJECT_NOT_FOUND` | 用 manifest 里的 `name` 或 `relativeDir` 作为 `-p` |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [本地开发编排](/zh/tutorials/dev-local/) — 从 Procfile 到 supervisor 的完整流程
- [`one run`](/zh/docs/run/) — 只给单条命令注入环境变量
- [manifest](/zh/docs/manifest/) — 项目列表的来源
