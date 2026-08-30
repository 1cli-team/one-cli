---
title: one serve
description: 本地 Web UI 管理 Workspace、Project 与机器级 Profile。
---

`one serve` 启动一个仅监听 `127.0.0.1` 的本地 Dashboard，并自动打开浏览器。Dashboard 会列出这台机器上已识别的 Workspace，让你切换 Workspace、只读查看其中所有 Project 及配置，以及管理 `one configure` 使用的机器级 Profile。

为什么仍不让 AI 直接编辑 Profile 文件：里面是 API key、kubeconfig path、registry token，泄漏代价高于 AI 能省下的几次输入。`one serve` 是把这些字段从命令行 / agent 上下文里物理隔离出来的入口。

## 用法

```bash
one serve [options]
```

启动后阻塞在前台，按 Ctrl-C 退出。所有 Dashboard 请求都把代码库和 `one.manifest.json` 当作只读数据。Profile 编辑与 `one configure` 共用 `~/.config/one/{config,credentials}.json`；Workspace/Project 选择只把 Profile 名写入 `~/.config/one/profile-bindings.json`。

## 参数

| 参数 | 说明 |
|---|---|
| `--host <host>` | 绑定主机；只接受 loopback（默认 `127.0.0.1`，也允许 `localhost`、`::1`）。非 loopback 直接报 `SERVE_BIND_FORBIDDEN`，无逃生 |
| `--port <n>` | 监听端口；默认 `0` = 由内核分配空闲端口，避免冲突 |
| `--open` | 完成后自动用浏览器打开（默认 `true`）；CI / headless / WSL / 远程 SSH 场景传 `--open=false` 关闭 |
| `-o, --output <fmt>` | `json` / `yaml` / `text`（默认按 TTY 检测） |

## 交互模式

`one serve` 没有终端交互式向导。Workspace、Project、Backend 和代码库配置字段全部只读。浏览器表单只能新增/修改机器 Profile，以及选择当前环境下 Workspace 或 Project 使用哪个 Profile。

本地人工配置直接运行 `one serve`；脚本、CI、agent 通常只需要 `--open=false` 拿 URL，不能绕过浏览器表单直接读取明文凭据。

## Workspace 识别与持久化

One CLI 在两种情况下把 Workspace 登记到本机列表：

- `one create` 完整创建成功后；
- 在 Workspace 根目录或任意子目录执行 `one serve` 时。

记录保存在 XDG-aware 的 `~/.config/one/workspaces.json`。这里只存本机条目 ID、Manifest Workspace ID、名称、规范化绝对路径和最近访问时间，不复制 Project 配置、Backend、Profile 或凭据。Workspace 目录暂时不可用时会保留并显示为 missing；Forget 只删除本机列表记录，不会删除目录、Manifest、Profile 或凭据。

在 Workspace 外运行 `one serve` 也可以打开历史列表。Dashboard 默认选中本次启动所在的 Workspace；没有当前 Workspace 时，选中最近访问且可用的记录。

## 环境选择与本机存储

Dashboard UI 环境选择器只提供开发（`?env=dev`）、预览（`?env=preview`）和生产（`?env=prod`）三种；UI 中的未知 query 值会回退到开发环境。切换它不会向 Manifest 添加环境，也不会升级 Manifest schema。核心/API 存储仍可表示 `staging` 等由其他 CLI/API 工作流传入的安全自定义 ID。

全局 Settings 页会隐藏环境选择器，因为 Profile 定义和 CRUD 是机器全局的，不按环境分区。链接仍保留 query，返回 Workspace/Project 时会恢复之前的绑定上下文。

```text
~/.config/one/
├── config.json             # Profile 名、非敏感字段、default、旧绑定
├── credentials.json        # Profile 凭据
├── profile-bindings.json   # v1：规范化 root + environment -> Profile 名
└── workspaces.json          # 已识别 Workspace 注册表
```

`profile-bindings.json` 是 mode `0600` 原子替换的机器本地 v1 存储。它用规范化 Workspace root 作为 key，所以即使两份代码拷贝带着相同 Manifest Workspace ID，选择也互不影响。文件不含凭据值，也不会写入任何代码库。

对一个 `(domain, backend)`，Profile 解析顺序为：

