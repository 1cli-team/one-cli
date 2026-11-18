# One Gateway

One Gateway carries Desktop requests and status between Desktop and the
execution machine. It opens a local Codex App Server connection for One CLI.
One CLI owns the workflow, including worktrees, PRs, merge monitoring and
verified Linear writeback.

One Gateway 转发 Desktop 的派发请求和任务状态，并为执行机器上的 One CLI
连接本机 Codex App Server。One CLI 负责 worktree、PR、合并跟踪和 Linear 回写。

## 本机开发 / Local development

Run `one dev` to install the development CLI and launch Desktop and the local
Gateway together on macOS or Linux. It uses the same configuration files as
`one dev:gateway`. Closing Desktop or stopping `one dev` shuts down the
Gateway it started. A Gateway already serving the saved local connection is
reused. Desktop also starts with a saved remote connection or before local
Gateway configuration is complete.

在 macOS 或 Linux 上运行 `one dev`，即可安装开发版 CLI，并一起启动桌面版和本机
Gateway。配置文件和 `one dev:gateway` 相同。关闭桌面版或停止 `one dev` 时，会关掉
这次启动的 Gateway。如果保存的本机地址上已有 Gateway 在运行，桌面会复用它。使用
远端连接或尚未配置本机 Gateway 时，桌面仍可启动。

From the One CLI checkout, run `one dev:gateway` to build both binaries and
start Gateway in the foreground. It reads
`$ONE_HOME/config/gateway-server.json` for the workflow and the saved
local HTTP address and token from `$ONE_HOME/config/gateway.json`. The default
`ONE_HOME` is `~/.one`. In an interactive terminal, Gateway displays the API
token once after it starts listening. Press Ctrl+C to stop.

在 One CLI 仓库运行 `one dev:gateway`，即可构建 One CLI 和 Gateway，并在前台启动。
流程配置读取 `$ONE_HOME/config/gateway-server.json`，监听地址和令牌复用
`$ONE_HOME/config/gateway.json` 中保存的本机 HTTP 连接。默认 `ONE_HOME` 为
`~/.one`。在交互终端启动时，监听成功后会显示一次 API 令牌。按 Ctrl+C 停止。

## 常驻机器 / Gateway host

- Use a dedicated macOS or Linux account for trusted repositories. Install
  `one`, `git` and a Codex CLI that supports permission profiles. Sign in to Codex
  and configure the Git commit identity and fetch/push authentication.
  For GitHub repositories, install `gh`, sign in and verify PR creation permissions.
- 在专用 macOS 或 Linux 账户上安装 `one`、`git` 和支持权限配置的 Codex CLI。
  登录 Codex，配置 Git 提交身份与拉取、推送凭据。使用 GitHub 仓库时，
  安装并登录 `gh`，确认账号能创建 PR。
- Authorize Linear on this account using the existing `one linear configure`
  and `one linear login` commands. Read the connection ID with
  `one linear status -o json` and project and team IDs with
  `one linear projects -o json`. Use the team's Linear workflow state IDs in the
  mapping below. For SSH setup, forward the configured Linear OAuth callback
  port to the host and use `one linear login --no-browser`.
- 使用已有的 `one linear configure` 和 `one linear login` 授权 Linear。
  用 `one linear status -o json` 查连接 ID，用 `one linear projects -o json`
  查项目和团队 ID。配置中填写各团队对应的 Linear 状态 ID。通过 SSH 授权时，
  转发 Linear OAuth 回调端口，然后使用 `one linear login --no-browser`。
- Clone the repo on the Gateway machine with the intended repository as both
  the fetch and push `origin`. Preinstall the check's tools and dependencies.
  Codex commands run with a worktree-scoped permission profile and network
  access disabled. Add specific read-only tool or dependency paths to
  `codexReadRoots` if the project needs them.
- 在常驻机器预先克隆仓库，确保 `origin` 的读写地址均指向目标仓库，
  并提前准备检查命令需要的工具和依赖。Codex 命令运行在 worktree 权限配置中，
  网络默认关闭。如果项目需要读取特定工具或依赖目录，将这些路径加入
  `codexReadRoots`。

