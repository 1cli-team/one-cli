---
title: "AI-Native Monorepo CLI 到底要控制什么"
description: "AI-native monorepo CLI 不只是生成目录，而是给 agent 一个稳定的工作区契约。"
date: "2026-05-17"
author: "One CLI Team"
tags: ["ai-native", "monorepo", "cli"]
---

## AI-native 是操作边界

AI-native CLI 不是简单地在介绍里写上 agent。它真正要解决的是：agent 能不能在一个稳定边界里工作。关键问题不是 agent 能不能改文件，而是 agent 能不能发现 workspace 结构、知道哪些文件是生成产物、运行正确的依赖命令，并把失败结果用可解析的方式返回。

对 monorepo 来说，这个边界更重要。一个仓库里可能同时有前端、后端、文档站、共享包、移动端和部署配置。如果没有共同契约，每次命令执行和 agent 接手都要重新猜。

## CLI 必须显式表达哪些事实

One CLI 把 workspace manifest 当作共同契约。生成的项目、模板来源、包管理器选择和运行意图，都应该来自结构化数据，而不是散落在 README 文案里。

一个 AI-native monorepo CLI 至少要显式表达这些事实：

- workspace root 在哪里。
- 哪些项目是 app、service、package 或 docs。
- 每个项目来自哪个模板。
- 每个项目应该用哪类依赖工具链。
- 哪些命令可以自动运行。
- 哪些错误有稳定的机器可读 code。

这也是为什么 `one create`、`one add`、`one templates` 和 JSON 输出属于同一个产品面。脚手架负责开始项目，但契约负责让项目在第一次生成之后仍然可维护。

## agent 需要的不只是 README

人可以读 README，再对照文件树推断缺失信息。agent 也可以这样做，但更慢，也更不稳定。如果 agent 要判断是在根目录运行 `pnpm install`，还是进某个服务里运行 `go mod download`，只靠目录名猜是不够的。

One CLI 的 bundled skill 给 agent 操作规则，manifest 给 agent 当前状态。两者配合后，agent 工作会更确定：

```bash
one templates -o json
one create my-app --yes -o json
one add nextjs-app --name web --yes -o json
```

这些命令对人也有用，但真正适合自动化的是 JSON envelope 和稳定 error code。

## 真正的差异点

大多数脚手架优化的是项目开始的第一分钟。AI-native monorepo CLI 要优化的是之后的交接：人让 agent 加服务、补依赖、检查 manifest，或者准备 workspace 运行。

稳定的 CLI 契约就在这里产生价值。它让 monorepo 不再只是一堆生成出来的文件，而是人、脚本和 agent 都能理解的工作区。
