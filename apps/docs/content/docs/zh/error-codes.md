---
title: 错误码大全
description: One CLI 所有错误码的 code / context / remediation 参考。本文件由 internal/platform/errors/codes.go 自动生成，请勿手工编辑。
---

import { Callout } from "fumadocs-ui/components/callout";

<Callout type="info">
本页由 `task gen-error-codes` 从 `internal/platform/errors/codes.go` 自动生成。
要改文案，改源文件后重跑命令；不要手工编辑这个 .md。
</Callout>

## 这是什么

每个 `one` 命令出错时都会发出一个**结构化错误信封**：

```json
{
  "schema": "one-cli/error/v1",
  "error": {
    "code": "TEMPLATE_NOT_FOUND",
    "message": "...",
    "context": { "available_templates": ["nestjs-api", "go-api", "..."] },
    "remediation": [
      {
        "action": "use-different-template",
        "hint": "用注册表里的模板",
        "command": "one add nestjs-api --name api"
      }
    ]
  }
}
```

字段含义：

- **`error.code`** —— 稳定、可路由的标识符；agent 按 code 分支，不要按 message 文本分支
- **`error.context`** —— 错误现场的关键数据；常常已经包含恢复需要的信息（例如 `available_templates` 已经在错误里，agent 不用再调一次 `one templates`）
- **`error.remediation`** —— 恢复动作列表，每条带 `action` / `hint` / 可选 `command`；agent 挑一条执行后重试

下面按命令域分组列出所有 code。

## 通用 / 生命周期

命令本身的失败、用户取消、内部序列化错误。

### `ONE_CLI_ERROR`

Generic CLI failure with no specific code.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `OUTPUT_MARSHAL_FAILED`

Internal: failed to marshal a result payload to JSON. Should never fire in practice.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `PROMPT_CANCELLED`

User cancelled an interactive prompt (Ctrl+C / ESC).

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `UNKNOWN_COMMAND`

First positional argument did not match any known subcommand.

**Remediation**:

- `show-help` — 查看可用命令<br />运行：`one --help`

## 工作区 / 项目

工作区识别、命名规则、目标目录冲突等。

### `EXISTING_TARGET_NOT_EMPTY`

目标目录含有文件，或 Git 仓库存在尚未提交的删除。请检查错误中列出的冲突。

**Remediation**:

- `use-different-dir` — 选择空目录或空 Git 仓库

### `INVALID_NAME`

Project / subproject name fails the ^[a-zA-Z0-9][a-zA-Z0-9_-]*$ pattern.

**Remediation**:

- `use-valid-name` — 用 kebab-case；空格替换为 -

### `INVALID_WORKSPACE_ROOTS`

one.manifest.json#workspace.roots is malformed.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `NODE_VERSION_UNSUPPORTED`

Local Node version is below the supported minimum.

**Remediation**:

- `upgrade-node` — 升级到 Node.js 18+

### `NOT_ONE_PROJECT`

Current directory is not a One workspace (one.manifest.json is missing).

**Remediation**:

- `create-workspace` — 当前目录缺少 one.manifest.json；请先创建工作区，或 cd 到已有工作区<br />运行：`one create <dir>`

### `PROJECT_NAME_REQUIRED`

Non-interactive create called without a workspace directory.

**Remediation**:

- `provide-name` — 把工作区目录作为位置参数<br />运行：`one create <workspace-directory>`

### `TARGET_EXISTS`

Subproject directory already exists.

**Remediation**:

- `use-different-name` — 换一个 --name

## Manifest

`one.manifest.json` 的格式 / 缺失 / 内容问题。

### `MANIFEST_INVALID`

one.manifest.json is malformed.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `MANIFEST_MISSING_OR_EMPTY`

Workspace has no manifest, or the manifest declares no projects.

**Remediation**:

- `add-project` — 新增一个项目<br />运行：`one add <template-id> --name <project-name>`

## 模板 / 注册表

模板注册表的拉取、解析、查找。

### `NO_TEMPLATES`

Registry is empty.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `REGISTRY_FETCH_FAILED`

Failed to download the template registry.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `REGISTRY_INVALID`

Registry JSON is malformed.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `REGISTRY_NOT_FOUND`

Registry path does not exist.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `SUBPROJECT_NAME_REQUIRED`

Non-interactive add called without --name.

**Remediation**:

- `provide-name` — 传入 --name<br />运行：`one add <template-id> --name <subproject-name>`

### `TEMPLATE_NOT_FOUND`

Requested template ID is not in the registry.

**Remediation**:

- `list-templates` — 查看所有可用模板 ID<br />运行：`one templates -o json`

### `TEMPLATE_REQUIRED`

Non-interactive add called without a template ID.

**Remediation**:

- `specify-template` — 把 template ID 作为位置参数<br />运行：`one add <template-id> --name <subproject-name>`

## Workspace 后置同步