Save a One CLI workflow JSON config with the IDs and paths for your machine. Both the running
and review states use Linear's `started` type. The done state uses `completed`.
An optional `base` overrides the default branch advertised by `origin` when a task
starts. An optional `check` argument array runs before publication and must exit
successfully.

保存 One CLI 流程的 JSON 配置并替换路径和 ID。「进行中」和「待审」使用 Linear 的
`started` 类型，「完成」使用 `completed` 类型。任务启动时使用 `origin` 的默认分支，
也可通过 `base` 指定。配置 `check` 时，命令在 worktree 内运行并通过后才发布 PR。

Use an empty `projects` object for the initial connection and monitoring setup.
Add repository mappings before dispatching Issues.

初次调试连接和监控时可以用空的 `projects` 对象。派发需求前填写仓库映射。

```json
{
  "stateDir": "/srv/one-gateway/state",
  "codexReadRoots": [],
  "projects": {
    "11111111-1111-1111-1111-111111111111": {
      "root": "/srv/repos/one-cli",
      "github": "1cli-team/one-cli",
      "connectionId": "LINEAR_CONNECTION_ID",
      "teams": {
        "22222222-2222-2222-2222-222222222222": {
          "running": "33333333-3333-3333-3333-333333333333",
          "review": "44444444-4444-4444-4444-444444444444",
          "done": "55555555-5555-5555-5555-555555555555"
        }
      }
    }
  }
}
```

For Gitea mappings, use `"repository": {"host": "git.example.com", "slug": "team/repo"}`
in place of the `github` field. The corresponding Gitea connection is stored in the execution machine’s One
configuration.

Gitea 映射使用 `"repository": {"host": "git.example.com", "slug": "team/repo"}`
字段，并在执行机器的 One 配置中保存该主机的连接。

```sh
one git host list
one git host configure https://git.example.com < /path/to/token
one git host test git.example.com
one git -C /srv/repos/project pr view 42
```

One Desktop 的「设置 → 代码库」也可管理本机的这些设置。同一用户和 `ONE_HOME`
下，CLI 和 Desktop 读取同一份代码库配置。

Build One CLI and Gateway with `mise run cli:build` and `mise run gateway:build`. Set a randomly generated
`ONE_GATEWAY_TOKEN` of at least 32 characters in your service environment.
Run the binary under a service manager with the same `ONE_HOME` and user
used for Linear authorization. Loopback is the default. Use HTTPS for direct
LAN access, or use an HTTPS reverse proxy over the default loopback listener.

用 `mise run cli:build` 和 `mise run gateway:build` 构建 CLI 和服务。在服务环境配置随机生成且至少 32 个字符的
`ONE_GATEWAY_TOKEN`，由服务管理器启动。选择授权 Linear 时使用的账户和
`ONE_HOME`。默认监听回环地址，局域网直连使用 HTTPS，也可以在回环监听前配置
HTTPS 反向代理。

```sh
services/gateway/bin/one-gateway -one /path/to/one -config /srv/one/issue-workflow.json
```

## One CLI

Save the Gateway connection from a terminal with the commands below. The token
prompt hides input. For automation, redirect a private token file into
`one gateway configure <address>`. One CLI and One Desktop share the saved
connection for the same user and `ONE_HOME`.

在终端运行以下命令即可配置并验证 Gateway 连接。令牌输入时会隐藏。
自动化时可将私有令牌文件重定向到 `one gateway configure <地址>` 的标准输入。
同一用户和 `ONE_HOME` 下，One CLI 和 One Desktop 共用这份连接。

```sh
one gateway configure http://127.0.0.1:39128
one gateway status -o json
```

Pass the Linear issue identifier or UUID and the target machine ID. Gateway
uses the repositories mapped to the issue's project and the machine's Codex
configuration. The built-in execution machine has the ID `local`.

派发时指定 Linear 需求编号或 UUID，以及目标执行机 ID。Gateway 使用需求所属项目
的仓库映射和执行机的 Codex 配置。内置执行机的 ID 是 `local`。

```sh
one gateway workers -o json
one gateway dispatch MG-3 --worker local -o json
one gateway wait JOB_ID --timeout 10m -o json
one gateway logs JOB_ID --follow
```

Use `--model` and `--effort` to override Codex settings for this dispatch. Each
omitted option uses the execution machine's default.

