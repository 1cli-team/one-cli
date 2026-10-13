---
title: one serve
description: Dashboard 已迁移至 One Desktop。
---

`one serve` 保留为迁移提示，不再启动 HTTP 服务或打开浏览器。原 `--port`、`--host`、`--open` 等服务参数已移除。

打开 One Desktop 管理本机工作区、登录、变量和共享凭据。`one create` 自动登记新工作区，已有工作区通过桌面版的“打开工作区”选择目录。工作区登记继续存放在 `~/.config/one/workspaces.json`，支持 `XDG_CONFIG_HOME`。

终端用户继续使用 `one run`、`one env`、`one login` 等命令。桌面版内置独立的 `one-api` 后端和 `one`，首次启动可选择安装终端 CLI。
