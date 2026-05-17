---
title: one.manifest.json 是什么
description: 工作区台账文件 — 谁在写、什么时候改、漂移会发生什么。
---

每个 one cli 工作区根目录都会有一个 `one.manifest.json`。这一页讲它是干什么的、谁该改它、什么时候不该改。

**适合读这页的人**：第一次看到 manifest 文件不知道怎么处理的人；想搞懂工作区状态从哪里读的人。

**读完会**：理解 manifest 是工作区的**真源**；知道你（vs one cli）什么时候该改它。

## 一句话定义

`one.manifest.json` 是工作区的**台账**——记录这个工作区有哪些项目、用了什么模板、为每个 domain 选了哪个 backend（env / deploy / container）、有哪些环境。

它的存在 ≡ "这是一个 one cli 工作区"（其它命令通过它的存在判断当前目录是不是 one 工作区）。

## 它存了什么

下面用 `jsonc` 展示字段含义。真实的 `one.manifest.json` 仍然是标准 JSON，不能把 `//` 注释写进文件里。

```jsonc
{
  "version": 1, // One CLI 写入和读取的 manifest schema 版本
  "workspace": { // 工作区身份信息
    "id": "demo-app-2bb61e", // 工作区唯一 ID，创建时生成
    "name": "demo-app" // 工作区名，通常来自 one create 的目录名或 --name
  },
  "environments": { // 工作区支持的环境集合
    "names": ["dev", "staging", "prod"], // 可用环境名
    "default": "dev" // 不显式传 --env 时使用的默认环境
  },
  "domains": { // 工作区级 domain 默认配置
    "env": { // secrets / 环境变量后端
      "kind": "infisical", // env 后端：dotenv 或 infisical
      "config": { // 当前 env 后端自己的配置
        "keys": ["VITE_API_URL", "VITE_PUBLIC_SITE"], // 已声明的工作区级变量名；值不进 manifest
        "projectId": "86c73b57-5d1b-4f99-90dc-5d0c8ee0e823", // Infisical project id
        "projectName": "demo-app", // Infisical 里的项目名
        "rootPath": "/" // Infisical 里的根路径
      }
    },
    "deploy": { // 工作区级 deploy 默认配置
      "kind": "kustomize", // 默认 deploy 后端
      "config": { // kustomize 后端配置
        "namespace": "demo-app-2bb61e", // 可选 Kubernetes namespace；不写时使用 workspace.id
        "kustomizationPath": "kustomize/overlays/prod" // one deploy render/apply 读取的 Kustomize overlay 目录
      }
    }
  },
  "projects": [ // 工作区内的项目登记表
    {
      "name": "web", // 项目名；one env / one deploy -p 会用它定位项目
      "templateId": "nextjs-app", // 创建这个项目时使用的模板 ID
      "relativeDir": "apps/web", // 项目相对工作区根目录的位置
      "toolchain": "node", // 项目工具链：node / go 等
      "buildVersion": "0.1.0", // 默认构建版本；container / deploy 会读取
      "packageManager": "pnpm", // Node 项目使用的包管理器
      "domains": { // 项目级 domain 覆盖
        "container": {}, // 空对象表示启用容器构建并继承 profile 默认值
        "deploy": { "kind": "kustomize" }, // 这个项目使用 kustomize 部署
        "dev": { "command": "pnpm run dev" } // one dev 启动这个项目时执行的命令
      }
    },
    {
      "name": "spa", // 第二个项目：静态前端应用
      "templateId": "react-spa", // React SPA 模板
      "relativeDir": "apps/spa", // 前端项目目录
      "toolchain": "node", // Node 工具链
      "buildVersion": "0.1.0", // 当前默认构建版本
      "packageManager": "pnpm", // 使用 pnpm install 安装依赖
      "domains": { // 只覆盖这个项目需要不同于 workspace 默认值的部分
        "deploy": {
          "kind": "rustfs", // 这个项目走对象存储部署，而不是 workspace 默认 kustomize
          "config": { "bucket": "demo-app-2bb61e" } // 对象存储 bucket；未写时通常使用 workspace.id
        },
        "dev": { "command": "pnpm run dev" } // one dev 启动命令
      }
    },
    {
      "name": "api", // 后端服务
      "templateId": "go-api", // Go API 模板
      "relativeDir": "services/api", // Go 服务目录
      "toolchain": "go", // Go 工具链
      "buildVersion": "0.1.0", // 当前默认构建版本
      "domains": {
        "container": {}, // 启用容器构建
        "deploy": { "kind": "kustomize" }, // 这个服务部署到 Kubernetes
        "dev": { "command": "go run ./cmd/server" } // Go 项目的 one dev 启动命令
      }
    }
  ]
}
```

主要字段：