manifest 写入后某个 per-domain 后端 sync 失败 / 回滚（由 `create` / `add` 抛出）。

### `STATUS_FIX_FAILED`

Workspace 后置同步失败：写入 manifest 后某个后端 sync 回滚或失败。

**Remediation**:

- `retry` — 重试触发该错误的命令

## Profile / CI / 本地开发

Profile 解析、CI 产物生成和本地开发过程中的问题。

### `LOCAL_ORCH_PORT_CONFLICT`

Two projects requested the same dev port and the dev runner could not auto-allocate a free one.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `RELEASE_FLOW_MISMATCH`

The release-flow backend's expected toolchain or repo state does not match the workspace.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

## Env — 输入校验

`one env` 命令的入参校验、覆写冲突等（与 Infisical 后端无关）。

### `ENV_BACKEND_INVALID`

The workspace Infisical binding is missing or invalid.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_BACKEND_UNCHANGED`

Reserved error code from the retired local environment workflow.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_INVALID_ENV_NAME`

Environment name fails ^[a-zA-Z0-9][a-zA-Z0-9-_]*$ (e.g. dev, staging, prod).

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_INVALID_KEY`

Variable name fails POSIX env-var pattern (uppercase + underscore + digits, must not start with digit).

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_KEY_NOT_FOUND`

Requested env var key does not exist at the given Infisical path/environment.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_MIGRATE_CONFLICT`

Reserved error code from the retired local environment workflow.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_MIGRATE_PARTIAL`

Reserved error code from the retired local environment workflow.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_PROFILE_NOT_FOUND`

manifest.environments[<env>] was requested by a backend but is missing or empty.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_PULL_CONFLICT`

Reserved error code from the retired local environment workflow.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_SET_KEY_REQUIRED`

env set called without <KEY>.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_SET_OVERWRITE_REQUIRED`

Variable already exists with a different value.

**Remediation**:

- `confirm-overwrite` — 加 --yes 确认覆盖

### `ENV_SET_VALUE_REQUIRED`

