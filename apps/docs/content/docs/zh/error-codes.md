---
title: 错误码大全
description: One CLI 所有错误码的 code / context / remediation 参考。本文件由 internal/errors/codes.go 自动生成，请勿手工编辑。
---

import { Callout } from "fumadocs-ui/components/callout";

<Callout type="info">
本页由 `task gen-error-codes` 从 `internal/errors/codes.go` 自动生成。
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

Target directory exists and is non-empty; create only writes into empty / new directories.

**Remediation**:

- `use-different-dir` — 换一个空的目标目录
- `remove-target` — 手动删除已存在的目录后重试

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

Non-interactive create called without a project name.

**Remediation**:

- `provide-name` — 把项目名作为位置参数<br />运行：`one create <project-name>`

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

### `REGISTRY_CREDENTIAL_MISSING`

Container push needs a registry, but none is configured.

**Remediation**:

- `build-local` — 只需要本地镜像时，使用 build，不需要 push<br />运行：`one container build <subproject>`
- `setup-registry` — 需要推送到镜像仓库时，先配置 registry<br />运行：`one configure add container/docker --profile <name> --use`

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

## 插件 / Profile / 部署

插件选择、profile 解析、部署 / CI 产物生成过程中的问题。

### `CI_PROVIDER_UNKNOWN`

one.manifest.json references an unknown CI provider.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `CI_RENDER_FAILED`

The selected CI provider returned an error while rendering the workflow.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `IMAGE_REF_INCOMPLETE`

Deploy / CI backend needs the container image ref but it is missing or incomplete (registry / name / tag).

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `K8S_PACKAGE_UNSUPPORTED`

A deploy backend selected a Kubernetes packaging form this build does not bundle.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `K8S_PLATFORM_UNDETECTED`

Kubernetes node architecture could not be detected before building an image for deploy.

**Remediation**:

- `check-k8s` — 确认 kubeconfig/context 可访问并能列出节点<br />运行：`kubectl get nodes -o wide`

### `LOCAL_ORCH_PORT_CONFLICT`

Two projects requested the same dev port and the dev runner could not auto-allocate a free one.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `PROFILE_ALREADY_EXISTS`

A profile with this name already exists. Re-run `one configure add <domain>/<backend> --profile <name>` to update existing credentials, or pick a different name.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `PROFILE_BACKEND_INVALID`

Profile.backend value is not recognised, or it doesn't belong to the declared domain.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED`

Profile's credentialSource is set to a value this build does not implement (only `file` is wired up so far).

**Remediation**:

- `use-file-source` — 把 config.json 中该 profile 的 credentialSource 改回 "file"（或删除该字段），并确保对应密钥写在 credentials.json

### `PROFILE_FILE_INVALID`

~/.config/one/config.json or credentials.json failed to parse as JSON.

**Remediation**:

- `edit-profile-file` — 手动检查并修复对应文件，或删除后重新 `one configure add <domain>/<backend> --profile <name>`<br />运行：`rm ~/.config/one/config.json ~/.config/one/credentials.json`

### `PROFILE_NONE_CONFIGURED`

No profile resolved from --profile / workspace binding / machine default. The backend needs an endpoint to talk to.

**Remediation**:

- `add-profile` — 创建第一个 profile（替换 <domain>/<backend> 为对应 pair，如 env/infisical / deploy/aws-s3 / container/docker）<br />运行：`one configure add <domain>/<backend> --profile work`

### `PROFILE_NOT_FOUND`

Requested profile does not exist under the (domain/backend) section.

**Remediation**:

- `list-profiles` — <br />运行：`one configure list env/infisical`
- `add-profile` — 创建新 profile<br />运行：`one configure add env/infisical --profile <name>`

### `PROFILE_VERSION_UNSUPPORTED`

config.json or credentials.json schema version does not match this binary.

**Remediation**:

- `upgrade-cli` — 升级 one cli 到最新版本，或删除两个文件后重建配置

### `RELEASE_FLOW_MISMATCH`

The release-flow backend's expected toolchain or repo state does not match the workspace.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

## Agent 文档 / Skills

`AGENTS.md` / `CLAUDE.md` / `.one/agents/**` 生成与 bundled skill 安装。

### `AI_CONFIG_INVALID`

