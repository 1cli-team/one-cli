---
title: 快速开始
description: 5 分钟跑通一个能用的 one 工作区 — create / add / 启动第一个项目。
---

5 分钟跑出第一个能用的 one 工作区，并把第一个项目启动起来。

**适合读这页的人**：装好 one 之后第一次跑；想验证环境 OK 的人；想快速体验概念的人。

**读完会**：手上有一个能启动的 Web 项目，并能在浏览器打开它。

> 还没装？先看 [安装](/zh/docs/installation/)。
> 想做生产级别的端到端流程？跳到 [创建生产可用的工作区](/zh/tutorials/first-workspace/)。

## Step 1：创建工作区

```bash
one create my-app
cd my-app
```

这一步会创建 `my-app/` 目录。接下来所有命令都在这个目录里执行。

## Step 2：加一个 Web 项目

快速开始固定用一个能直接在浏览器里打开的 Web 项目：

```bash
one add react-spa --name web
```

`one add` 不自动下载项目用到的包，下一步会下载。

## Step 3：下载依赖并启动项目

项目第一次运行前，需要先下载它用到的包。复制这条命令，在 workspace 根目录执行即可；第一次会稍微久一点：

```bash
pnpm install
```

下载完成后启动 Web 项目：

```bash
pnpm -C apps/web dev
```

看到终端打印的 `Local: http://localhost:.../` 后，用浏览器打开即可。这个 Web 示例本身不要求预置 `.env`，所以快速开始不需要配置 env。

## 完了

你已经创建并启动了第一个 Web 项目：

| 命令 | 干了什么 |
|---|---|
| `one create` | 创建工作区 |
| `one add` | 加一个 Web 项目 |
| `pnpm install` | 下载项目用到的包 |
| `pnpm -C apps/web dev` | 启动第一个 Web 项目 |

后面的能力按目标进入对应流程：环境变量用 `one env`，上线走 `one deploy`。容器镜像构建 / 推送属于部署流程里的底层环节，需要单独控制时再看进阶文档。

## 下一步

按你的目标选一条：

- **想真上生产？** → [创建生产可用的工作区](/zh/tutorials/first-workspace/)（端到端 30 分钟）
- **想接 Infisical secrets？** → [环境变量指南](/zh/tutorials/env-vars/)
- **想让 Claude 帮你跑 one？** → [安装 Skill 到 Agent](/zh/tutorials/skills-install/)
- **想查每个命令细节？** → [CLI 命令](/zh/docs/cli-overview/)
- **不想用？** 删掉 `my-app/` 文件夹就是。
