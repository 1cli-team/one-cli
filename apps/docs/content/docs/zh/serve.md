---
title: one serve
description: 本地 Web UI 编辑机器级 profile —— 凭据手工录入，AI 不接触。
---

`one serve` 启动一个仅监听 `127.0.0.1` 的小型 HTTP 服务，并自动打开浏览器，让你在 Web 表单里编辑 `~/.config/one/config.json` 与 `~/.config/one/credentials.json` —— 也就是 `one configure` 管理的 `(domain, backend)` profile section（Infisical 凭据、S3 ak/sk、kubeconfig context、registry token、dotenv 名字）。

为什么不让 AI 直接编辑这个文件：里面是 API key、kubeconfig path、registry token，泄漏代价高于 AI 能省下的几次输入。`one serve` 是把这些字段从命令行 / agent 上下文里物理隔离出来的入口。

## 用法

```bash
one serve [options]
```

启动后阻塞在前台，按 Ctrl-C 退出。所有 mutation 直接写盘 `~/.config/one/{config,credentials}.json`（mode 0600，原子 rename 写入），与 `one configure <verb> <pair>` 共用底层。

## 参数

| 参数 | 说明 |
|---|---|
| `--host <host>` | 绑定主机；只接受 loopback（默认 `127.0.0.1`，也允许 `localhost`、`::1`）。非 loopback 直接报 `SERVE_BIND_FORBIDDEN`，无逃生 |
| `--port <n>` | 监听端口；默认 `0` = 由内核分配空闲端口，避免冲突 |
| `--open` | 完成后自动用浏览器打开（默认 `true`）；CI / headless / WSL / 远程 SSH 场景传 `--open=false` 关闭 |
| `-o, --output <fmt>` | `json` / `yaml` / `text`（默认按 TTY 检测） |

## 交互模式

`one serve` 没有终端交互式向导。它启动本地 Web UI，把需要手工录入的敏感 profile 字段放到浏览器表单里处理。

本地人工配置直接运行 `one serve`；脚本、CI、agent 通常只需要 `--open=false` 拿 URL，不能绕过浏览器表单直接读取明文凭据。

## 输出

绑定成功后立即向 stdout 发出一次启动信封，然后阻塞：

```json
{
  "schema": "one-cli/serve/v1",
  "status": "listening",
  "url": "http://127.0.0.1:54321/?token=8bRxr7N-GN1Q...",
  "host": "127.0.0.1",
  "port": 54321,
  "token": "8bRxr7N-GN1Q..."
}
```

URL 自带的 `?token=` 是本次启动一次性生成的 32 字节 session token；同样的 token 也作为 `HttpOnly; SameSite=Strict` cookie 在首次 GET `/` 时写入浏览器。Process 退出后此 token 立即失效——下次启动重新生成，旧 URL 不可复用。

## 安全模型

`one serve` 持有 profile 文件，profile 文件持有凭据 → 这个本地服务就是凭据外泄目标。`/api/*` 三层独立防御依次生效，每一层挡住一种威胁：

| 防御层 | 挡住的威胁 | 行为 |
|---|---|---|
| Host header 校验 | DNS rebinding（攻击者域名 resolve 到 127.0.0.1） | `Host` 必须是绑定的 `127.0.0.1:<port>` 或 `localhost:<port>`，否则返 `421 Misdirected Request` |
| Origin 校验（仅 mutating） | 跨源表单 / 脚本 POST | POST/PUT/DELETE 的 `Origin` 必须等于服务 self-origin，否则 `403 Forbidden` |
| Session token | 残留 tab 复用、CSRF | `/api/*` 必须带正确 token（cookie 或 `?token=` 查询参数），否则 `401 Unauthorized` |

凭据**默认掩码**：`GET /api/configure*` 返回 `clientSecret: "********"` / `accessKeySecret: "********"` / `password: "********"`。UI 的 "显示原文" 按钮调 `?reveal=1` 取真值。这层 masking 把单点失误的爆炸半径降到「攻击者拿到的还是一份带星号的视图」。

