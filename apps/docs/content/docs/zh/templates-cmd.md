---
title: one templates
description: 列出可用模板及其元数据。
---

`one templates` 把模板注册表里所有可用模板列出来。终端里看人类格式；pipe / `-o json` 拿结构化。

## 用法

```bash
one templates [-o <fmt>]
```

## 参数

| 参数 | 说明 |
|---|---|
| `-o, --output <fmt>` | `json` / `yaml` / `text`（默认按 TTY 检测） |

## 交互模式

`one templates` 本身没有交互式向导：它只列模板。人类在终端直接运行会看到易读列表；脚本和 agent 用 `-o json` 读取模板 ID、分类和兼容 backend。

如果你想边看边选模板，请运行 [`one add`](/zh/docs/add/) 的交互模式。

## 输出（schema `one-cli/templates/v1`）

```jsonc
{
  "schema": "one-cli/templates/v1",
  "templates": [
    {
      "id": "nestjs-api",
      "code": "ne",
      "category": "backend",
      "name": "NestJS API 服务",
      "description": "NestJS + TypeScript，适合 API 服务与业务后台",
      "toolchain": "node",
      "tags": ["api", "nestjs", "typescript", "backend"],
      "domains": {
        "container": { "default": "docker", "compat": ["docker"] },
        "deploy": { "default": "kustomize", "compat": ["kustomize"] }
      }
    },
    {
      "id": "go-api",
      "code": "go",
      "category": "backend",
      "name": "Go API 服务",
      "toolchain": "go"
    },
    // ... 8 more
  ]
}
```

## 完整模板列表

当前注册表有 10 个模板。每个都有详细对比页：

| ID | 类别 | 详细 |
|---|---|---|
| `nestjs-api` | backend | [→](/zh/docs/templates/) |
| `go-api` | backend | [→](/zh/docs/templates/) |
| `astro-site` | frontend | [→](/zh/docs/templates/) |
| `starlight-docs` | frontend / docs | [→](/zh/docs/templates/) |
| `nextjs-app` | frontend | [→](/zh/docs/templates/) |
| `react-spa` | frontend | [→](/zh/docs/templates/) |
| `expo-mobile` | frontend / mobile | [→](/zh/docs/templates/) |
| `ts-library` | library | [→](/zh/docs/templates/) |
| `go-lib` | library | [→](/zh/docs/templates/) |
| `electron-app` | frontend / desktop | [→](/zh/docs/templates/) |

不知道选哪个？看 [模板决策树](/zh/docs/templates/)。

## 示例

### 人类

```bash
one templates
```

### Agent / 脚本

```bash
# 拿所有模板 ID
one templates -o json | jq -r '.templates[].id'

# 按类别过滤
one templates -o json | jq '.templates[] | select(.category == "backend")'

# 拿单个模板的描述
one templates -o json | jq '.templates[] | select(.id == "nestjs-api")'
```

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `REGISTRY_FETCH_FAILED` | 网络问题；context 里有 registry url |
| `REGISTRY_INVALID` | 注册表 JSON 损坏；联系维护方 |
| `REGISTRY_NOT_FOUND` | 注册表路径不存在 |
| `NO_TEMPLATES` | 注册表是空的（不应该发生） |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [模板决策树](/zh/docs/templates/) — 怎么选
- [`one add`](/zh/docs/add/) — 选好模板后用它加进工作区
