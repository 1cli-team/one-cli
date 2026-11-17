# One Gateway

One Gateway carries Desktop requests and status between Desktop and the
execution machine. It opens a local Codex App Server connection for One CLI.
One CLI owns the workflow, including worktrees, PRs, merge monitoring and
verified Linear writeback.

One Gateway 转发 Desktop 的派发请求和任务状态，并为执行机器上的 One CLI
连接本机 Codex App Server。One CLI 负责 worktree、PR、合并跟踪和 Linear 回写。

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

## One Desktop

Open **Settings → Gateway connection**. Enter the server address and the same
token configured in the service environment. Test the connection and save it.
Open **Gateway monitor** from the home sidebar to see connection health and
task counts. Use the same Linear project binding in Desktop and Gateway to
select **Send to Codex** from an Issue.

在 **设置 → Gateway 连接** 填入服务地址和相同的令牌，测试连接后保存。
在 **设置 → 代码库** 测试本机 GitHub 或保存 Gitea 的 HTTPS 根地址及 API Token。
Desktop 和执行机器分开部署时，在执行机器通过 One CLI 配置该机的连接。
在首页侧栏打开 **Gateway 监控 → 配置项目**，选择已绑定的 Linear 项目。
One Desktop 根据每个工作区的 Git `origin` 匹配执行机器上已登记的仓库，并一起保存。
One CLI 在同一 Codex 会话中提供这些 worktree，为有改动的仓库分别开 PR，
确认相关 PR 全部合并后回写 Linear。One CLI 使用项目已绑定的 Linear 团队及其工作流。
没有单独待审状态时，Linear 需求保持进行中，直到 PR 合并后回写完成。
保存后可在需求详情选择 **交给 Codex**。One CLI 在开始执行时读取远端的默认分支。

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
