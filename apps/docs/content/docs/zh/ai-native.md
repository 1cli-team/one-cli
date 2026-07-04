---
title: 'AI Native 治理规则'
description: 'One CLI 怎样把 agent 调用、错误恢复和仓库级工程契约变成可执行的治理规则。'
---

One CLI 里的 "AI Native" 不是内置聊天助手，也不是给命令套一层提示词。它是一组治理规则：agent 可以直接调用 CLI、读取结构化结果、按错误码恢复，并从仓库里拿到长期有效的工程契约。

**适合读这页的人**：评估 One CLI 是否值得引入的人；要给团队定义 agent 使用边界的人；想确认 One CLI 和普通脚手架差异的人。

**读完会**：知道 One CLI 管什么、不管什么，以及 agent 在这个工作区里应该按哪些规则行动。

## 治理范围

One CLI 不只生成项目文件，也会在 monorepo 里留下可持续执行的治理层。

这个治理层覆盖四类边界：

1. **自动化接口**：agent 和 CI 读取结构化输出，不抓终端文本
2. **错误恢复**：agent 按稳定错误码、上下文和恢复建议处理失败
3. **工程契约**：agent 从仓库里的 `AGENTS.md` 和 `.one/agents/` 读取长期规则（`CLAUDE.md` 指向 `AGENTS.md`）
4. **权限边界**：本机凭据、环境和部署配置有明确归属，不交给 agent 猜

## 规则一：命令输出必须可解析

面向 agent 或 CI 时，命令结果要走结构化接口：

```bash
one templates -o json
```

返回的是稳定 schema：

```json
{
  "schema": "one-cli/templates/v1",
  "total": 10,
  "templates": [
    {
      "id": "nestjs-api",
      "category": "backend",
      "toolchain": "node"
    }
  ]
}
```

终端里直接运行 `one templates` 仍然是人类可读的表格；pipe / 非 TTY 默认输出 JSON。脚本和 agent 仍建议显式传 `-o json`，避免执行环境变化影响解析。

治理含义很简单：**人看文本，agent 读 schema**。文案、颜色、表格可以调整；JSON schema 才是自动化契约。

## 规则二：错误恢复看 code / context / remediation

One CLI 的错误使用统一 envelope：

```json
{
  "schema": "one-cli/error/v1",
  "error": {
    "code": "TEMPLATE_NOT_FOUND",
    "message": "模板 \"api-fastify\" 不存在，使用 `one templates` 查看可用模板。",
    "context": {
      "available_templates": [
        "nestjs-api",
        "go-api",
        "astro-site",
        "starlight-docs",
        "nextjs-app",
        "react-spa",
        "expo-mobile",
        "ts-library",
        "go-lib",
        "electron-app"
      ],
      "requested_template": "api-fastify"
    },
    "remediation": [
      {
        "action": "list-templates",
        "hint": "查看所有可用模板 ID",
        "command": "one templates -o json"
      }
    ]
  }
}
```

agent 的处理顺序应该是：

1. 读 `error.code` 判断错误类型
2. 读 `error.context` 使用 CLI 已经给出的上下文
3. 优先采用 `error.remediation[]` 里的恢复动作
4. 只有缺少上下文时才回到人类确认

不要按 `message` 文本做分支。`message` 是给人看的，可能会翻译、改写；`code` 才是稳定接口。

完整码表见 [错误码大全](/zh/docs/error-codes/)。

## 规则三：工程契约要留在仓库里

One CLI 创建工作区和添加模板时，会维护仓库级 agent harness：

- `AGENTS.md`：canonical 的瘦路由入口，给 Codex 等 agent 读取
- `CLAUDE.md`：生成的一行指针，让 Claude Code 跟随 `./AGENTS.md`
- `.one/agents/conventions.md`：工作区约定和 One CLI 操作规则
- `.one/agents/projects/<dir>.md`：每个 manifest 项目一份技术栈指南，文件名来自 `relativeDir`，斜杠会拍平成连字符
- `.one/agents/ops/*.md`：当 manifest 启用相关 domain 时生成 `one dev`、`one env`、`one container`、`one deploy` 操作指南

这些文件不是一次性提示词，而是 `one.manifest.json` 的投影：manifest 是事实源，`AGENTS.md` 保持小而稳定，细节文件按需打开。One CLI 不会再把 agent stub 复制进子项目目录。

示例约定：

```text
- Do not put business logic in Controllers.
- DTOs must use class-validator decorators.
- Use HttpException subclasses; do not throw bare Error.
- Use pino logging; request traceId is injected automatically.
```

受管区块由 CLI 刷新，区块外可以写团队自己的约定。如果受管区块不对，应该修模板或重新运行对应 One CLI 流程，而不是手动改生成块。

## 规则四：配置和凭据有边界

One CLI 可以管理 env、container、deploy 等机器级配置，但 agent 不应该接触真实凭据。

推荐边界：

- `one configure` / `one.manifest.json` 记录可审计的选择和 profile 引用
- `one serve` 在 `127.0.0.1` 打开本地配置界面，敏感值由人手工录入
- `.env*`、私钥、云厂商 token 不进 Git，也不写进 agent 可复用文档
- agent 可以读取结构化状态、执行缺失依赖安装和项目生成，但涉及发布、删除、覆盖凭据时应回到团队策略或人工确认

这也是治理规则的一部分：One CLI 让 agent 能行动，但不会把所有权限默认交给 agent。

## 自检表

评估一个工具是否适合 agent 直接调用，可以问这 5 个问题：

1. pipe / 非 TTY 输出是否默认可解析？
2. 是否支持显式 `-o json`？
3. 错误是否有稳定 `code`？
4. 错误是否带 `context` 和 `remediation`？
5. 仓库里是否有 agent 可读取、可持续更新的工程契约？

都满足，才适合被长期放进 agent 工作流。只做到一两条，就是普通 CLI 加了一点自动化友好性。

## 怎么验证这套规则

以当前安装的 `one` 为准：

```bash
one templates -o json
```

在任意 One 工作区里触发一个不存在的模板，可以看到 `one-cli/error/v1` 错误 envelope：

```bash
one add api-fastify --name api --yes -o json
```

添加一个真实模板后，可以检查工作区级路由和集中式细节指南：

```bash
one add nestjs-api --name api --yes -o json
ls AGENTS.md CLAUDE.md .one/agents/conventions.md .one/agents/projects/services-api.md
```
