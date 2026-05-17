---
title: "为什么 One CLI 把 manifest 放在中心"
description: "one.manifest.json 不只是脚手架产物，它是人、CLI 和 agent 共同读取的工程契约。"
date: "2026-05-12"
author: "One CLI Team"
tags: ["manifest", "agent", "monorepo"]
---

## manifest 是工程事实源

One CLI 生成的工作区里，`one.manifest.json` 是最重要的文件之一。它不是给 CLI 自己看的临时缓存，而是描述整个 workspace 的事实源：有哪些 app、有哪些 package、使用什么 preset、当前项目依赖哪些运行边界。

传统脚手架通常只负责把文件写出来。生成完成后，项目结构变成一堆约定，后续工具只能靠目录名、package script 或 README 猜测真实意图。One CLI 的做法是把这些决定写进 manifest，让后续命令和 agent 都能读取同一份结构化上下文。

## 为什么不只靠目录结构

目录结构能回答“文件在哪里”，但回答不了“这个目录为什么存在”。例如 `apps/web` 可能是 Next.js，也可能是 Vite React；`packages/shared` 可能是可发布库，也可能只是内部工具包。对人来说，这些差异可以靠经验判断；对 agent 来说，靠猜测会直接放大误操作风险。

manifest 让这些语义变成显式字段：

- 项目类型和模板来源可以被稳定读取。
- 后续 `one add` 可以判断新增模块应该写到哪里。
- agent 能先确认 workspace 边界，再决定是否安装依赖或运行命令。
- CI、部署和本地运行配置可以引用同一份项目清单。

## agent 需要可验证的上下文

给 agent 的指令如果只写“这是一个全栈项目”，信息量太低。agent 还需要知道这个全栈项目由哪些具体模板组成、包管理器是什么、Go 模块在哪里、哪些文件是生成产物，哪些文件是用户维护的业务代码。

这也是 One CLI 把 manifest 和 bundled `one-cli` skill 放在一起设计的原因。manifest 描述当前 workspace，skill 描述应该如何操作它。两者合起来，agent 才能从“我猜这个项目大概是这样”变成“我先读取事实，再做最小必要动作”。

## 一个更稳定的工作流

理想的 One CLI 工作流是这样的：

```bash
one create my-stack --preset 1.bgok.fnav.ei --yes
cd my-stack
one add nextjs-app --name admin --yes
```

每一步都会把结构变化写回 manifest。后续人类开发者打开仓库，能看到项目的真实边界；agent 接手维护，也能从 manifest 开始检查，而不是从一堆脚本和目录名里猜。

manifest 的价值不在于多一个配置文件，而在于把脚手架、文档、CLI 命令和 agent 操作收敛到同一份工程契约。