用 `--model` 和 `--effort` 可指定本次任务的 Codex 模型和推理强度。省略的参数沿用
执行机的默认配置。

```sh
one gateway dispatch MG-3 --worker local --model MODEL_ID --effort high -o json
```

Use `data.jobId` from the dispatch response with `one gateway job`, `wait`, or
`logs`. The completed result includes the final Codex reply when available.

使用派发结果中的 `data.jobId` 查看任务、等待结果或读取日志。任务结果会带出可读取的
Codex 最终回复。

### 回答执行中的提问 / Answer questions during execution

When Codex needs a clarification, Gateway saves the question and pauses the task.
One Desktop sends a system notification while running, including when its window
is hidden. Open the execution details to read and answer the question.
The execution machine resumes the original Codex thread and worktree after it
receives the answer.

Codex 需要你确认时，Gateway 会保存问题并暂停任务。One Desktop 运行时会发送系统
通知，窗口隐藏时也能收到。在执行详情中查看问题并回复，执行机会在原来的 Codex 会话
和工作区里继续。

`one gateway wait JOB_ID -o json` returns early with state `waiting_for_answer`
when a question needs an answer. Use the question ID from `data.questions` in
`reply`, then wait again. `one gateway job JOB_ID` includes the questions and
their saved answers.

`wait` 遇到待回答的问题会返回 `waiting_for_answer`。从 `data.questions` 取得问题 ID，
回答后再次等待任务。`job` 可以查看问题和已保存的回答。

```sh
one gateway wait JOB_ID -o json
one gateway reply JOB_ID QUESTION_ID '请使用蓝色。' -o json
one gateway wait JOB_ID -o json
```

### 继续未完成的任务 / Continue an unfinished task

If a Codex turn ends before the issue is complete, Gateway saves the progress
and sets `waiting_to_continue`. The execution machine can run other tasks while
this task is paused. PR publication starts after the work is confirmed complete
and its configured checks pass. In Desktop's execution details, select
**Continue task** to add the task to the end of the original machine's queue.
When the machine claims it, Codex resumes the original thread and worktree.

一轮结束时，只要任务还没完成，Gateway 就保存进度并进入「待继续」。下一步是否明确
都适用。暂停期间，执行机可以执行其他任务。任务确认完成、配置的检查通过后才会发布 PR。
在 Desktop 的执行详情中点击 **继续任务**，任务会排到原执行机的队尾。执行机领到任务后，
会沿用原来的 Codex 会话和工作区。

`wait` returns when the task pauses. The last saved summary is available in
`data.progress.summary`.

`wait` 会在任务进入「待继续」时返回。上轮保存的进度在 `data.progress.summary` 中。

```sh
one gateway wait JOB_ID -o json
one gateway continue JOB_ID -o json
one gateway wait JOB_ID -o json
```

## One Desktop

Open **Settings → Gateway connection**. Enter the server address and the same
token configured in the service environment. Test the connection and save it.
Open **Gateway monitor** from the home sidebar to see connection health and
task counts. Use **Dispatch issue** to select a Linear issue and an eligible
execution machine.

在 **设置 → Gateway 连接** 填入服务地址和相同的令牌，测试连接后保存。
在 **设置 → 代码库** 测试本机 GitHub 或保存 Gitea 的 HTTPS 根地址及 API Token。
Desktop 和执行机器分开部署时，在执行机器通过 One CLI 配置该机的连接。
在首页侧栏打开 **Gateway 监控 → 配置项目**，选择已绑定的 Linear 项目。
One Desktop 根据每个工作区的 Git `origin` 匹配执行机器上已登记的仓库，并一起保存。
One CLI 在同一 Codex 会话中提供这些 worktree，为有改动的仓库分别开 PR，
确认相关 PR 全部合并后回写 Linear。One CLI 使用项目已绑定的 Linear 团队及其工作流。
没有单独待审状态时，Linear 需求保持进行中，直到 PR 合并后回写完成。
保存后可在 **派发需求** 中选择需求和执行机。One CLI 在开始执行时读取远端的默认分支。

Use **Settings → Code repositories** to test the local GitHub connection or add
a Gitea server. Enter the root HTTPS URL and an API token with account read and
pull request permissions. For a separate execution machine, configure its One
CLI account there.

