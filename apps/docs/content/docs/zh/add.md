---
title: one add
description: 往工作区里加一个模板化项目。
---

`one add` 从模板注册表选一个模板写入工作区，登记到 manifest，并按模板的 `defaults` 同步 Dockerfile / Kustomize / workflow 等产物。

有两条入口：

- 人类第一次用：直接跑 `one add`，在交互式选择器里选模板分类、模板，并输入项目名。
- 脚本 / 已知模板：先跑 `one templates` 看模板 ID，再执行 `one add <template-id> --name <project-name>`。

`template-id` 是模板 ID，例如 `nestjs-api` / `nextjs-app` / `ts-library`，不是项目名；项目名由 `--name` 决定。

## 用法

```bash
one add [template-id] --name <project-name> [--deploy-provider <backend>] [options]
```

## 参数

| 参数 | 说明 |
|---|---|
| `template-id` | 模板 ID（如 `nestjs-api`）；不传走交互式选择 |
| `-n, --name` | 项目名（必填，非交互模式） |
| `-y, --yes` | 非交互模式 |
| `--deploy-provider <backend>` | 显式选择 deploy 后端（必须在模板的 compat 列表里） |
| `-o, --output <fmt>` | `json` / `yaml` / `text` |

工作区根用 pnpm；项目自身的工具链由模板决定（Node 模板用 pnpm，Go 模板用 Go toolchain，等等）。

## 交互模式

直接运行 `one add` 会进入终端交互式选择器，适合第一次使用或不确定模板 ID 时使用。它会依次询问模板分类、模板、项目名；如果模板支持多个 deploy 后端，还会询问本项目使用哪个 deploy 后端。

非交互场景要显式传模板 ID 和项目名：

```bash
one add nestjs-api --name api --yes
```

## 输出

```json
{
  "schema": "one-cli/add/v1",
  "subproject_name": "user-api",
  "target_path": "/abs/path/my-app/services/user-api",
  "template_id": "nestjs-api",
  "toolchain": "node",
  "package_manager": "pnpm",
  "ai_guides": {
    "status": "completed",
    "providers": ["codex", "claude-code"],
    "generated_files": [
      "AGENTS.md",
      "CLAUDE.md",
      ".one/agents/conventions.md",
      ".one/agents/projects/services-user-api.md",
      ".one/agents/ops/dev.md",
      ".one/agents/ops/secrets.md",
      ".one/agents/ops/container.md",
      ".one/agents/ops/deploy.md"
    ],
    "file_count": 8
  }
}
```

`warnings[]` 存在时表示模板兼容性或后置同步有非阻断提示；项目仍然加成功。`ai_guides.status` 表示根目录 `AGENTS.md`、`CLAUDE.md` 和 `.one/agents/**` 是否刷新成功。`ai_guides.generated_files` 是工作区相对路径。

## 示例

### 交互（人类）

```bash
cd my-app
one add
```

这个流程会依次询问：

1. 模板分类（Frontend / Backend / Library）
2. 具体模板（比如 `nestjs-api`）
3. 项目名（比如 `api`）

不确定模板 ID 时，用这一种最稳。

### 先看模板，再显式添加

```bash
one templates
one add nestjs-api --name api
```

`one templates` 列出的 `id` 就是 `one add` 后面的第一个参数。

### 非交互（CI / agent）

```bash
one add nestjs-api      --name user-api --yes
one add nextjs-app  --name web      --yes
one add ts-library    --name shared   --yes
```

### Agent 调用（拿 JSON）

```bash
one add nestjs-api --name user-api --yes -o json | jq
```

## 加完会自动做的事

- 把项目登记到 `one.manifest.json#projects[]`
- 写入 `projects[].domains.{container,deploy}` 字段（按模板的 `domains.<name>.default`）
- `container/docker` 项目根加 `Dockerfile`
- `deploy/kustomize` 工作区根加 `kustomize/base` 和 `kustomize/overlays/{dev,staging,prod}`
- S3 兼容 deploy 后端不写本地部署产物；部署时使用 `one configure add deploy/aws-s3 --profile <name>` 或其它拆分后的 S3 后端（`deploy/aliyun-oss`、`deploy/r2` 等）配置的对象存储 profile
- 加 GitHub Actions workflow 条目
- 刷 `AGENTS.md`、`CLAUDE.md` 和 `.one/agents/**`

如果有失败的 step（比如 agent 文档刷新失败），项目仍然加成功，只是相关字段会标 `failed` / `skipped`。

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `TEMPLATE_NOT_FOUND` | 模板 ID 错；context 里有 `available_templates`，挑一个用 |
| `TEMPLATE_REQUIRED` | 非交互场景没传 template-id；显式传一个 |
| `INVALID_NAME` | `--name` 不符合 `^[a-zA-Z0-9][a-zA-Z0-9_-]*$` |
| `SUBPROJECT_NAME_REQUIRED` | 非交互模式必须传 `--name` |
| `TARGET_EXISTS` | 项目目录已存在；换 `--name` |
| `NOT_ONE_PROJECT` | cwd 不是工作区；先 `one create <dir>`，或 `cd` 到已有工作区 |
| `REGISTRY_FETCH_FAILED` | 网络问题；查 context 里的 registry url |
| `AI_GUIDE_EXISTS` | 工作区根已有用户自管的 `AGENTS.md` / `CLAUDE.md`，刷不动 |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 模板选择

不知道选哪个？看 [模板决策树](/zh/docs/templates/)。

## 加完之后

- 检查 `one.manifest.json#projects[]` 确认项目登记
- Agent 文档、容器 / 部署 artefacts 都已由 `one add` 在执行过程中同步完成
- `one add` 不自动安装依赖：JS / TS 工作区在根目录跑 package manager install；Go 项目进项目目录跑 `go mod download`，修改 imports 或需要修复模块元数据时再跑 `go mod tidy`
