---
title: one container
description: 查看、构建、推送项目 Dockerfile 镜像。
---

`one container` 操作 `one.manifest.json#projects[].domains.container` 声明的项目。当前容器后端是 Dockerfile-driven：模板提供 Dockerfile，One CLI 负责解析项目、推断镜像名、拼 registry tag，并调用 Docker。

## 用法

```bash
one container info
one container build [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
one container push  [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
```

`[subproject]` 和 `-p / --project` 都可选一个项目，支持 manifest 里的 `name` 或 `relativeDir`。不传时对所有启用 container 的项目执行。

## 交互模式

`one container info` 和 `one container push` 不打开交互式向导。`one container build` 在 TTY 下如果没有传 `--build-version`，且无法从 manifest / Git / 项目元数据稳定推断版本，会让你选择镜像版本或输入自定义版本。

脚本、CI、agent 应显式传 `--build-version` 和 `--profile`，或先用 `--dry-run` 看将执行的 Docker 命令。

## info

只读检查每个可构建项目的 Dockerfile、workload name 和 image override：

```bash
one container info -o json
```

输出 schema：`one-cli/container-info/v2`。

## build

```bash
one container build api
one container build -p services/api --build-version v0.1.0
one container build --dry-run
```

默认构建本地 tag：`<workload>:<version>`。当传 `--profile`，或 manifest 里 pin 了 container profile 时，会拼出 registry-qualified tag：`<registry>/[namespace/]<workload>:<version>`，并在有 username/password 时先执行 `docker login`。

`--build-version` 是非交互 / CI 用版本号。TTY 模式没传版本时，CLI 会从 manifest、Git 或项目元数据推断，必要时提示选择。

输出 schema：`one-cli/container-build/v2`。

## push

```bash
one container push api --profile ghcr
one container push -p apps/web --build-version v0.1.0 --dry-run
```

`push` 必须能解析到项目所用 kind 的 container profile（例如 `container/ghcr`、`container/dockerhub`、`container/acr` 或通用 `container/docker`）。若 registry tag 不在本地，但匹配的本地裸 tag 存在，CLI 会先 `docker tag <workload>:<version> <registry>/.../<workload>:<version>`，再推送。

输出 schema：`one-cli/container-push/v1`。

## profile 解析

`build` / `push` 使用 Docker registry profile 的顺序：

1. `--profile <name>`
2. `~/.config/one/config.json#workspaces[workspaceId].projects[project].profiles[container/kind]`
3. `~/.config/one/config.json#workspaces[workspaceId].profiles[container/kind]`
4. `~/.config/one/config.json#container/<kind>.default`

配置一次即可复用：

```bash
one configure add container/ghcr --profile ghcr \
  --namespace "$GITHUB_USER" \
  --username "$GITHUB_USER" \
  --password "$GHCR_PAT" \
  --use
```

支持的 kind 包括 `container/docker`（通用 registry）、`container/dockerhub`、`container/ghcr` 和 `container/acr`。

## manifest 条件

`nestjs-api`、`go-api`、`nextjs-app` 模板默认启用 `container/docker`。库、移动端、Electron 默认不启用 container；这些项目跑 `one container` 会被跳过。

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `BACKEND_NOT_ENABLED` | 当前 workspace 没有项目声明 container backend；换模板或补 manifest |
| `REGISTRY_CREDENTIAL_MISSING` | 先 `one configure add container/<kind> --profile <name> --use` |
| `IMAGE_TAG_NOT_FOUND` | push 前先 build，或显式传同一个 `--build-version` |
| `CONTAINER_BUILD_FAILED` | 进入项目目录直接跑 Dockerfile 构建命令看完整日志 |

完整码表：[错误码大全](/zh/docs/error-codes/)。

## 进一步阅读

- [构建与推送镜像](/zh/tutorials/container-build-push/) — 端到端流程
- [`one configure`](/zh/docs/configure/) — 配置 `container/docker` profile
- [`one deploy`](/zh/docs/deploy/) — kustomize 部署会自动构建 / 推送镜像
