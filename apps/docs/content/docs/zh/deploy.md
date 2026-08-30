---
title: one deploy
description: 按项目派发到 kustomize、S3-compatible、Vercel、Cloudflare 或 EdgeOne 部署后端。
---

`one deploy` 是按项目部署入口。第一次部署时选择兼容的部署目标和本机连接，后续部署复用项目配置。

## 用法

```bash
one deploy [project] [--provider <target>] [--profile <connection>] [--env <env>] [--dry-run]
```

## 参数

| 参数 | 说明 |
|---|---|
| 位置参数 `project` | 只部署一个项目；支持 manifest 里的 `name` 或 `relativeDir` |
| `-p, --project <name|path>` | 为旧脚本和 CI 保留的选择参数 |
| `--provider <target>` | 自动化首次部署时显式指定目标 |
| `--profile <name>` | 本次使用指定本机连接 |
| `--env <env>` | 覆盖部署目标环境，同时作为环境变量注入环境 |
| `--env-provider dotenv|infisical` | 覆盖 workspace manifest 里选择的 env provider |
| `--build-version <version>` | 非交互 / CI 用镜像版本；主要用于 kustomize 自动构建 |
| `--dry-run` | 打印 docker / kubectl / 对象存储 / 平台 CLI 计划，不触碰远端 |

## 交互模式

尚未配置部署的项目在 TTY 下依次询问：

1. 部署哪个项目（未传时）
2. 部署目标类别
3. 当前版本已实现且与技术栈兼容的具体服务
4. 缺少本机连接时选择“现在配置”或“稍后配置”

选择“稍后配置”会以 0 退出、不修改工作区，并打印准确恢复命令。脚本使用 `one deploy <project> --provider <target> --profile <connection>`。

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
2. `profile-bindings.json` 中 Project + environment 的 `deploy/<backend>` 绑定
3. `profile-bindings.json` 中 Workspace + environment 的 `deploy/<backend>` 绑定
4. `config.json#workspaces` 中的旧 Project 绑定
5. `config.json#workspaces` 中的旧 Workspace 绑定
6. `~/.config/one/config.json#deploy/<backend>.default`

环境感知的 key 使用规范化 Workspace root。deploy 解析使用 Project 已配置的部署环境，没有时回退 `prod`；单次覆盖使用 `--profile`。在 `one serve` 中选择每个环境的 Workspace/Project Profile 名。Dashboard 环境来自 `?env=`，切换它不会改动部署环境或 Manifest。

manifest 永远不保存本机 Profile 名。`one configure use <pair> --profile <name> --workspace` 和 `--project <name|path>` 仍可创建兼容的无环境旧绑定。

## 示例

```bash
one deploy --dry-run
one deploy web --env staging --dry-run
one deploy api --provider kustomize --profile prod-k8s --build-version v0.1.0
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
| `BACKEND_NOT_ENABLED` | 非交互调用请显式指定项目和部署目标 |
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