Use **Gateway monitor → Configure project** after connecting. Select a bound
Linear project. One Desktop matches each workspace's Git `origin` to a registered
checkout on the execution machine and saves them together. One CLI provides the
worktrees to one Codex session, opens a PR for each changed repository, and
updates Linear after all of those PRs merge. One CLI uses the
project's bound Linear team and its workflow. If the workflow has no separate
review state, the issue stays in progress until the PR is merged. One CLI reads
the remote default branch when the task starts. Register checkouts on the
Gateway host with `one git repo add`.

在 Gateway 所在机器用 `one git repo add` 登记已有 checkout 后，它会出现在
仓库选择列表。

For manual provisioning, the connection file is `$ONE_HOME/config/gateway.json`
(default `~/.one/config/gateway.json`). Use permissions `0600` on Unix.

手动部署时可以写入 `$ONE_HOME/config/gateway.json`
（默认 `~/.one/config/gateway.json`）。在 Unix 上将文件权限设为 `0600`。

```json
{
  "url": "https://gateway.example.internal",
  "token": "REPLACE_WITH_YOUR_GATEWAY_TOKEN"
}
```

One CLI stores job records under `stateDir/jobs` and checkouts under
`stateDir/worktrees`. Failed work is retained for inspection. Use the service
log and job record to inspect failures or uncertain PR creation. The service
reconciles an interrupted delivery against the repository host and writes Done after
checking for a human merge. Keep the state directory across service restarts.

任务文件保存在 `stateDir/jobs`，worktree 保存在 `stateDir/worktrees`。
遇到失败或 PR 创建结果不明时，可通过服务日志和任务记录排查。重启服务时保留
状态目录，以便 One CLI 查回已有 PR 并继续跟踪人工合并。

To run a mapped issue directly on the execution machine, use
`one linear workflow <project-id> <issue-id> --config /srv/one/issue-workflow.json`.
One CLI retains its checkpoints. Run the same command again to resume after an
interruption. The workflow configuration can also be supplied to Gateway.

可在执行机器运行上述 One CLI 命令，处理已映射的需求并跟踪 PR。中断后使用同一条
命令继续，任务状态保存在配置指定的目录。

## 局域网执行机 / LAN execution machines

The same `one-gateway` binary runs in both roles. The Gateway process starts
the local execution machine automatically. Add a Worker HTTP listener on a
specific trusted LAN address to accept additional machines. The control address
uses the connection settings above.

服务端和执行机共用 `one-gateway` 二进制。Gateway 自动启动本机执行器。
在可信内网的具体地址开启 Worker HTTP 监听，即可接入更多机器。
Desktop 和 One CLI 使用上文的控制端地址。

```sh
one-gateway -config /srv/one/issue-workflow.json -worker-listen 192.168.1.20:39129
one gateway workers invite
```

Generate one pairing code per machine in **Gateway monitor → Add execution
machine**, or with the CLI command above. On the execution machine, use the same
service account and `ONE_HOME` for repository registration, pairing, and running
the Worker. The Worker keeps its credentials in
`$ONE_HOME/config/gateway-worker.json`. Run it under the machine's service
manager. Codex tasks also require a compatible, signed-in Codex CLI on that
machine.

通过 **Gateway 监控 → 添加执行机** 或上述 CLI 命令为每台机器生成配对码。
在执行机用相同账户和 `ONE_HOME` 登记仓库、配对并运行 Worker。
配对凭据保存在 `$ONE_HOME/config/gateway-worker.json`，由系统服务管理器常驻运行
Worker。运行 Codex 任务的机器还需安装并登录兼容的 Codex CLI。

```sh
one git repo add /srv/repos/application
one-gateway worker join http://192.168.1.20:39129 --name linux-01
one-gateway worker run
```

### 通过 One 配置远端授权 / Configure remote accounts with One

Start Gateway with `-worker-https-url https://gateway.example.internal` to
accept Worker connections through the same HTTPS origin as the control API.
Use direct TLS or a local TLS-terminating proxy that overwrites
`X-Forwarded-Proto`. On the target, run the current Gateway binary and join using
the HTTPS invite. For an existing HTTP Worker, finish its assigned jobs, stop the
service, and pass `--replace` to `worker join` to obtain a new HTTPS identity.
Restart `worker run` and use the new ID from `one gateway workers`.

