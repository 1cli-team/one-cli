---
title: one deploy
description: 按项目派发到 kustomize、S3-compatible、Vercel、Cloudflare 或 EdgeOne 部署后端。
---

`one deploy` 是 per-project 部署入口。它读取每个项目的 `projects[].domains.deploy.kind`，再派发给对应后端：后端 / SSR 常见是 `kustomize`，静态前端常见是 S3-compatible 对象存储，也可以使用 Vercel、Cloudflare、EdgeOne。

## 用法

```bash
one deploy [-p <name|path>] [--profile <name>] [--env <env>] [--env-provider dotenv|infisical] [--build-version <version>] [--dry-run]
```

## 参数

| 参数 | 说明 |
|---|---|
| `-p, --project <name|path>` | 只部署一个项目；支持 manifest 里的 `name` 或 `relativeDir` |
| `--profile <name>` | 本次部署临时使用指定 deploy profile |
| `--env <env>` | 覆盖部署目标环境，同时作为环境变量注入环境 |
| `--env-provider dotenv|infisical` | 覆盖 workspace manifest 里选择的 env provider |
| `--build-version <version>` | 非交互 / CI 用镜像版本；主要用于 kustomize 自动构建 |
| `--dry-run` | 打印 docker / kubectl / 对象存储 / 平台 CLI 计划，不触碰远端 |

## 交互模式

`one deploy` 不是完整向导，但 TTY 下有少量补全式询问：

- kustomize 部署需要镜像版本且没有传 `--build-version` 时，会沿用 `one container build` 的版本选择逻辑。
- Cloudflare 部署缺少可用 profile，且你没有显式传 `--profile` 时，可能会询问 API token / account ID 并保存一个默认 profile。

脚本、CI、agent 应显式传 `--profile`、`--env`、`--build-version`，并先用 `--dry-run` 确认计划。

## 后端

| backend | 适合项目 | 行为 |
|---|---|---|
| `kustomize` | API、SSR、需要容器的服务 | 自动 build / push 镜像，同步 overlay，然后 `kubectl apply -k` |
| `aws-s3` / `aliyun-oss` / `tencent-cos` / `minio` / `rustfs` / `r2` | 静态站 | 构建产物，确保 bucket，走 S3-compatible 协议上传 |
| `vercel` | 前端托管 | 调 Vercel CLI/API 部署 |
| `cloudflare` | Cloudflare Workers | 调 `wrangler deploy` |
| `edgeone` | EdgeOne Pages | 调 `edgeone pages deploy` |

## 环境映射

| backend | `prod` 或空 | 其他环境 |
|---|---|---|
| `kustomize` | `kustomize/overlays/prod` | `kustomize/overlays/<env>` |
| `vercel` | production deploy | preview deploy |
| `cloudflare` | `wrangler deploy` | `wrangler deploy --env=<env>` |
| `edgeone` | production deploy | preview deploy |
| S3-compatible | deploy 目标不变 | deploy 目标不变；只影响构建时注入的 env |

`--env` 必须存在于 `one.manifest.json#environments.names`。

## profile 解析

每个 deploy target 独立解析 profile：

1. `--profile <name>`
2. `~/.config/one/config.json#workspaces[workspaceId].projects[project].profiles[deploy/backend]`
3. `~/.config/one/config.json#workspaces[workspaceId].profiles[deploy/backend]`
4. `~/.config/one/config.json#deploy/<backend>.default`

manifest 不再保存本机 profile 名。用 `one configure use <pair> --profile <name> --workspace` 绑定当前工作区；需要只绑定某个项目时加 `--project <name|path>`。

## 示例

```bash
one deploy --dry-run
one deploy -p web --env staging --dry-run
one deploy -p api --profile prod-k8s --build-version v0.1.0
```

## 输出 schema

deploy 输出 schema 按 provider 分开：

| backend | schema |
|---|---|
| `kustomize` | `one-cli/deploy-apply/v1` |
| S3-compatible | `one-cli/deploy-apply/v1` |
| `vercel` | `one-cli/deploy-apply-vercel/v1` |
| `cloudflare` | `one-cli/deploy-apply-cloudflare/v1` |
| `edgeone` | `one-cli/deploy-apply-edgeone/v1` |

dry-run 会优先打印将执行的命令行，适合 CI 或上线前确认。

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `BACKEND_NOT_ENABLED` | 目标项目没有 deploy backend；换模板或补 `projects[].domains.deploy` |
| `PROFILE_NOT_FOUND` | `one configure list deploy/<backend>` 看本机已有 profile |
| `PROFILE_NONE_CONFIGURED` | 先 `one configure add deploy/<backend> <name> --use` |
| `ENV_UNKNOWN_ENVIRONMENT` | 把环境名加入 `manifest.environments.names`，或换成已有环境 |
| `REGISTRY_CREDENTIAL_MISSING` | kustomize 自动构建前先配置 `container/docker` profile |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [第一次部署](/zh/tutorials/deploy/) — 部署一个项目
- [多 backend 部署](/zh/tutorials/deploy-multi-backend/) — 多后端、多项目、多环境
- [`one configure`](/zh/docs/configure/) — 配置 deploy profile
- [`one container`](/zh/docs/container/) — 镜像构建 / 推送细节
