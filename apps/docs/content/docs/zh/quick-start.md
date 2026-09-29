---
title: 快速开始
description: 创建工作区、添加 Web 项目，用 one dev 准备依赖并启动开发服务。
---

从空工作区开始，添加一个 React Web 项目，再用 One CLI 启动它。

> 还没安装？先看[安装说明](/zh/docs/installation/)。

## 1. 创建工作区

```bash
one create my-app
cd my-app
```

One 会生成工作区目录、`one.manifest.toml`、`mise.toml` 和 `AGENTS.md`。接下来的命令都在 `my-app/` 中运行。

## 2. 添加 Web 项目

```bash
one add react-spa --name web
```

项目位于 `apps/web/`。One 将它登记到 Manifest，并把模板已有的任务加入根 `mise.toml`。需要浏览其他模板时，运行 `one templates`。

## 3. 启动开发服务

```bash
one dev -p web
```

One 按需准备 mise、项目工具和依赖，然后启动 Web 项目。首次运行需要联网，下载可能需要一些时间；后续会复用匹配的依赖。无需提前手动安装 pnpm 或激活 mise。

用浏览器打开日志中的 `Local: http://localhost:.../` 地址。这个示例不需要登录 Infisical 或绑定变量。按 Ctrl+C 停止服务。

## 4. 查看任务与构建

```bash
one run
one build -p web
```

`one run` 列出当前工作区任务。`one dev` 和 `one build` 分别是 `one run dev` 和 `one run build` 的简写。

想在浏览器里管理项目和开发服务，可以运行：

```bash
one serve
```

Dashboard 支持添加项目、管理变量、启动开发服务与查看控制台。

## 下一步

- [第一个工作区](/zh/tutorials/first-workspace/)：了解生成文件和目录约定。
- [任务管理](/zh/docs/run/)：选择多个项目、自定义任务和查看日志。
- [环境变量](/zh/tutorials/env-vars/)：登录 Infisical，再保存项目变量。
- [与 AI 协作](/zh/docs/ai-native/)：使用项目说明、命令帮助和结构化结果。
