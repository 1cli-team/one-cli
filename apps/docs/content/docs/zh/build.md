---
title: build 任务
description: 通过 mise 构建项目及其本地依赖。
---

`one build` 是 `one run build` 的简写。`build` 和 `test`、`lint` 一样是普通任务名，共用任务解析、参数和帮助。最终执行 mise 中生效的任务图，包括你在 `mise.toml` 中定义的覆盖。

```sh
one build
one build -p web -p api
one build -p web --env prod
one build -p web --dry-run -o json
one build --force
one build -p web --ui raw
```

Node 从 `package.json` 读取命令，使用 pnpm；Go 从 Taskfile 读取命令。全工作区构建只包含存在 build 的项目，显式选择没有 build 的项目会报错。本地 Node 上游先于下游构建。默认并发为 1，`--concurrency 4` 可并行执行独立任务。

Dashboard 展示项目构建命令及其来源；修改对应源文件即可改变命令。跨项目依赖和缓存规则由 mise 配置声明。完整任务管理、自定义缓存、终端模式和 Actions 示例见 [one run](/zh/docs/run/)。
