---
title: "One CLI 和通用脚手架工具有什么不同"
description: "One CLI 关注 workspace 契约和 agent-safe 操作，而不只是初始化生成项目。"
date: "2026-05-17"
author: "One CLI Team"
tags: ["scaffolding", "comparison", "workflow"]
---

## 差异在生成之后

通用脚手架工具很有用，因为它们节省了最开始的 setup 步骤。它们创建 starter app、写配置文件，让项目快速进入熟悉的基础状态。这件事仍然有价值。

One CLI 面向的是另一个问题：文件已经存在之后怎么办。团队还需要继续加项目、向 agent 解释 workspace、安全安装依赖、配置部署目标，并让结构长期保持可理解。

## 通用脚手架优化项目开始

大多数脚手架工具优化的是快速开始：

- 选择框架。
- 生成文件。
- 安装依赖。
- 打印下一条命令。

这个流程对单个 app 足够。但当仓库变成 monorepo，或者 coding agent 需要结构化上下文时，它就不完整了。

如果后续自动化还要检查目录并猜哪些命令有效，就说明最初的脚手架没有留下足够契约。

## One CLI 优化 workspace 生命周期

One CLI 保留初始化生成流程，但在外面加了一层 workspace。manifest、template registry、JSON output 和 bundled skill 都是为了让未来操作有可靠起点。

差异会体现在日常任务里：

- `one add` 可以在不丢失 workspace 上下文的情况下追加项目。
- `one templates -o json` 给 agent 一个可解析的模板清单。
- 稳定 error code 让自动化不需要解析文本也能恢复。
- bundled skill 告诉 agent 如何按工具链安装依赖。
- manifest 告诉工具当前 workspace 里到底有什么。

这让 One CLI 不只是一次性生成器，更像 workspace contract manager。

## 什么时候这个区别重要

如果你只需要一个很小的单体 app，通用脚手架可能就够了。如果你预期 workspace 会包含多个项目、agent、部署路径或反复交接，那么契约比第一次写文件更重要。

One CLI 面向的是第二种情况。它仍然负责脚手架，但更大的目标是让生成后的 workspace 能被人、脚本和 coding agent 持续理解。
