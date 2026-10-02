---
title: CLI 总览
description: One CLI 日常命令和高级入口。
---

| 命令 | 用途 |
| --- | --- |
| `one create` | 创建工作区 |
| `one add` | 添加项目 |
| `one env` | 查看和管理项目环境变量 |
| `one login` / `one whoami` / `one logout` | 单账号浏览器会话 |
| `one env --global` | 发现共享凭据位置与环境 |
| `one exec` | 注入变量后执行命令 |
| `one serve` | 打开 Dashboard |
| `one locale` | 本机语言 |
| `one upgrade` | 将 One CLI 更新到最新稳定版 |
| `one init mise` / `one init hooks` | 工作区工具配置 |
| `one run` | 列出和执行工作区任务 |
| `one skills` | 透传命令到 `npx skills` |
| `one templates` | 查看项目模板 |
| `one hk` | 执行 Git hooks 检查 |

`dev`、`build`、`test`、`lint` 都是普通工作区任务名。`one <任务名>` 是 `one run <任务名>` 的简写：`one dev` 执行 `one run dev`，`one build` 执行 `one run build`。通用参数与任务发现见[任务管理](/zh/docs/run/)。

通过 `one help --all` 发现完整命令，执行前查看对应 `--help`。Agent 无需在 Dashboard 复制执行命令，可以自主读取帮助、列出目录与变量元数据，再用明确的环境和目录执行任务。详情见[登录与共享凭据](/zh/docs/login/)。