| 字段 | 含义 |
|---|---|
| `version` | One CLI 写入和读取的 manifest schema 版本 |
| `workspace` | 工作区身份（`id` + `name`）。`one create --env-provider infisical` 的自动绑定会用 `name` 命名 Infisical 项目 |
| `environments` | 环境名列表 + 默认环境。被 secrets backend、`one deploy --env`、每个 project 的 `domains.deploy.config.env` 三处共用 |
| `domains.env` / `.deploy` / `.container` | 工作区级 backend 选择：`{kind, config}`，按需出现。`kind` 是 bare backend 名（`dotenv` / `infisical` / `kustomize` / ...），`config` 是 kind-specific JSON blob |
| `projects[]` | **核心**——所有项目登记表；每项带自己的 `buildVersion`（默认 `0.1.0`）和可选的 `domains` override block |
| `projects[].domains.env` | 项目级 env override（path / inherits / disabled / keys），无 `kind`（继承 workspace） |
| `projects[].domains.container` | 项目级 container override（kind / image / namespace）。空对象表示这个项目启用容器构建，并使用本机 profile 解析链 |
| `projects[].domains.deploy` | 项目级 deploy backend，**有** `kind`（deploy 是真正按项目变种的：web → vercel、api → kustomize） |
| `projects[].domains.dev` | 项目级开发启动命令。`one dev` 读取 `command`，例如 Node 项目 `pnpm run dev`、Go 项目 `go run ./cmd/server` |

`domains.deploy.config.namespace` 是可选覆盖；不写时，Kubernetes namespace 默认使用 `workspace.id`（例如 `demo-app-2bb61e`）。只有你想把多个 workspace 放进同一个固定 namespace，或者想用 `demo-app-prod` 这类环境命名时，才需要显式写它。

> 设计要点：workspace 级和项目级都用 `domains` 包起来，词汇一致。env 在项目级只放 override；deploy 在项目级带 `kind`，因为不同项目可以走不同部署后端；container 项目级可以是空对象，也可以带 image / profile / namespace / kind 等覆盖项。

## 谁在写它

| 命令 | 改 manifest |
|---|---|
| `one create` | 创建初始 manifest，含 `workspace` 身份 + 空 `projects` 数组；同时写入 `domains.env.kind`、`environments`，并启用 always-on 的 CI / dev 约定 |
| `one add` | 给 `projects[]` 加一项；按模板的 `domains.<name>.default` 把每个 backend 写到 `projects[].domains.<name>`；同时写入 `projects[].domains.dev.command`，并跑 infra Sync 重对齐磁盘 |
| `one env set` | 把变量名记到 `domains.env.config.keys` 或 `projects[i].domains.env.keys`（值不进 manifest）；Infisical 未绑定时会触发 lazy auto-bind |
| `one container build` | 写回 `projects[i].domains.container.image`，并按需写 `domains.container.config.platform` |
| `one deploy --env <name>` | **不写** manifest；只把 `--env` 透传给当前 deploy 调用 |
| **你**（手工） | 极少；下面讲 |

## 什么时候你该手工改

90% 的情况你不需要碰它。剩下的少数场景：

<!-- verify-cli:ignore-start -->
1. **重命名项目**——目前没有 `one rename` 命令。手动改 manifest 里的 `name` + 改文件夹名，让 manifest 与磁盘对齐即可
2. **删除项目**——目前没有 `one remove`。手动从 `projects[]` 里删掉对应条目，删掉它的文件夹
<!-- verify-cli:ignore-end -->
3. **切换 deploy 后端**——比如把 `web` 从 `aws-s3` 改到 `vercel`：编辑 `projects[i].domains.deploy.kind`，并用 `one configure use deploy/vercel --profile <name>` 切 default profile
4. **调整本地开发命令**——比如把 `projects[i].domains.dev.command` 从 `pnpm run dev` 改成项目自己的启动脚本。`one dev` 以 manifest 为准，不会自动追踪后续 `package.json` 变化

> 这些字段由 One CLI 维护，不需要手改：
> - `workspace.roots`：永远是 `apps/services/packages`，写死代码
> - `ai.providers`：所有 provider 默认全启用（当前 codex + claude-code，自动同时生成 AGENTS.md + CLAUDE.md）
> - 顶层 `ci` / `dev`：永远启用，不再 opt-in；删掉这两个字段或加上都不会被读。项目级 `projects[].domains.dev.command` 仍然会被 `one dev` 读取

## 漂移会怎样

"漂移" = manifest 里写的和文件系统真实状态不一致。常见场景：

- 你手工删了 `services/user-api/` 文件夹，但忘了改 manifest
- 你 git pull 拉到了同事加的项目，但本地 manifest 没刷
- 模板生成时某些步骤失败，项目登记了但 `ready: false`

可以重新跑相关 per-domain 命令（`one add` / `one container build` / `one deploy render` 等）来重对齐；它们会读 manifest 并报告自己缺什么。

## 不改 manifest 的事

不要把这些放进 manifest：

- 业务运行时配置（数据库连接、API URL 等）→ 用 `.env` + Secrets
- 项目自己的依赖 / 脚本 → 在项目自己的 `package.json` 里
- 用户偏好（编辑器配置）→ 不在工作区级别管
- 临时状态（构建产物、缓存）→ gitignore

## 校验

`one.manifest.json` 有 schema 校验。格式坏了会冒泡 `MANIFEST_INVALID`；缺失或空会冒泡 `MANIFEST_MISSING_OR_EMPTY`，remediation 指向 `one create` / `one add`。

详见 [错误码大全](/zh/docs/error-codes/) 的 `MANIFEST_*` 章节。

## 想看实例

跑 `one create my-app && cat my-app/one.manifest.json` 就能看到一个新工作区 manifest：里面会有 `workspace` 身份、默认环境集合、env 后端选择和空 `projects` 数组。再加一个项目：

```bash
cd my-app
one add nestjs-api --name api
cat one.manifest.json
```

看 `projects[]` 多了一条，`projects[0].domains.{container,deploy}` 按 nestjs-api 模板的 defaults 自动填好。