1. 本次命令的 `--profile`；
2. Project + environment 绑定；
3. Workspace + environment 绑定；
4. `config.json` 中的旧 Project 绑定；
5. `config.json` 中的旧 Workspace 绑定；
6. 机器 default。

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
| 代码库只读边界 | 浏览器或旧客户端写源码/配置 | 旧 Manifest mutation 路径直接返回 `409 SERVE_REPOSITORY_READ_ONLY` |

凭据**默认掩码**：`GET /api/configure*` 返回 `clientSecret: "********"` / `accessKeySecret: "********"` / `password: "********"`。UI 的 "显示原文" 按钮调 `?reveal=1` 取真值。Workspace/Project 投影只返回解析到的 Profile 名和 source，不返回 Profile 字段或凭据。

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
| `GET` | `/api/workspaces` | 本机 Workspace 列表与状态 | `one-cli/workspaces/v1` |
| `DELETE` | `/api/workspaces/{entryId}` | Forget 本机记录，不删除 Workspace | 无响应体 |
| `GET` | `/api/workspaces/{entryId}/overview` | 所选 Workspace 与 Project 概览 | `one-cli/workspace-overview/v1` |
| `GET` | `/api/workspaces/{entryId}/profile-bindings/env?env={environment}` | Workspace env Profile 的有效名/source | `one-cli/workspace-profile/v1` |
| `PUT` | `/api/workspaces/{entryId}/profile-bindings/env?env={environment}` | 选择/取消 Workspace env Profile；body `{profile}` | `one-cli/workspace-profile/v1` |
| `GET` | `/api/workspaces/{entryId}/projects/{name}?env={environment}` | 只读 Project/配置投影与有效 Profile 名 | `one-cli/workspace-project/v1` |
| `PUT` | `/api/workspaces/{entryId}/projects/{name}/profile-bindings/{domain}?env={environment}` | 选择/取消 Project Profile；body `{profile}` | `one-cli/workspace-project/v1` |
| `GET/PUT` | `/api/workspace/profile-bindings/env?env={environment}` | 启动 Workspace 的 Workspace 绑定别名 | 与复数路由相同 |
| `GET` | `/api/workspace/projects/{name}?env={environment}` | 启动 Workspace 的 Project 投影别名 | `one-cli/workspace-project/v1` |
| `PUT` | `/api/workspace/projects/{name}/profile-bindings/{domain}?env={environment}` | 启动 Workspace 的 Project 绑定别名；body `{profile}` | `one-cli/workspace-project/v1` |

复数 Workspace API 只接受不透明的 `entryId`。服务端从注册表解析路径，并在每次读取或本机绑定 mutation 前重新校验 Manifest；客户端提交的任意 `root` 不会参与路径选择。`{domain}` 只能是 `env`、`deploy` 或 `container`；Backend 由服务端从只读 Manifest 推导，浏览器不能提交。空 Profile 字符串会删除该层直接绑定，恢复 fallback 解析。

旧的 Project/Environment/Deploy/Container settings PUT 路径（包括 `/api/workspace/...` 和 `/api/workspaces/{entryId}/...`）仍为旧客户端保留，但始终返回 `409 SERVE_REPOSITORY_READ_ONLY`。Dashboard 没有任何写源码或 `one.manifest.json` 的路由。复制 Workspace 导致两个有效路径共用一个 Manifest ID 时，两条记录都会保留并标记冲突：允许只读检查，本机绑定修改在冲突解决前返回 `409 Conflict`。

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
| `SERVE_PAYLOAD_INVALID` | POST/PUT 请求体不是合法 JSON 或缺必填字段（如 `name` 或 `profile`） |
| `SERVE_REPOSITORY_READ_ONLY` | 在源码/代码审查中修改代码库配置；Dashboard 只写机器 Profile 和绑定 |
| `PROFILE_FILE_INVALID` | 修复错误指向的本机 Profile 文件（`config.json`、`credentials.json` 或 `profile-bindings.json`） |
| `PROFILE_IN_USE` | 先把所有引用该 Profile 的 Workspace/Project 环境绑定改为 **Automatic**，再删除 |
| `PROFILE_BACKEND_INVALID` | URL 里的 `(domain, backend)` 不是合法 pair |

完整码表：[错误码大全](/zh/docs/error-codes/)。
