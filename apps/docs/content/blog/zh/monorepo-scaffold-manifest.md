---
title: "为什么 Monorepo 脚手架需要 manifest"
description: "monorepo 脚手架不应该只记录文件生成在哪里，还应该记录项目为什么存在。"
date: "2026-05-17"
author: "One CLI Team"
tags: ["manifest", "scaffold", "monorepo"]
---

## 文件结构只是第一层

大多数脚手架都能生成目录结构。这很有用，但 monorepo 需要的不只是目录。一个 workspace 里有项目、角色、依赖、部署目标和约定，这些信息应该在第一次生成之后继续可见。

如果没有 manifest，后续工具只能从目录名和脚本推断意图。仓库刚创建时也许还能工作，但随着团队继续增加服务、包和部署路径，这种推断会越来越不可靠。

## manifest 解释意图

One CLI 写入 `one.manifest.json`，让 workspace 保留一份结构化的自身描述。后续命令和 agent 都可以从 manifest 读取项目清单和运行意图。

这对很多常见任务都有用：

- 用 `one add` 继续追加前端或后端。
- 判断依赖应该安装在哪里。
- 理解当前项目来自哪些模板。
- 让生成的 agent 指南和 workspace 保持一致。
- 让 agent 先读取事实，再改文件。

manifest 不是代码的替代品，而是告诉工具“代码如何组织”的地图。

## 脚手架决策应该能被再次读取

第一次 scaffold 命令里包含很多有价值的决定：选择了哪个模板、选择了哪个部署目标、使用哪种环境变量策略。如果这些决定只沉到文件里，未来每个工具都要重新发现。

manifest 会把这些决定留下来。这样下一个命令不需要从零开始猜，自动化也更安全。

```bash
one create product-suite --yes -o json
one add nestjs-api --name api --yes -o json
one add nextjs-app --name web --yes -o json
```

每一步都应该留下足够结构，让下一步更安全。

## agent 视角

coding agent 需要边界。manifest 能让 agent 在行动前回答几个基础问题：我是不是在 One workspace 里？有哪些项目？哪些是生成内容？应该使用哪个包管理器？

这也是为什么 manifest-driven scaffolding 比一次性生成目录更适合 AI-native development。工具不只是创建文件，还会保留 agent 之后需要读取的 workspace 事实。
