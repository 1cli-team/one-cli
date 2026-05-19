---
title: "为什么 CLI JSON 输出对 Coding Agent 很重要"
description: "稳定的 JSON envelope 让 agent 根据 error code 和 context 分支，而不是解析给人看的文案。"
date: "2026-05-17"
author: "One CLI Team"
tags: ["json", "cli", "agent"]
---

## 给人看的文本不是契约

人类友好的 CLI 输出适合终端阅读，但不适合作为自动化契约。文案会随着表达、语言和格式变化。如果 agent 必须解析句子才能判断错误原因，这个集成从一开始就很脆弱。

One CLI 把 JSON 输出当作命令契约的一部分。目标很简单：agent 应该读取结构化结果，根据稳定字段分支，并把有用上下文报告给用户。

## error code 比解析 message 更可靠

机器可读错误里最重要的是 code，而不是句子。类似 “template not found” 这样的 message 以后可能变得更友好，也可能换成另一种语言。但 code 应该保持稳定。

所以 agent workflow 应该优先使用：

```bash
one templates -o json
one create my-app --yes -o json
one add nextjs-app --name web --yes -o json
```

如果命令失败，agent 应该检查 `error.code` 和 `error.context`，而不是从 `error.message` 里抠语义。

## context 能减少重复探测

好的 CLI 错误不只是说失败了，还应该返回帮助调用方恢复的上下文。

例如模板名写错时，错误上下文可以携带可用模板。agent 就能直接展示有效选项，而不是再运行一次探测命令。目标目录冲突时，上下文也可以说明具体路径。

这样 agent 的循环会更短：

1. 用 JSON 输出运行命令。
2. 读取稳定 code 和 context。
3. 判断是否能自动恢复，还是需要问用户。
4. 报告精确原因。

## JSON 输出也是产品设计

JSON 输出经常被当作实现细节。对面向 agent 的 CLI 来说，它其实是产品设计。它定义了 agent 能信任什么，也定义了用户能审计什么。

One CLI 的命令表面很小，但输出契约让它可以被脚本、CI 和 coding agent 安全使用。这就是“能在终端跑”的 CLI 和“能参与 AI-native workflow”的 CLI 之间的差异。
