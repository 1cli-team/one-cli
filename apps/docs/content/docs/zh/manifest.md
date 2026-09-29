---
title: one.manifest.json 是什么
description: 工作区的项目登记表、环境变量来源和本地开发配置。
---

每个 One CLI 工作区根目录都有 `one.manifest.json`。它记录工作区身份、项目路径、工具链、环境和环境变量来源，供命令定位项目并选择执行方式。

## 示例

```json
{
  "version": 1,
  "workspace": {
    "id": "demo-app-2bb61e",
    "name": "demo-app"
  },
  "environments": {
    "names": [
      "dev",
      "preview",
      "prod"
    ],
    "default": "dev"
  },
  "projects": [
    {
      "name": "web",
      "templateId": "react-spa",
      "relativeDir": "apps/web",
      "toolchain": "node",
      "buildVersion": "0.1.0",
      "packageManager": "pnpm",
      "env": {
        "path": "/apps/web",
        "inherits": true,
        "keys": [
          "API_URL"
        ]
      },
      "service": {
        "url": "http://localhost:5173"
      }
    },
    {
      "name": "api",
      "templateId": "go-api",
      "relativeDir": "services/api",
      "toolchain": "go"
    }
  ],
  "env": {
    "siteUrl": "https://app.infisical.com",
    "projectId": "your-project-id",
    "rootPath": "/"
  }
}
```

实际文件使用严格 JSON，不接受注释和未知字段。

## 主要字段

| 字段 | 含义 |
|---|---|
| `version` | Manifest 版本，当前为 `1` |
| `workspace` | 稳定的工作区 `id` 与名称 `name` |
| `environments` | 环境名称和默认环境 |
| `env` | 可选 Infisical 绑定：`siteUrl`、`projectId`、`projectName`、`rootPath` 和 `keys` |
| `projects[]` | 项目名称、路径、模板、工具链，可选的 `packageManager` 和 `buildVersion` |
| `projects[].env` | 项目覆盖项：`path`、`inherits`、`disabled` 和变量名 `keys`；后端继承工作区 |
| `projects[].service.url` | Dashboard 可选本机访问地址；执行命令由 mise 任务定义 |

Infisical 的 `env` 可以包含 `projectId`、`projectName`、`rootPath` 和 `keys`。Manifest 不保存变量值或本机 Profile 名；凭据保存在系统 keyring，变量值交给 Infisical。

## 谁会修改它

| 操作 | 修改内容 |
|---|---|
| `one create` | 创建工作区身份、默认环境、环境来源和空项目列表 |
| `one add` | 登记项目元数据 |
| `one env set` | 登记变量名；Infisical 可初始化项目绑定 |
| `one serve` | 用户审阅后，经过 revision 校验保存项目配置或环境来源变更 |

`one run` 读取已经存在的 mise 任务。创建工作区、添加项目或显式执行 `one init mise` 时，将包脚本和 Taskfile 任务投影为原生 mise 命令；执行任务不会重新生成配置。

## 手动修改

重命名或删除项目时，应同时维护登记信息和磁盘目录。项目命令变化后，更新对应的 mise 任务。目录约定保持 `apps/`、`services/`、`packages/`。

如果清单与磁盘不一致，检查登记路径，恢复缺少的项目文件或修正登记项。业务变量值、依赖、缓存和构建产物不应写入 Manifest。

## 已移除的部署配置

`deploy` 和 `container` 域已下线。清单中仍有这些字段时返回 `MANIFEST_INVALID`，并提示手动删除对应字段。CLI 不提供迁移，也不会清理已有 Dockerfile、平台配置或 CI 文件。

无效 JSON 或未知字段同样返回 `MANIFEST_INVALID`。详见[错误码](/zh/docs/error-codes/)。

## 从 domains 迁移

不再接受 `domains` 包装层。将工作区 Infisical 的 `domains.env.config` 提到顶层 `env`，项目的 `domains.env` 提到项目的 `env`。开发命令移入 mise 任务，本机访问地址移入 `service.url`，移除 `kind` 和 `config` 包装。原 dotenv 工作区删除旧绑定及本地文件 `path` 覆盖，需要时再绑定 Infisical。

旧清单返回 `MANIFEST_INVALID` 并提示迁移。One CLI 不会自动改写清单，也不会导入或删除已有 `.env` 文件；需要的值应显式写入 Infisical。env 的 `pull`、`switch` 子命令及 `--env-provider` 参数已删除。旧 preset 代码 `d` 保留占位但不再接受，请使用 `i` 或省略环境段。

## 已停用的 dev 配置

旧 `projects[].dev.command` 不再执行。读取旧 `dev.url` 时兼容为 `service.url`；`one init mise --dry-run` 可预览迁移，`one init mise` 会删除旧 dev 对象并保留 URL。Manifest 继续使用 JSON。
