---
title: one configure
description: 管理部署、环境变量和镜像仓库所需的本机连接与偏好设置。
---

`one configure` 管理**本机连接和偏好设置**；`one configure mise` 生成工作区工具配置，`one configure hooks` 生成 hk 检查并安装本地 Git 启动器。连接密钥只保存在本机，不写入工作区或 Git。

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
one configure open
one configure mise [--dry-run] [--node-version <version>] [--go-version <version>]
one configure hooks [--dry-run]
```

没有连接时，无参 `one configure` 进入建立连接向导；已有连接时显示简洁概览。`show` / `use` / `remove` 在终端可直接选择已有连接；脚本仍显式传 `<pair>` 和 `--profile`。

## hooks 工作区提交检查

新工作区已经配置 hk。克隆后运行 `one configure hooks` 安装当前 checkout 的钩子；旧工作区可先用 `one configure hooks --dry-run -o json` 预览迁移。配置过程不下载工具，保留用户自定义检查；具体规则和迁移限制见 [`one hk`](/zh/docs/hk/)。

## 交互模式

本地人工配置推荐用交互式向导：

```bash
one configure
one configure add
```

向导先选择要连接的服务，再询问连接名称和该服务需要的字段。自动化命令中继续使用稳定服务 ID；敏感字段使用密码式输入。

脚本和 CI 不应等待交互式向导；请显式传服务 ID、连接名称（`--profile`）和服务参数。

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
2. `profile-bindings.json` 中的 Project + environment 绑定
3. `profile-bindings.json` 中的 Workspace + environment 绑定
4. `config.json#workspaces` 中的旧 Project 绑定
5. `config.json#workspaces` 中的旧 Workspace 绑定
6. `~/.config/one/config.json` 里对应 `domain/backend.default`

环境绑定按规范化 Workspace root、environment 和 `(domain, backend)` 定位，只保存 Profile 名。Dashboard UI 通过 `?env=` 只提供 `dev`、`preview`、`prod`；核心/API 也接受其他工作流传入的安全自定义 ID。全局 Settings 中的 Profile CRUD 不按环境分区。空环境保持旧解析链。

`one.manifest.json` 永远不保存本机 Profile 名。`one configure use ... --workspace` 和 `--project` 作为旧绑定仍兼容；需要每个环境不同选择时使用 `one serve`。

同名 profile 可以存在于不同 backend 下，例如 `deploy/aws-s3` 和 `deploy/kustomize` 都可以有 `prod`。

## 存储位置

```text
~/.config/one/
├── config.json             # Profile 非敏感字段、default、旧绑定
├── credentials.json        # 敏感字段：clientSecret、accessKeySecret、password
├── profile-bindings.json   # v1：规范化 root + environment -> Profile 名
└── cache/                  # 短期 token 缓存
```

三个 JSON 文件都是 mode `0600` 的机器本地文件；`profile-bindings.json` 只含名字。它们都不会修改或升级 `one.manifest.json`。`show` 默认掩码敏感字段，只有 `show --reveal` 会输出明文。

## mise 工作区工具配置

新建 workspace 会自动生成 mise 配置，添加项目时自动更新。日常仍使用原来的命令：

```bash
one create my-app -y
cd my-app
one add react-spa --name web -y
one dev web
one run web -- pnpm build
```

旧 workspace 可一次性生成配置，之后也使用同样的日常命令：

```bash
one configure mise --dry-run -o json
one configure mise
```

预览返回每个文件的 `before` / `after`，不执行 mise、不联网、不读取项目密钥。实际写入仅涉及根目录和各项目的 `.mise/conf.d/one.toml`，这些生成文件可纳入 Git。Manifest schema 保持 v1。

根配置固定 Node 版本（默认 `24.15.0`，再次生成沿用之前的版本），包管理器版本来自根 `package.json#packageManager`；Go 项目使用 `go.mod` 的 `go` / `toolchain` 声明。需要调整时用 `--node-version` / `--go-version` 指定完整版本。Go override 不能低于 `go.mod` 最低要求；自定义 Node engines 的兼容性需自行确认，当前尚未解析完整 npm 版本范围。

One 保留用户的 `mise.toml` 和自定义任务；同目录的 `mise.toml` 可以覆盖生成默认值。手改生成文件会触发 `MISE_CONFIG_CONFLICT`，应将定制内容移入用户配置，再恢复生成文件。已有冲突会在 `one add` 渲染项目之前报告；若后续磁盘写入失败，项目保留，修复错误后运行 `one configure mise` 完成配置。

根据项目已有能力生成 `one:dev`、`one:build`、`one:test`、`one:lint`。这些任务执行时读取当前 Manifest、package scripts 或 Taskfile；额外命令参数继续通过 `one run <project> -- <cmd> [args...]` 传递。按需使用 `one mise` 访问配置信任、诊断和任务命令，无需单独安装 mise。手动运行 mise 任务时，PATH 中的 One 必须支持生成配置的执行协议；也可设置 `ONE_BINARY_PATH` 为测试版 One 的绝对路径。

`ONE_RUNTIME=builtin` 可用于临时诊断，让 `one run` / `one dev` 使用机器现有工具；该模式不提供 mise 环境。移除该变量即可恢复自动选择。没有根生成配置的旧 workspace 默认使用 builtin，不会静默迁移。

此轮仅接入 `run`、`dev` 和创建流程。CI、部署前构建、工具锁文件生成和跨平台工具安装矩阵留待下一阶段；它们目前仍沿用原有实现。工具的精确版本声明不等于完整的跨平台 `mise.lock`。

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
| `mise` | `one-cli/mise-config/v1` |

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `PROFILE_NONE_CONFIGURED` | 先跑 `one configure add <pair> --profile <name> --use` |
| `PROFILE_NOT_FOUND` | `one configure list <pair>` 看本机已有 profile |
| `PROFILE_BACKEND_INVALID` | 确认 profile 所在 backend 与目标 project 的 deploy/container backend 一致 |
| `PROFILE_FILE_INVALID` | 修复错误 context 指向的文件（`config.json`、`credentials.json` 或 `profile-bindings.json`） |
| `PROFILE_VERSION_UNSUPPORTED` | 升级 One CLI，或只重建不兼容的机器本地文件 |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [`one serve`](/zh/docs/serve/) — 编辑 Profile 并选择环境感知的本机绑定
- [`one env`](/zh/docs/env-vars/) — 使用 `env/infisical` profile
- [`one deploy`](/zh/docs/deploy/) — 使用 deploy profile
- [`one container`](/zh/docs/container/) — 使用 container profile
