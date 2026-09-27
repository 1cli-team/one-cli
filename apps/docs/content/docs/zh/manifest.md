---
title: one.manifest.json 是什么
description: 工作区的项目登记表、环境变量来源和本地开发配置。
---

每个 One CLI 工作区根目录都有 `one.manifest.json`。它记录工作区身份、项目路径、工具链、环境和环境变量来源，供命令定位项目并选择执行方式。

## 示例

```json
{
  "version": 1,
  "workspace": { "id": "demo-app-2bb61e", "name": "demo-app" },
  "environments": {
    "names": ["dev", "preview", "prod"],
    "default": "dev"
  },
  "domains": { "env": { "kind": "dotenv" } },
  "projects": [
    {
      "name": "web",
      "templateId": "react-spa",
      "relativeDir": "apps/web",
      "toolchain": "node",
      "buildVersion": "0.1.0",
      "packageManager": "pnpm",
      "domains": {
        "env": { "path": ".env", "inherits": true, "keys": ["API_URL"] },
        "dev": { "command": "pnpm dev" }
      }
    },
    {
      "name": "api",
      "templateId": "go-api",
      "relativeDir": "services/api",
      "toolchain": "go",
      "domains": { "dev": { "command": "go run ./cmd/server" } }
    }
  ]
}
```

实际文件使用严格 JSON，不接受注释和未知字段。

## 主要字段

| 字段 | 含义 |
|---|---|
| `version` | Manifest 版本，当前为 `1` |
| `workspace` | 稳定的工作区 `id` 与名称 `name` |
| `environments` | 环境名称和默认环境 |
| `domains.env` | 工作区环境变量来源：`dotenv` 或 `infisical`，可附带后端专属 `config` |
| `projects[]` | 项目名称、路径、模板、工具链，可选的 `packageManager` 和 `buildVersion` |
| `projects[].domains.env` | 项目覆盖项：`path`、`inherits`、`disabled` 和变量名 `keys`；后端继承工作区 |
| `projects[].domains.dev` | `one dev` 执行的 `command` |

Infisical 的 `domains.env.config` 可以包含 `projectId`、`projectName`、`rootPath` 和 `keys`。Manifest 不保存变量值或本机 Profile 名；凭据保存在本机 Profile，变量值交给 dotenv 或 Infisical。

## 谁会修改它

| 操作 | 修改内容 |
|---|---|
| `one create` | 创建工作区身份、默认环境、环境来源和空项目列表 |
| `one add` | 登记项目及其开发命令 |
| `one env set` | 登记变量名；Infisical 可初始化项目绑定 |
| `one env switch` | 修改环境变量来源 |
| `one serve` | 用户审阅后，经过 revision 校验保存项目配置或环境来源变更 |

`one build` 按工具链选择项目构建命令：Node 项目使用包脚本，Go 项目使用 `Taskfile.yml`。CI 通过 `one ci` 单独管理。

## 手动修改

重命名或删除项目时，应同时维护登记信息和磁盘目录。项目开发脚本变化后，更新 `projects[].domains.dev.command`。目录约定保持 `apps/`、`services/`、`packages/`。

如果清单与磁盘不一致，检查登记路径，恢复缺少的项目文件或修正登记项。业务变量值、依赖、缓存和构建产物不应写入 Manifest。

## 已移除的部署配置

`deploy` 和 `container` 域已下线。清单中仍有这些字段时返回 `MANIFEST_INVALID`，并提示手动删除对应字段。CLI 不提供迁移，也不会清理已有 Dockerfile、平台配置或 CI 文件。

无效 JSON 或未知字段同样返回 `MANIFEST_INVALID`。详见[错误码](/zh/docs/error-codes/)。
