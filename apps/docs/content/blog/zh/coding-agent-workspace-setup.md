---
title: "Coding Agent 应该怎样准备一个 workspace 运行"
description: "安全的 workspace setup 应该从 manifest 开始，只补缺失依赖，并报告执行过的精确命令。"
date: "2026-05-17"
author: "One CLI Team"
tags: ["agent", "dependencies", "workspace"]
---

## 先找工程契约

当用户让 coding agent 准备一个项目运行时，第一步不应该是安装依赖。第一步应该是找到 workspace 契约。在 One CLI 工作区里，这个契约就是 `one.manifest.json`。

manifest 会告诉 agent：当前目录是不是 One workspace、根目录使用什么包管理器、有哪些子项目。它比根据 `apps/`、`services/` 或 package script 猜测要安全得多。

## 按工具链补依赖，而不是按习惯

常见错误是到处运行同一个 install 命令。对单技术栈小项目可能没问题，但在混合 workspace 里很容易出错。

One CLI 给 agent 的规则是按工具链区分依赖安装：

- JS、TS、Node 项目从 workspace root 使用声明的包管理器安装。
- Go 项目在对应 Go project 目录里运行 module 命令。
- `go mod tidy` 只在 import 变化或 module 元数据需要修复时使用，不要每次检查都习惯性运行。

这个区分可以减少 agent 对依赖文件的无意义改动。

## 一个可复用的 setup 流程

可靠的 agent 流程可以先从可解析命令开始：

```bash
one templates -o json
```

然后直接读取 manifest，判断依赖路径，只运行缺失的部分。对于使用 pnpm 的 Node workspace，通常是：

```bash
pnpm install
```

对于 Go service，通常是：

```bash
go mod download
```

具体命令应该由当前 workspace 状态决定，而不是由 agent prompt 里的固定习惯决定。

## 报告命令，而不是只说完成

setup 完成后，agent 应该告诉用户自己到底执行了什么。这样过程可复现，用户也能发现不必要的动作。

好的 setup 报告应该包含：

- 检测到的 workspace root。
- 使用的包管理器或 Go module 路径。
- 精确执行过的安装命令。
- 因依赖修复而变化的文件。
- 因依赖已经存在而跳过的命令。

这只是一个很小的纪律，但能避免很多隐藏的本地状态问题。
