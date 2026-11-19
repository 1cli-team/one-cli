---
name: one-gateway
description: Configure One Gateway in One Desktop by linking a Linear project, mapping repositories, or setting up optional OpenViking knowledge for execution machines. Also use for dispatching and monitoring One Gateway tasks (Gateway 监控、绑定 Linear 项目、仓库映射、同步 OpenViking、派发需求).
---

# One Gateway

`one gateway` talks to the configured Gateway service. One CLI and One Desktop share the connection for the same user and `ONE_HOME`. Start with `one gateway --help` when command availability is unclear and `one gateway status -o json` to inspect the connection. When asked to configure the connection, use **Settings → Gateway connection** in One Desktop or let the user enter the token at the hidden prompt for `one gateway configure <address>`.

## Link a Linear project and map its repositories

1. Open **Settings → Linear connection** (设置 → Linear 连接) and connect the Linear account.
   On first setup, register a Linear OAuth application with the exact redirect URL
   shown in Desktop, save its public Client ID, then click **Connect Linear** (连接 Linear).
   In the One project, open **Issues → Bind Linear project** (任务 → 绑定 Linear 项目).
   Choose the account, Linear project, and default team, then save the binding.
2. In each workspace that belongs to the One project, check that the Git `origin`
   in **Worktrees** points to the intended GitHub or Gitea repository. If a checkout
   is missing from the mapping picker, register it on the Gateway host with
   `one git repo add <checkout-path>`. Register the corresponding checkout on
   any separate execution machine that will run the task as well, using its
   service account and `ONE_HOME`. For Gitea, configure the host connection in
   One CLI on the machine that will access the repository.
3. With Gateway connected, open **Gateway monitor → Projects → Configure project**
   (Gateway 监控 → 项目映射 → 配置项目). Select the bound Linear project.
   Desktop matches each workspace's `origin` to a registered checkout. Select
   the intended checkout wherever there are multiple matches, then save and
   confirm the repositories listed under the project mapping.

## Set up OpenViking knowledge (optional)

Gateway can dispatch and run tasks without OpenViking. Set it up when the user
wants a workspace knowledge base or wants a worker to use its OpenViking context.

1. In One Desktop, open **Settings → OpenViking** (设置 → OpenViking). Enter the
   server URL. Supply the API key if required. Click **Test connection**
   (测试连接), then **Save connection** (保存连接).
2. Open the workspace's **Knowledge base** (知识库). Choose or enter the resource
   directory, then click **Verify and bind** (验证并绑定).
3. To copy the connection to an execution machine, open **Gateway monitor →
   Execution machines → Sync OpenViking** (Gateway 监控 → 执行机器 → 同步 OpenViking)
   for the target. The machine must be online. A remote machine must be paired
   over HTTPS. Use an OpenViking address reachable from the target, since
   `localhost` refers to the target itself. Confirm the target before clicking
   **Sync connection** (同步连接) because it replaces that machine's saved URL
   and API key. Verify that Desktop reports a successful save on the target.

Sync copies the server connection. Verify the workspace's resource-directory
binding separately under the account and `ONE_HOME` the worker uses. If the user
wants new Gateway tasks to recall and extract memories, use **Knowledge base →
Memories → Automatic memory** (知识库 → 记忆 → 自动记忆) for that workspace.
This sends task content, user answers, and Codex replies to OpenViking.

## Find and dispatch work

- To locate an existing execution, use the Gateway monitor or `one gateway jobs --issue ISSUE_ID -o json`. For a status update on a known execution, run `one gateway job JOB_ID -o json` once and report the snapshot.
- For a log snapshot, use `one gateway logs JOB_ID`.
- When asked to dispatch, use `one gateway workers -o json` to identify an online worker with the repositories mapped to the issue's Linear project. Use the worker ID from the listing. If the mapping is missing, follow [the project mapping steps](#link-a-linear-project-and-map-its-repositories).
- Run `one gateway dispatch ISSUE_ID --worker WORKER_ID -o json`. Once Gateway returns `data.jobId`, report the issue, worker, job ID, and returned status, then end the current turn by default. Gateway and the worker handle execution in the background. If the response is interrupted, look up the issue's jobs before retrying to avoid duplicate work.

## Follow an execution when explicitly requested

Use this workflow only when the user explicitly asks you to wait or keep following the execution, for example, “wait until the PR is created” or “等到 PR 出来”.

- Use `one gateway wait JOB_ID --timeout 1m -o json` in bounded intervals until the requested milestone or a decision that requires the user. A timeout only stops the local wait. If the task pauses, use its saved question or progress as described below.
- Once the run ends, report its state and the pull requests in `data.result.pullRequests` when present. Verify PR merge status separately when the user asks whether it merged.

## Answer or continue a paused execution

- For `waiting_for_answer`, find the unanswered question in `data.questions`. Relay it if it needs the user's decision. Submit an authorized answer with `one gateway reply JOB_ID QUESTION_ID 'ANSWER' -o json`.
- For `waiting_to_continue`, read `data.progress.summary`. Use `one gateway continue JOB_ID -o json` to resume the same session and worktree when the user asks you to continue or has asked you to carry the execution through and the next step is in scope. Surface any decision the saved progress needs from the user.