one.manifest.json#ai is malformed.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `AI_CONFIG_MISSING`

Reserved for legacy AI provider gates; current workspaces always render for every supported provider so this code is no longer surfaced.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `AI_GUIDES_FAILED`

Agent docs refresh failed; see surfaced error message.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `AI_GUIDE_EXISTS`

Existing AGENTS.md / CLAUDE.md is not managed by One CLI.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `AI_NO_SUBPROJECTS`

Workspace has no recognizable projects yet.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `AI_PROVIDER_INVALID`

Unknown AI provider; only codex / claude-code are supported.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `SKILLS_INSTALL_FAILED`

Could not copy bundled skill to the target agent skills directory (check permissions).

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `SKILLS_NOT_BUNDLED`

Bundled skill directory is missing inside the package.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

## Env — 输入校验

`one env` 命令的入参校验、覆写冲突等（与 Infisical 后端无关）。

### `ENV_BACKEND_INVALID`

env switch 的 <backend> 不合法，必须是 dotenv 或 infisical。

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_BACKEND_UNCHANGED`

工作区已经在使用目标 backend，无需切换。

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

目标 backend 已有同名 key 但值不一致；为防止误覆盖，默认拒绝。

**Remediation**:

- `overwrite` — 确认要覆盖，加 --overwrite 重跑<br />运行：`one env switch infisical --overwrite` *(destructive)*
- `skip-sync` — 或只切 manifest，不做数据迁移<br />运行：`one env switch infisical --no-sync`

### `ENV_MIGRATE_PARTIAL`

部分 key 同步失败；manifest 已切换，但未完成的 key 仍只在原 backend。

**Remediation**:

- `retry` — 检查报错原因（网络 / 权限），修复后再跑同步：one env switch infisical（manifest 已切，等价 sync-only）

### `ENV_PROFILE_NOT_FOUND`

manifest.environments[<env>] was requested by a backend but is missing or empty.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `ENV_PULL_CONFLICT`

Existing on-disk .env differs from the values pulled from Infisical.

**Remediation**:

- `force-overwrite` — 覆盖本地 .env（destructive）<br />运行：`one env pull --env <env> --force` *(destructive)*

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

- `use-existing-env` — 查看 one.manifest.json#environments.names 中已声明的环境，或改用 --env 指定其中一个
- `create-via-set` — 在 dotenv 后端，用 set 隐式创建：one env set <KEY> <VALUE> --env <name>
- `register-env` — 在 Infisical 后端，先在 UI 创建环境，再把名称加入 one.manifest.json#environments.names

## Infisical 后端

与 Infisical API 交互过程中的认证、权限、网络问题。

### `INFISICAL_API_ERROR`

Infisical API returned an unexpected error. See error.context for details.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `INFISICAL_AUTH_FAILED`

Universal Auth login was rejected by Infisical (bad client id / secret, or rate limited).

**Remediation**:

- `rotate-credentials` — 重新生成 client secret 或确认 client id 来自正确的 organization

### `INFISICAL_AUTH_MISSING`

No default env profile supplies Universal Auth credentials.

**Remediation**:

- `add-profile` — 在 Infisical → Organization → Access Control → Identities 创建 Universal Auth machine identity，再用 client-id / client-secret 配 profile<br />运行：`one configure add env/infisical --profile <name> --client-id <id> --client-secret <secret> --use`
- `use-existing-profile` — 或切到已配置的 profile<br />运行：`one configure use env/infisical --profile <name>`

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

one.manifest.json#domains.env is missing, or the workspace is not using env/infisical.

**Remediation**:

- `create-with-infisical` — 新工作区在 create 时选择 Infisical<br />运行：`one create <dir> --env-provider infisical`
- `configure-profile` — 已有工作区需确认 manifest.domains.env.kind=infisical，并配置 env/infisical profile<br />运行：`one configure add env/infisical --profile <name> --use`

### `INFISICAL_PROJECT_CREATE_FORBIDDEN`

机器身份没有 create-project 权限。

**Remediation**:

- `grant-admin-role` — 在 Infisical 后台给该 machine identity 授予 organization-level 的 admin 角色，或先手动建项目并把 projectId 写入 manifest
- `use-explicit-id` — 手动在 UI 创建项目后，把 ID 写进 one.manifest.json#domains.env.config.projectId

### `INFISICAL_PROJECT_NAME_TAKEN`

Infisical 项目名已被占用；auto-bind 会自动加随机后缀重试，但重试次数耗尽后会冒泡此错误。

**Remediation**:

- `use-explicit-name` — 在 one.manifest.json#domains.env.config.projectName 写一个不冲突的项目名后重试 env 命令

### `INFISICAL_PROJECT_NOT_FOUND`

Infisical project id does not exist or the machine identity has no access to it.

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

A domain command was invoked in a workspace where that domain is not configured.

**Remediation**:

- `configure-domain` — 在 one.manifest.json 的 domains 块中配置该域（domains.env.kind / projects[].domains.container 等），或选用声明它的模板再 one add

### `BACKEND_VERB_NOT_SUPPORTED`

The active backend in this domain does not implement the requested verb (e.g. `one env pull` against the dotenv backend).

**Remediation**:

- `switch-backend` — 切换到支持该 verb 的同 domain backend（例如 env 域改用 infisical）

### `CLOUDFLARE_CLI_MISSING`

deploy/cloudflare 找不到 wrangler CLI。

**Remediation**:

- `install-project-wrangler` — 在当前 subproject 目录安装 wrangler<br />运行：`pnpm add -D wrangler`
- `install-wrangler` — 或全局安装 wrangler CLI<br />运行：`npm i -g wrangler`

### `CLOUDFLARE_DEPLOY_FAILED`

wrangler CLI 退出码非 0；查看上游日志获取详情。

**Remediation**:

- `verify-token` — 确认 API token 仍然有效，且对目标 account / Worker 有 Edit Workers 权限
- `verify-account-id` — 多账号场景下 wrangler 需要 CLOUDFLARE_ACCOUNT_ID；在 profile 里设置 --account-id 或在 dash 里复制 Account ID

### `CLOUDFLARE_PROFILE_INVALID`

deploy/cloudflare profile 缺少 API token。

**Remediation**:

- `configure-cloudflare` — 在 dash.cloudflare.com → My Profile → API Tokens 创建 API token，然后写入 profile<br />运行：`one configure add deploy/cloudflare --profile <name> --use --token $CLOUDFLARE_API_TOKEN`

### `CONTAINER_KIND_UNKNOWN`

manifest declares an unrecognised container kind. Supported kinds: docker / dockerhub / ghcr / acr.

**Remediation**:

- `fix-manifest-kind` — 把 projects[i].domains.container.kind 改成支持的 kind

### `CONTAINER_PROFILE_INVALID`

Container profile is missing required fields for its kind (e.g. acr needs region, docker needs registry).

**Remediation**:

- `reconfigure-container` — 重新配置 container profile<br />运行：`one configure add container/<kind> --profile <name> --use`

### `DOMAIN_INVALID`

Domain name is not one of the recognised domains (container / deploy / dev / ci / env).

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `DOMAIN_NOT_PER_SUBPROJECT`

This domain operates at workspace scope; -p / --project is not allowed.

**Remediation**:

- `drop-flag` — 去掉 -p / --project 重试

### `DOMAIN_NOT_REGISTERED`

Domain is recognised but this build has no backend implementation for it.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `DOMAIN_REQUIRED`

A domain (container / deploy / dev / ci / env) is required but its section is missing in one.manifest.json.

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `EDGEONE_CLI_MISSING`

deploy/edgeone 找不到 edgeone CLI。

**Remediation**:

- `install-edgeone` — 全局安装腾讯云 EdgeOne CLI<br />运行：`npm i -g edgeone`
- `install-edgeone-via-pnpm` — 或使用 pnpm 全局安装<br />运行：`pnpm add -g edgeone`

### `EDGEONE_DEPLOY_FAILED`

edgeone CLI 退出码非 0；查看上游日志获取详情。

**Remediation**:

- `verify-token` — 确认 EdgeOne API token 仍然有效，且对目标 EdgeOne Pages 项目有部署权限
- `verify-project` — 首次部署需要先在 EdgeOne 控制台创建 Pages 项目；project name 写在 manifest.projects[i].domains.deploy.config.projectName

### `EDGEONE_PROFILE_INVALID`

deploy/edgeone profile 缺少 EdgeOne API token。

**Remediation**:

- `configure-edgeone` — 创建 EdgeOne Pages API token 后写入 profile<br />运行：`one configure add deploy/edgeone --profile <name> --use --token $EDGEONE_API_TOKEN`

### `IMAGE_TAG_NOT_FOUND`

Container push target image tag does not exist in the local Docker daemon.

**Remediation**:

- `build-image` — 先构建要推送的镜像<br />运行：`one container build <subproject>`

### `IMAGE_TAG_REQUIRED`

Container build needs a version tag but no subproject buildVersion, Git tag, or package version was available.

**Remediation**:

- `provide-tag` — 显式指定镜像版本 tag<br />运行：`one container build <subproject> --build-version v0.1.0`
- `set-build-version` — 或在 one.manifest.json 里设置 projects[].buildVersion
- `create-git-tag` — 或在当前提交上创建 Git tag<br />运行：`git tag v0.1.0`

### `PATCH_CONFLICT`

Two configuration fragments contributed conflicting patches to the same backend target.

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

### `RUN_COMMAND_NOT_FOUND`

one run could not locate the requested executable on PATH.

**Remediation**:

- `check-spelling` — 确认命令名拼写正确
- `use-package-runner` — 对于 npm script，使用包管理器调用<br />运行：`one run -- npm run <script>`

### `RUN_DOTENV_MISSING`

one run could not find a .env file for the resolved subproject.

**Remediation**:

- `pull-secrets` — 先把 Infisical 环境变量拉到项目 .env<br />运行：`one env pull`
- `specify-subproject` — 或显式指定项目（按 manifest 里的 name 或相对路径）<br />运行：`one run -p <name|path> -- <cmd>`

### `SERVE_BIND_FORBIDDEN`

one serve 拒绝绑定到非 loopback 地址（profile 文件含敏感凭据，仅 127.0.0.1 / localhost 才安全）。

**Remediation**:

- `use-loopback` — 改用 127.0.0.1（默认）<br />运行：`one serve --host 127.0.0.1`

### `SERVE_PAYLOAD_INVALID`

POST/PUT 请求体不是合法 JSON 或缺少必要字段。

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `SERVE_PORT_BUSY`

one serve 无法绑定请求的端口（被占用或权限不足）。

**Remediation**:

- `use-random-port` — 改用随机端口（让内核分配空闲端口）<br />运行：`one serve --port 0`
- `pick-different-port` — 或显式换一个空闲端口<br />运行：`one serve --port 17900`

### `SERVE_TOKEN_INVALID`

请求未携带有效的 session token。Token 在启动 `one serve` 时打印的 URL 中（?token=...），并在首次访问后写入 cookie。

> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。

### `SUBPROJECT_NOT_FOUND`

-p / --project named a project that does not exist in manifest.projects.

**Remediation**:

- `list-projects` — 查看现有项目<br />运行：`cat one.manifest.json`

### `VERCEL_CLI_MISSING`

deploy/vercel 找不到 vercel CLI。

**Remediation**:

- `install-vercel-cli` — 全局安装 vercel CLI（推荐用 pnpm/npm 全局）<br />运行：`npm i -g vercel`
- `install-vercel-cli-via-pnpm` — 或使用 pnpm 全局安装<br />运行：`pnpm add -g vercel`

### `VERCEL_DEPLOY_FAILED`

vercel CLI 退出码非 0；查看上游日志获取详情。

**Remediation**:

- `verify-token` — 确认 API token 仍然有效，且对目标 team / project 有 deploy 权限
- `verify-project-link` — 首次部署需要 vercel link：cd 到项目目录手动跑一次 `vercel link --token $TOKEN`

### `VERCEL_PROFILE_INVALID`

deploy/vercel profile 缺少 API token。

**Remediation**:

- `configure-vercel` — 在 vercel.com → Account Settings → Tokens 创建 API token，然后写入 profile<br />运行：`one configure add deploy/vercel --profile <name> --use --token $VERCEL_TOKEN`

### `WORKSPACE_NESTED_FORBIDDEN`

Refusing to create a workspace inside an existing workspace; nesting one workspace inside another corrupts both manifests.

**Remediation**:

- `use-add` — 在现有工作区里加项目，应该用 one add<br />运行：`one add <template> --name <subproject-name>`
- `create-elsewhere` — 或换到工作区外的目录再 one create
