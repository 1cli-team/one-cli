---
title: CLI 总览
description: One CLI 日常命令和高级入口。
---

| 命令 | 用途 |
| --- | --- |
| `one create` | 创建工作区 |
| `one add` | 添加项目 |
| `one dev` | 启动开发 |
| `one build` | 执行项目构建 |
| `one env` | 查看和管理项目环境变量 |
| `one login` / `one whoami` / `one logout` | 单账号浏览器会话 |
| `one env --global` | 发现全局变量位置与环境 |
| `one run` | 注入变量后执行命令 |
| `one serve` | 打开 Dashboard |
| `one locale` | 本机语言 |
| `one init mise` / `one init hooks` | 工作区工具配置 |
| `one ci` / `one templates` / `one skills` | 自动化与资源 |

通过 `one help --all` 发现完整命令，执行前查看对应 `--help`。Agent 无需在 Dashboard 复制执行命令，可以自主读取帮助、列出目录与变量元数据，再用明确的环境和目录执行任务。详情见[登录与全局变量](/zh/docs/login/)。
