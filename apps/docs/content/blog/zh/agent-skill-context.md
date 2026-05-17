---
title: "给 agent 的 one-cli skill 应该记录什么"
description: "One CLI 只暴露一个 bundled skill，里面记录命令契约、preset 词汇和依赖补齐规则。"
date: "2026-05-12"
author: "One CLI Team"
tags: ["skill", "codex", "dependencies"]
---

## skill 不是营销文档

One CLI 的 bundled `one-cli` skill 面向的是 Codex、Claude Code、Cursor 这类本地 coding agent。它不应该重复官网介绍，也不应该写成抽象架构说明。真正有价值的是可执行的工作规则：什么时候读 manifest、什么时候调用 `one templates -o json`、什么时候可以补依赖、什么时候必须停下来让用户决定。

对 agent 来说，skill 的作用是降低猜测空间。它需要把工程里的“隐性约定”变成明确步骤。

## 需要记录的三类信息

第一类是命令契约。One CLI 的命令输出支持 JSON envelope，错误也有稳定的 `error.code`。skill 应该要求 agent 使用 `-o json`，并从 `error.context` 读取恢复所需的数据，而不是解析自由文本。

第二类是 workspace 事实源。只要目录里有 `one.manifest.json`，就应该把它当成 One workspace 的根。agent 不能靠 `apps/`、`packages/` 或 package.json 猜根目录，因为这些结构在不同模板组合里可能变化。

第三类是依赖补齐规则。JS、TS、Node 项目应在 workspace root 按声明的包管理器安装；Go 项目应进入对应 Go subproject 后运行 module 命令。这个差异必须写清楚，否则 agent 很容易在错误目录里运行安装命令。

## 为什么只暴露一个 skill

把每个命令拆成一个 skill 看起来更细，但实际会让 agent 在入口处做更多选择。One CLI 更适合暴露一个统一的 `one-cli` skill，再在 skill 内部按任务路由到 bootstrap、add-feature、dependencies、reference 等工作流。

这样做有两个好处：

- 用户只需要告诉 agent “使用 one-cli skill”。
- skill 内部可以共享 manifest、preset、模板目录和错误恢复规则。

这和 CLI 本身的设计一致：外部表面保持小，内部规则保持清楚。

## 一个合格的 agent 操作顺序

当用户让 agent 准备运行一个 One workspace 时，推荐顺序是：

1. 向上寻找最近的 `one.manifest.json`。
2. 读取 manifest，确认 package manager 和 subprojects。
3. 对 JS/TS/Node 依赖在 workspace root 执行安装。
4. 对 Go subproject 进入子目录执行 `go mod download`，修改 imports 或需要修复模块元数据时再执行 `go mod tidy`。
5. 把实际运行过的命令报告给用户。

这套顺序的目标不是自动做更多事，而是把“可以自动做的事情”限制在清楚边界内。
