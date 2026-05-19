---
title: "面向 AI Workspace 的模板治理"
description: "当模板同时携带约定、依赖规则和生成说明时，agent 操作会更安全。"
date: "2026-05-17"
author: "One CLI Team"
tags: ["templates", "governance", "agent"]
---

## 模板不只是文件，也是规则

模板经常被理解成一组起步文件。对 AI-ready workspace 来说，模板还承载规则：依赖怎么安装、环境变量怎么说明、哪些文件是生成说明、哪些文件开始属于业务代码。

如果这些规则只隐含在文件结构里，agent 就只能推断。如果规则进入模板和 manifest 契约，agent 就可以检查。

## 治理从创建时开始

One CLI 模板的目标，是让第一个项目和后续操作保持一致。这意味着 `one create` 和 `one add` 不应该只写文件，还应该登记足够结构，让未来命令知道自己创建了什么。

好的模板治理要回答这些问题：

- 这个项目属于哪个类别。
- 它期望什么 runtime 和包管理器。
- 哪些默认命令可以安全运行。
- 哪些环境变量需要用户自己填写。
- 哪些文件是生成指南，哪些是应用代码。

这些信息对人有用，对 coding agent 更重要。

## agent 不应该发明约定

当 agent 打开一个生成 workspace 时，它不应该临时发明工作流。它应该读取 manifest，遵循 bundled skill，使用文档化命令。

这意味着模板治理必须足够明确。生成的 Next.js app、Go API 或文档站，都应该带着让 CLI 和 agent 后续理解它的元数据。

```bash
one templates -o json
one add go-api --name api --yes -o json
```

模板名、项目名和命令输出共同构成一条可审计的 setup 路径。

## 为什么时间越长越重要

治理的价值会在仓库交接之后变大。新的团队成员或新的 agent session 可以检查 workspace，并恢复当初的设计意图。这会降低上手成本，也减少本地 setup 误操作。

模板治理不是为了让脚手架更重，而是为了让生成的 workspace 足够耐用，可以被人和 agent 反复操作。
