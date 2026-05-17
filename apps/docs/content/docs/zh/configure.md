---
title: one configure
description: 管理机器级 endpoint profile：Infisical、对象存储、Kubernetes、Vercel、Cloudflare、EdgeOne 和 Docker registry。
---

`one configure` 管的是**本机 profile**，不是某个 workspace 的业务配置。profile 保存 endpoint、账号和凭据，供 `one env`、`one container`、`one deploy`、`one run` 读取。

## 用法

```bash
one configure
one configure add
one configure add <pair> --profile <name> [backend flags...] [--use]
one configure list [pair]
one configure current [pair]
one configure show <pair> --profile <name> [--reveal]
one configure use <pair> --profile <name>
one configure remove <pair> --profile <name>
one configure locale [auto|zh-CN|en-US]
```

无参 `one configure` 和 `one configure add` 会打开交互式向导；脚本、CI、agent 应显式传 `<pair>`、profile 名和对应 backend flags。

## 交互模式

本地人工配置推荐用交互式向导：

```bash
one configure
one configure add
```

向导会先让你选择要配置的 `(domain, backend)`，例如 `env/infisical`、`deploy/aws-s3`、`container/docker`，再逐项询问 profile 名、endpoint、token、ak/sk、kubeconfig 等字段。敏感字段会以密码输入方式录入。

脚本、CI、agent 不应该等待交互式向导；请显式传 pair、profile 名和 backend 参数。

## 支持的 pair

| pair | 用途 |
|---|---|
| `env/infisical` | Infisical site URL + Universal Auth client id / secret |
| `deploy/aliyun-oss` | 阿里云 OSS |
| `deploy/tencent-cos` | 腾讯云 COS |
| `deploy/aws-s3` | AWS S3 |
| `deploy/minio` | 自部署 MinIO |
| `deploy/rustfs` | 自部署 RustFS |
| `deploy/r2` | Cloudflare R2 |
| `deploy/kustomize` | Kubernetes kubeconfig + context |
| `deploy/vercel` | Vercel API token |
| `deploy/cloudflare` | Cloudflare API token |
| `deploy/edgeone` | Tencent EdgeOne Pages API token |
| `container/docker` | 通用 Docker registry host、namespace、username、password |
| `container/dockerhub` | Docker Hub username、password/token、namespace |
| `container/ghcr` | GitHub Container Registry username、PAT、namespace |
| `container/acr` | 阿里云 ACR region、username、password/token、namespace |

`env/dotenv` 不需要 profile；它用于本地 `.env` 工作流。S3 兼容 deploy 后端共用一组 profile 字段，但每个供应商都有自己的 backend ID。

## 常用示例

```bash
one configure add env/infisical --profile work \
  --client-id "$INFISICAL_CLIENT_ID" \
  --client-secret "$INFISICAL_CLIENT_SECRET" \
  --use

one configure add deploy/aws-s3 --profile web-prod \
  --region us-east-1 \
  --access-key-id "$AWS_ACCESS_KEY_ID" \
  --access-key-secret "$AWS_SECRET_ACCESS_KEY" \
  --use

one configure add deploy/kustomize --profile prod-k8s \
  --kubeconfig ~/.kube/config \
  --kubeconfig-context prod \
  --use

one configure add container/ghcr --profile ghcr \
  --namespace "$GITHUB_USER" \
  --username "$GITHUB_USER" \
  --password "$GHCR_PAT" \
  --use
```

## profile 解析顺序

命令实际使用 profile 时按这个顺序找：

1. 命令行 `--profile <name>`
2. `one.manifest.json` 里的 project / workspace profile pin
3. `~/.config/one/config.json` 里对应 `domain/backend.default`

同名 profile 可以存在于不同 backend 下，例如 `deploy/aws-s3` 和 `deploy/kustomize` 都可以有 `prod`。

## 存储位置

```text
~/.config/one/
├── config.json         # 非敏感字段：endpoint、region、default 指针
├── credentials.json    # 敏感字段：clientSecret、accessKeySecret、password
└── cache/              # 短期 token 缓存
```

两个 JSON 文件都是 `0600`。`show` 默认掩码敏感字段，只有 `show --reveal` 会输出明文。

## 输出 schema

| 命令 | schema |
|---|---|
| `add` | `one-cli/configure-add/v1` |
| `list <pair>` | `one-cli/configure-list/v1` |
| `list` | `one-cli/configure-list-all/v1` |
| `current <pair>` | `one-cli/configure-current/v1` |
| `current` | `one-cli/configure-current-all/v1` |
| `show` | `one-cli/configure-show/v1` |
| `use` | `one-cli/configure-use/v1` |
| `remove` | `one-cli/configure-remove/v1` |

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `PROFILE_NONE_CONFIGURED` | 先跑 `one configure add <pair> --profile <name> --use` |
| `PROFILE_NOT_FOUND` | `one configure list <pair>` 看本机已有 profile |
| `PROFILE_BACKEND_INVALID` | 确认 profile 所在 backend 与目标 project 的 deploy/container backend 一致 |
| `PROFILE_FILE_INVALID` | 手工修复或删除 `~/.config/one/config.json` / `credentials.json` 后重建 |
| `PROFILE_VERSION_UNSUPPORTED` | 旧格式配置不兼容，按当前 `(domain, backend)` 重新配置 |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [`one serve`](/zh/docs/serve/) — 用本地 Web UI 手工编辑这些 profile
- [`one env`](/zh/docs/env-vars/) — 使用 `env/infisical` profile
- [`one deploy`](/zh/docs/deploy/) — 使用 deploy profile
- [`one container`](/zh/docs/container/) — 使用 container profile