Non-interactive env set called without <VALUE>.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_UNKNOWN_ENVIRONMENT`

请求的环境名不在 manifest.environments.names 列表中。

**Remediation**:

- `use-existing-env` — 查看可用环境，并使用 `--env` 选择其中一个。CLI 的 `set` 可以创建新环境，读取操作要求环境已经存在。<br />运行：`one env`

## Infisical 后端

与 Infisical API 交互过程中的认证、权限、网络问题。

### `INFISICAL_API_ERROR`

Infisical API returned an unexpected error. See error.context for details.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `INFISICAL_AUTH_FAILED`

The Infisical session was rejected or expired.

**Remediation**:

- `login` — <br />运行：`one login`

### `INFISICAL_AUTH_MISSING`

No active Infisical browser session.

**Remediation**:

- `login` — <br />运行：`one login`

### `INFISICAL_FOLDER_NOT_FOUND`

The requested Infisical folder does not exist in the requested environment.

**Remediation**:

- `check-env-name` — 确认 --env 名是否拼对（dev / staging / prod 等）
- `create-folder` — 在该 folder 下写入第一个环境变量值时会自动创建<br />运行：`one env set --env <env> -p <name|path> KEY value`
- `verify-path` — 或在 Infisical UI 里确认 folder 是否存在

### `INFISICAL_NETWORK_ERROR`

Network error reaching the Infisical API. Check siteUrl + connectivity.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `INFISICAL_NOT_CONFIGURED`

当前工作区尚未连接 Infisical 项目。读取、刷新和删除不会初始化存储。

**Remediation**:

- `set-first-variable` — 使用 CLI 或 Dashboard 保存第一个变量时，会创建并连接 Infisical 项目。<br />运行：`one env set <KEY>`

### `INFISICAL_PROJECT_CREATE_FORBIDDEN`

当前账号没有创建项目权限，请选择一个已有且有权访问的项目。

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `INFISICAL_PROJECT_NAME_TAKEN`

遇到同名项目并添加短后缀重试后，仍无法创建。One 不会根据名称连接已有项目。

**Remediation**:

- `use-explicit-name` — 在 one.manifest.json#env.projectName 写一个不冲突的项目名后重试 env 命令

### `INFISICAL_PROJECT_NOT_FOUND`

Infisical project id does not exist or the current account has no access to it.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

## 未分组

以下错误码未匹配任何分组前缀，请补充 `tools/gen-error-codes/main.go` 的 `groups` 表。

### `BACKEND_ID_UNKNOWN`

one.manifest.json refers to a backend id that this build does not recognise.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `BACKEND_INTERFACE_MISMATCH`

Internal: the dispatched backend failed its capability assertion. Build-side bug; should never reach end users.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `BACKEND_INVOKE_FAILED`

Backend's Invoke method returned an error.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `BACKEND_NOT_ENABLED`

The requested environment backend is not configured.

**Remediation**:

- `configure-domain` — Configure the environment backend with one env bind.

### `BACKEND_VERB_NOT_SUPPORTED`

The requested environment operation is not supported.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `DEPENDENCIES_NOT_INSTALLED`

Node dependencies required for local development are not installed.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `DOMAIN_INVALID`

The requested configuration domain is not supported.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `DOMAIN_NOT_PER_SUBPROJECT`

This domain operates at workspace scope; -p / --project is not allowed.

**Remediation**:

- `drop-flag` — 去掉 -p / --project 重试

### `DOMAIN_NOT_REGISTERED`

Domain is recognised but this build has no backend implementation for it.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `DOMAIN_REQUIRED`

The environment configuration is missing in one.manifest.json.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `HOOKS_CONFIG_CONFLICT`

Existing Git hooks or hk configuration conflict with One's generated setup.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `MISE_CONFIG_CONFLICT`

A managed mise configuration was modified or changed during generation.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `MISE_INSTALL_FAILED`

One could not download, migrate, verify, or prepare its managed mise runtime.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `MISE_NOT_FOUND`

The explicitly selected mise executable is unavailable.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `MISE_VERSION_UNSUPPORTED`

The installed mise version is unsupported or could not be read.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `PATCH_CONFLICT`

Two configuration fragments contributed conflicting patches to the same backend target.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `PREFERENCES_FILE_INVALID`

The local preferences file could not be read or parsed.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `PREFERENCES_INVALID`

The requested preference value is not supported.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `PRESET_FLAG_CONFLICT`

Preset id and explicit flag declared conflicting values for the same field.

**Remediation**:

- `drop-conflicting-flag` — 去掉与 --preset 冲突的显式 flag（preset 已经表达了该选择）

### `PRESET_INVALID`

Preset id failed v1 grammar (bad version / segment shape / unknown code).

**Remediation**:

- `regen-preset` — 用 `one serve` 打开 dashboard 重新挑组合得到新的 preset id（dashboard 页面将在后续版本上线）
- `check-syntax` — v1 形如 `1.bgok.fnav.ei` —— 前缀为版本号，段以 `.` 分隔，每段首字符是 f/b/l/e kind

### `RUNTIME_INVALID`

The selected execution runtime is not builtin or mise.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `RUNTIME_TASK_NOT_FOUND`

The project does not provide the requested runtime task.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `RUN_COMMAND_NOT_FOUND`

one exec could not locate the requested executable on PATH.

**Remediation**:

- `check-spelling` — 确认命令名拼写正确
- `use-package-runner` — 对于 npm script，使用包管理器调用<br />运行：`one exec -- npm run <script>`

### `RUN_DOTENV_MISSING`

Reserved error code from the retired local environment workflow.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `RUN_USAGE_INVALID`

one exec arguments do not match `one exec [project] -- <cmd> [args...]`.

**Remediation**:

- `use-run-separator` — 用 -- 分隔 One CLI 参数和子进程命令<br />运行：`one exec [project] -- <cmd> [args...]`

### `SERVE_BIND_FORBIDDEN`

one serve 拒绝绑定到非 loopback 地址（本地接口可操作敏感凭据，仅 127.0.0.1 / localhost 才安全）。

**Remediation**:

- `use-loopback` — 改用 127.0.0.1（默认）<br />运行：`one serve --host 127.0.0.1`

### `SERVE_MANIFEST_CONFLICT`

one.manifest.json changed after the Dashboard draft was opened; the stale draft was not written.

**Remediation**:

- `reload-manifest` — 重新加载 Workspace 配置，确认磁盘上的新修改后再应用草稿

### `SERVE_PAYLOAD_INVALID`

POST/PUT 请求体不是合法 JSON 或缺少必要字段。

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `SERVE_PORT_BUSY`

one serve 无法绑定请求的端口（被占用或权限不足）。

**Remediation**:

- `use-random-port` — 改用随机端口（让内核分配空闲端口）<br />运行：`one serve --port 0`
- `pick-different-port` — 或显式换一个空闲端口<br />运行：`one serve --port 17900`

### `SERVE_REPOSITORY_READ_ONLY`

Dashboard only writes explicitly allowlisted Project fields and env Backend switches through their revision-checked endpoints; this legacy route is not writable.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `SUBPROJECT_NOT_FOUND`

-p / --project named a project that does not exist in manifest.projects.

**Remediation**:

- `list-projects` — 查看现有项目<br />运行：`cat one.manifest.json`

### `WORKSPACE_NESTED_FORBIDDEN`

Refusing to create a workspace inside an existing workspace; nesting one workspace inside another corrupts both manifests.

**Remediation**:

- `use-add` — 在现有工作区里加项目，应该用 one add<br />运行：`one add <template> --name <subproject-name>`
- `create-elsewhere` — 或换到工作区外的目录再 one create
