---
title: "Preset ID 应该解决什么问题"
description: "Preset ID 是可复现的模板组合编码，不应该膨胀成另一个业务场景 DSL。"
date: "2026-05-12"
author: "One CLI Team"
tags: ["preset", "templates", "product-design"]
---

## preset 是模板组合，不是业务剧本

One CLI 的 preset ID 用来表达一组模板选择。例如一个 preset 可以表示 Go API、Next.js app 和 TypeScript library 的组合。它的目标是让脚手架输入短、稳定、可复现，而不是把所有业务意图都编码进一个字符串。

这个边界很重要。只要 preset 开始承载“电商后台”“内容平台”“社交应用”这类业务场景，它就会很快变成另一套 DSL。DSL 会要求解释器、版本迁移、兼容策略和大量例外规则，反而让 CLI 的核心变重。

## compact code 的价值

compact preset code 的价值在于确定性：

- 相同 preset 在不同机器上展开相同模板组合。
- preset code 可以写进 manifest，方便审查和回放。
- 文档、模板构建器和 agent prompt 能共享同一套短编码。
- 不支持的版本可以在写文件前直接失败。

这类能力对脚手架和治理很关键，但它们都停留在工程结构层，不替用户做业务建模。

## 自定义项目名交给 fallback command

模板组合可以编码，但项目名、目录名和后续配置往往更适合留在命令层。例如同一个 `nextjs-app` 模板可以添加为 `web`、`admin` 或 `console`。这些名字不是 preset 的核心语义，写进 fallback commands 更清楚。

```bash
one create my-stack --preset 1.bgok.fnav.ei --yes
one add nextjs-app --name admin --yes
```

这种分工让 preset 只负责“选了哪些模板”，让 CLI 命令负责“这些模板在当前 workspace 里叫什么、写到哪里”。

## 保持小表面

One CLI 的产品表面应该保持小：`create`、`add`、`templates`、`configure`、`serve`、`skills` 这些命令已经覆盖主要路径。preset 是 `create` 的输入之一，不需要成为新的中心概念。

好的 preset 设计应该让用户少输入、让 agent 少猜测、让生成结果可回放。它不需要解释每一个业务场景，也不应该替代清晰的模板目录和 manifest。
