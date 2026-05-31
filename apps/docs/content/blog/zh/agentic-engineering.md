---
title: "Agentic Engineering 需要结构化 workspace"
description: "Agentic Engineering 的重点不是把 prompt 写得更长，而是让 AI coding agents 有清晰的 workspace 状态、可复现命令和更少猜测。"
date: "2026-05-31"
author: "One CLI Team"
tags: ["agentic-engineering", "workspace", "cli"]
---

## Agentic Engineering 是一种团队工程实践

Agentic Engineering 指的是：把软件团队的工程流程设计成适合 AI coding agents 接手执行的形态。它不只是 prompt 工程，也不是简单地让 agent 有权限改文件。真正重要的是 repository 结构、命令契约、项目元数据、依赖边界和 review 流程，能不能让人把任务交出去时，也把任务依赖的上下文交清楚。

对软件团队来说，这个变化首先是操作层面的。一个会改代码的 agent，需要知道同事也需要知道的事实：这是哪类 workspace、要改哪个 project、依赖怎么安装、用什么命令验证、什么输出代表成功或失败。如果这些信息只存在于习惯、README 描述或口头约定里，每一次 agent 执行都会先从反向推断开始。

## Coding agents 需要 workspace 契约

AI coding agents 可以检查文件，但“能看文件”和“理解工程意图”不是一回事。一个仓库里有 `apps/web`、`services/api`、`packages/ui`，并不能说明根目录该用哪个包管理器、这些 project 来自什么模板、哪些命令可以在哪个目录安全执行。

结构化 workspace 会在 agent 动手之前给出一份契约：

- workspace root 是明确的。
- project 名称和路径是显式记录的。
- project 类型和 toolchain 可以被读取。
- 依赖安装方式跟随 project 类型。
- build、test、dev、env 命令有稳定入口。
- 自动化场景能读取机器可解析输出。

这份契约减少了 agent plan 里的猜测，也让 review 更容易追踪。agent 报告的不只是“我做完了”，而是能对齐 CLI 和 manifest 记录过的同一批事实。

## Manifest-driven CLI 会减少模糊地带

Manifest-driven CLI workflow 会把项目 setup 里的关键决策写成持久状态。这样 agent 不需要从目录名里推断所有事情，而是可以先读取人和工具共同维护的结构化事实。

在 One CLI workspace 里，这个文件就是 [`one.manifest.json`](/zh/docs/manifest/)。它记录 workspace identity、projects、template 来源、toolchain、environments 和 project-level domains。多一个 manifest 的目的不是增加仪式感，而是把工程意图显式化，让后续命令和 agent session 都能从事实开始。

这在重复性工作里尤其重要：

- 新增 frontend、backend 或 package 时，应该更新同一份 project registry。
- 运行命令时，应该能解析到正确的 project 目录。
- 安装依赖时，应该跟随 manifest 记录的 toolchain。
- 给 agent 的操作规则，应该指向和 CLI 相同的事实源。

当 manifest 成为契约时，人可以提出任务，agent 可以先检查 workspace，再决定下一步怎么做。

## One CLI 在这里的位置

[One CLI](/zh/) 是面向 AI coding agents 的结构化 workspace CLI。它不是 AI agent framework，也不负责帮你构建 autonomous agents。它的边界更窄：创建和维护一个更容易被人、脚本和 coding agents 安全操作的 workspace。

实际用到的是这些能力：

- [`one create`](/zh/docs/create/) 创建 workspace skeleton 和 manifest。
- [`one add`](/zh/docs/add/) 添加模板化 project，并把它登记进 manifest。
- [`one run`](/zh/docs/run/) 解析 project，并从正确目录执行命令。
- JSON 输出和稳定 error code 让 agent 更容易解析命令结果。
- [`one skills install`](/zh/docs/skills/) 把 One CLI 的操作规则安装到支持的 coding agents。

这些能力支撑 Agentic Engineering 的方式很朴素：让 workspace 少一点隐式约定。agent 仍然需要理解任务、阅读代码、提出或执行修改；One CLI 提供的是一个更稳定的起点。

## 一个实际的 One CLI 流程

一个小团队可以从第一条命令开始，把新的 agent-ready workspace 做成可复现流程：

```bash
one create agentic-product --yes
cd agentic-product
one add nextjs-app --name web --yes -o json
one add nestjs-api --name api --yes -o json
cat one.manifest.json | jq
```

到这一步，workspace 结构已经被记录下来。coding agent 可以先检查 `one.manifest.json`，看到 `web` 和 `api` 是两个独立 project，而不是猜某条命令应该在根目录跑，还是进某个子目录跑。

接下来给 agent 的任务就可以更具体：

```text
先读取 One CLI workspace manifest。
新增一个 shared TypeScript package，用来放 API client types。
运行相关 build 或 lint 命令，并报告精确命令。
```

agent 可以沿用开发者也会使用的命令契约：

```bash
one add ts-library --name api-client --yes -o json
one run -p web -- pnpm lint
one run -p api -- pnpm test
```

如果命令失败，JSON 输出和文档化 error code 会比解析本地化 help 文案更可靠。整个过程仍然需要人 review，但它的起点是一组更稳定的工程事实。

## 先从契约开始

改进 Agentic Engineering 最直接的方式，是在 agent 开始改代码之前，先去掉不必要的模糊地带。给 workspace 一份 manifest，用可复现 CLI 命令，优先使用机器可读输出，并把 setup 步骤留在 repository 里。

想试这个流程，可以从[快速开始](/zh/docs/quick-start/)看起，或者走一遍[手动创建工作区](/zh/tutorials/first-workspace/)。如果想看适合 agent 的命令输出模式，继续读[输出和错误码](/zh/tutorials/json-output-error-codes/)。