不在范围：

- 多用户访问（仅 127.0.0.1 单人）
- 0.0.0.0 / 局域网暴露（`SERVE_BIND_FORBIDDEN` 直接拒绝）
- 文件外部变更实时推送（外部 `one configure ... add` 改了文件，需要刷浏览器才能看到）

## 示例

### 默认（推荐）：随机端口 + 自动开浏览器

```bash
one serve
# ✓ profile UI 已启动: http://127.0.0.1:54321/?token=...
# 系统默认浏览器自动打开，Ctrl-C 退出
```

### CI / headless / WSL：只要 URL，不开浏览器

```bash
one serve --open=false
# 印出 URL，由你或别的工具自己用
```

### 固定端口（测试 / 文档截屏）

```bash
one serve --port 17900
```

### 容器 / 远程 SSH

`one serve` 默认只绑 127.0.0.1。要在远端机器跑、本地浏览器访问，靠 SSH 端口转发：

```bash
# 远端
one serve --open=false --port 17900

# 本地
ssh -L 17900:127.0.0.1:17900 remote-host
# 复制远端 stdout 打印的 URL（替换主机为 127.0.0.1）打开
```

不要试图改 `--host 0.0.0.0`——会被 `SERVE_BIND_FORBIDDEN` 直接拒掉。

## REST API

UI 用什么，你就能用什么。所有路由都需要带 token（cookie 或 `?token=`）+ Host 头匹配 + (mutating 需要) Origin 头匹配。

| 方法 | 路径 | 说明 | 响应 schema |
|---|---|---|---|
| `GET` | `/api/configure` | 全部 profile section | `one-cli/serve-configure-config/v1` |
| `GET` | `/api/configure/{domain}/{backend}` | 单个 section（`?reveal=1` 取真值） | `one-cli/serve-configure-section/v1` |
| `POST` | `/api/configure/{domain}/{backend}` | upsert：body `{name, profile, use?}` | `one-cli/serve-configure-upsert/v1` |
| `DELETE` | `/api/configure/{domain}/{backend}/{name}` | 删除 | `one-cli/serve-configure-remove/v1` |
| `PUT` | `/api/configure/{domain}/{backend}/default` | 切 default：body `{name}` | `one-cli/serve-configure-use/v1` |

合法 `(domain, backend)` 包括：`env/infisical`、`env/dotenv`、`deploy/aws-s3`、`deploy/aliyun-oss`、`deploy/tencent-cos`、`deploy/minio`、`deploy/rustfs`、`deploy/r2`、`deploy/kustomize`、`deploy/vercel`、`deploy/cloudflare`、`deploy/edgeone`、`container/docker`。其它组合返回 404。

curl 探活示例（替换 `<port>` `<token>` 为 stdout 的 envelope 里那两个值）：

```bash
curl -s "http://127.0.0.1:<port>/api/configure?token=<token>" | jq '.config | keys'
```

## 错误恢复

| 错误码 | 处理 |
|---|---|
| `SERVE_PORT_BUSY` | 换端口，或 `--port 0` 让内核挑空闲端口 |
| `SERVE_BIND_FORBIDDEN` | 仅允许绑定 loopback；改回 `127.0.0.1`（远程访问走 SSH 隧道） |
| `SERVE_TOKEN_INVALID` | 重启 `one serve` 拿新 URL；旧 token 过期或 process 已重启 |
| `SERVE_PAYLOAD_INVALID` | POST/PUT 请求体不是合法 JSON 或缺必填字段（如 `name`） |
| `PROFILE_FILE_INVALID` | `~/.config/one/config.json` 或 `credentials.json` 解析失败；手工修或删了重建 |
| `PROFILE_BACKEND_INVALID` | URL 里的 `(domain, backend)` 不是合法 pair |

完整码表：[错误码大全](/zh/docs/error-codes/)。