启动 Gateway 时加上 `-worker-https-url https://gateway.example.internal`，
Worker 和控制 API 即可共用同一个 HTTPS 地址。使用直接 TLS，或由本机代理终止 TLS
并覆盖 `X-Forwarded-Proto`。执行机使用新版 Gateway 二进制和 HTTPS 邀请配对。
已有 HTTP Worker 时，先等已派发任务结束，再停止服务，用
`worker join --replace` 获取新的 HTTPS 身份。重启 `worker run` 后，
从 `one gateway workers` 取得新的 ID。

Run the following from the initiating machine. The Linear command opens the
local browser and sends the one-time callback code to the target for exchange.
It uses the public client ID and redirect URI from the local One configuration
by default. Gitea reads a target-specific token from the hidden prompt or
standard input. The target saves the credentials in its own One configuration.

从发起端运行以下命令。Linear 在本机浏览器中授权，随后把一次性授权码交给目标机器
交换令牌。默认使用本机 One 配置中的公开 Client ID 和回调地址。Gitea 令牌使用目标机
专用令牌，可通过隐藏提示或标准输入传入。两种凭据均保存在目标机器自己的 One 配置中。

```sh
one gateway setup linear --worker WORKER_ID
one gateway setup gitea https://git.example.internal --worker WORKER_ID
one gateway setup clone https://git.example.internal/team/repo.git --worker WORKER_ID
one gateway setup project LINEAR_PROJECT_ID --worker WORKER_ID
one gateway setup status --worker WORKER_ID -o json
```

Repeat for the Gateway's built-in execution machine (`--worker local`), which
resolves and routes Linear issues. Each target uses its own Linear connection,
Git identity, Gitea token and Codex login. Use a distinct service account or
`ONE_HOME` when running two roles on the same host with independent credentials.
A Worker paired over HTTPS initializes its own workflow config if
`--workflow-config` is omitted. `setup project` reads the initiating machine’s
`gateway-server.json` by default and applies repository references and Linear
states to the target's workflow. Use `--config` for a different source file.
After setup, the target continues to execute and report results while the
initiating machine is offline.

Gateway 本机（`--worker local`）负责查找 Linear 需求和派发，也需要配置。
每台目标机器使用自己的 Linear 连接、Git 提交身份、Gitea 令牌和 Codex 登录。
同一机器运行两个角色且需要独立凭据时，请分别使用服务账号或 `ONE_HOME`。
HTTPS Worker 在省略 `--workflow-config` 时会初始化专用流程配置。
`setup project` 默认读取发起端的 `gateway-server.json`，并向目标导入仓库引用及
Linear 状态映射。可用 `--config` 指定其他来源文件。配置完成后，发起端离线时，
目标机器仍可执行任务并回传结果。

### 在指定机器派发 Linear 需求 / Dispatch Linear issues to a specific machine

In **Gateway monitor → Dispatch issue**, select a Linear issue and an eligible
machine. In One CLI, pass the machine ID returned by `one gateway workers`.
To accept Linear issues on a remote worker, configure its own Linear and code
host credentials plus a local workflow file with the same Linear project,
repositories, and base branches.
Use a persistent state directory for the workflow so the assigned machine can
resume monitoring and Linear writeback across restarts.

在 **Gateway 监控 → 派发需求** 选择 Linear 需求和执行机。One CLI 使用
`one gateway workers` 返回的机器 ID。远端机器需要自己的 Linear 和代码库凭据，以及包含同一 Linear 项目、
仓库和基准分支的流程配置。流程状态目录用于在执行机重启后恢复 PR 跟踪和 Linear 回写，
请保留该目录。

```sh
one-gateway worker run --workflow-config /srv/one/issue-workflow.json
one gateway dispatch MG-3 --worker WORKER_ID -o json
```

The execution record shows the pull requests when they are ready for review.
The selected machine keeps tracking the merge and updating Linear in its
workflow process. Repeated requests for the same issue resume its original
assignment.

执行记录会在 PR 进入待审时显示链接。所选机器上的流程进程继续跟踪合并并回写 Linear。
重复派发同一需求时，Gateway 使用原有任务记录。
