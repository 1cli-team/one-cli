// Package errors centralises every CLI error code with its default
// remediation hints. This is the single source of truth — internal/cli/*
// callers should pass the Code constants from this package, not stringy
// literals, so the typechecker catches typos and the docs site can render
// the catalogue from one place.
//
// Adding a new code: register it in the Codes map below, then add a typed
// constant. Tests assert that every constant has an entry.
package errors

import "github.com/torchstellar-team/one-cli/packages/cli/internal/output"

// Code is a typed string for error codes so misuse is a compile-time error.
type Code string

// Code constants. Keep alphabetical within each group for grep-ability.
const (
	// Generic / lifecycle.
	ONE_CLI_ERROR         Code = "ONE_CLI_ERROR"
	UNKNOWN_COMMAND       Code = "UNKNOWN_COMMAND"
	PROMPT_CANCELLED      Code = "PROMPT_CANCELLED"
	OUTPUT_MARSHAL_FAILED Code = "OUTPUT_MARSHAL_FAILED"

	// Workspace / project.
	NOT_ONE_PROJECT            Code = "NOT_ONE_PROJECT"
	NODE_VERSION_UNSUPPORTED   Code = "NODE_VERSION_UNSUPPORTED"
	INVALID_NAME               Code = "INVALID_NAME"
	INVALID_WORKSPACE_ROOTS    Code = "INVALID_WORKSPACE_ROOTS"
	PROJECT_NAME_REQUIRED      Code = "PROJECT_NAME_REQUIRED"
	EXISTING_TARGET_NOT_EMPTY  Code = "EXISTING_TARGET_NOT_EMPTY"
	TARGET_EXISTS              Code = "TARGET_EXISTS"
	WORKSPACE_NESTED_FORBIDDEN Code = "WORKSPACE_NESTED_FORBIDDEN"

	// Registry / template.
	REGISTRY_FETCH_FAILED    Code = "REGISTRY_FETCH_FAILED"
	REGISTRY_INVALID         Code = "REGISTRY_INVALID"
	REGISTRY_NOT_FOUND       Code = "REGISTRY_NOT_FOUND"
	NO_TEMPLATES             Code = "NO_TEMPLATES"
	TEMPLATE_NOT_FOUND       Code = "TEMPLATE_NOT_FOUND"
	TEMPLATE_REQUIRED        Code = "TEMPLATE_REQUIRED"
	SUBPROJECT_NAME_REQUIRED Code = "SUBPROJECT_NAME_REQUIRED"

	// Preset (see internal/preset). Surfaced by `one create --preset`.
	PRESET_INVALID       Code = "PRESET_INVALID"
	PRESET_FLAG_CONFLICT Code = "PRESET_FLAG_CONFLICT"

	// Manifest.
	MANIFEST_INVALID          Code = "MANIFEST_INVALID"
	MANIFEST_MISSING_OR_EMPTY Code = "MANIFEST_MISSING_OR_EMPTY"

	// Workspace post-write sync failure. Raised by `create` / `add` when a
	// per-domain backend sync fails (or rolls back manifest) after the
	// initial write. DOCTOR_FAILED is retained only as a Go-side alias so
	// older internal references compile; public docs and wire payloads use
	// STATUS_FIX_FAILED.
	STATUS_FIX_FAILED Code = "STATUS_FIX_FAILED"
	DOCTOR_FAILED          = STATUS_FIX_FAILED // internal compatibility alias; do not document.

	// Per-domain backend selection (container / deploy / dev / ci / env).
	// Surface when one.manifest.json references a backend the build doesn't
	// know about, when a domain is required but missing, or when a profile
	// is mismatched with its target backend.
	BACKEND_ID_UNKNOWN                    Code = "BACKEND_ID_UNKNOWN"
	DOMAIN_REQUIRED                       Code = "DOMAIN_REQUIRED"
	DOMAIN_INVALID                        Code = "DOMAIN_INVALID"
	DOMAIN_NOT_REGISTERED                 Code = "DOMAIN_NOT_REGISTERED"
	DOMAIN_NOT_PER_SUBPROJECT             Code = "DOMAIN_NOT_PER_SUBPROJECT"
	SUBPROJECT_NOT_FOUND                  Code = "SUBPROJECT_NOT_FOUND"
	PATCH_CONFLICT                        Code = "PATCH_CONFLICT"
	BACKEND_INVOKE_FAILED                 Code = "BACKEND_INVOKE_FAILED"
	BACKEND_NOT_ENABLED                   Code = "BACKEND_NOT_ENABLED"
	BACKEND_VERB_NOT_SUPPORTED            Code = "BACKEND_VERB_NOT_SUPPORTED"
	BACKEND_INTERFACE_MISMATCH            Code = "BACKEND_INTERFACE_MISMATCH"
	PROFILE_FILE_INVALID                  Code = "PROFILE_FILE_INVALID"
	PROFILE_VERSION_UNSUPPORTED           Code = "PROFILE_VERSION_UNSUPPORTED"
	PROFILE_NOT_FOUND                     Code = "PROFILE_NOT_FOUND"
	PROFILE_ALREADY_EXISTS                Code = "PROFILE_ALREADY_EXISTS"
	PROFILE_NONE_CONFIGURED               Code = "PROFILE_NONE_CONFIGURED"
	PROFILE_BACKEND_INVALID               Code = "PROFILE_BACKEND_INVALID"
	PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED Code = "PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED"

	IMAGE_REF_INCOMPLETE        Code = "IMAGE_REF_INCOMPLETE"
	IMAGE_TAG_NOT_FOUND         Code = "IMAGE_TAG_NOT_FOUND"
	IMAGE_TAG_REQUIRED          Code = "IMAGE_TAG_REQUIRED"
	CI_PROVIDER_UNKNOWN         Code = "CI_PROVIDER_UNKNOWN"
	CI_RENDER_FAILED            Code = "CI_RENDER_FAILED"
	K8S_PLATFORM_UNDETECTED     Code = "K8S_PLATFORM_UNDETECTED"
	K8S_PACKAGE_UNSUPPORTED     Code = "K8S_PACKAGE_UNSUPPORTED"
	REGISTRY_CREDENTIAL_MISSING Code = "REGISTRY_CREDENTIAL_MISSING"
	CONTAINER_KIND_UNKNOWN      Code = "CONTAINER_KIND_UNKNOWN"
	CONTAINER_PROFILE_INVALID   Code = "CONTAINER_PROFILE_INVALID"
	RELEASE_FLOW_MISMATCH       Code = "RELEASE_FLOW_MISMATCH"
	ENV_PROFILE_NOT_FOUND       Code = "ENV_PROFILE_NOT_FOUND"
	LOCAL_ORCH_PORT_CONFLICT    Code = "LOCAL_ORCH_PORT_CONFLICT"

	// Vercel deploy backend.
	VERCEL_CLI_MISSING     Code = "VERCEL_CLI_MISSING"
	VERCEL_PROFILE_INVALID Code = "VERCEL_PROFILE_INVALID"
	VERCEL_DEPLOY_FAILED   Code = "VERCEL_DEPLOY_FAILED"

	// Cloudflare deploy backend.
	CLOUDFLARE_CLI_MISSING     Code = "CLOUDFLARE_CLI_MISSING"
	CLOUDFLARE_PROFILE_INVALID Code = "CLOUDFLARE_PROFILE_INVALID"
	CLOUDFLARE_DEPLOY_FAILED   Code = "CLOUDFLARE_DEPLOY_FAILED"

	// EdgeOne (Tencent) deploy backend.
	EDGEONE_CLI_MISSING     Code = "EDGEONE_CLI_MISSING"
	EDGEONE_PROFILE_INVALID Code = "EDGEONE_PROFILE_INVALID"
	EDGEONE_DEPLOY_FAILED   Code = "EDGEONE_DEPLOY_FAILED"

	// Agent docs / skills.
	AI_CONFIG_INVALID     Code = "AI_CONFIG_INVALID"
	AI_CONFIG_MISSING     Code = "AI_CONFIG_MISSING"
	AI_GUIDES_FAILED      Code = "AI_GUIDES_FAILED"
	AI_GUIDE_EXISTS       Code = "AI_GUIDE_EXISTS"
	AI_NO_SUBPROJECTS     Code = "AI_NO_SUBPROJECTS"
	AI_PROVIDER_INVALID   Code = "AI_PROVIDER_INVALID"
	SKILLS_NOT_BUNDLED    Code = "SKILLS_NOT_BUNDLED"
	SKILLS_INSTALL_FAILED Code = "SKILLS_INSTALL_FAILED"

	// Env vars — input validation (provider-agnostic).
	ENV_INVALID_ENV_NAME       Code = "ENV_INVALID_ENV_NAME"
	ENV_INVALID_KEY            Code = "ENV_INVALID_KEY"
	ENV_SET_KEY_REQUIRED       Code = "ENV_SET_KEY_REQUIRED"
	ENV_SET_OVERWRITE_REQUIRED Code = "ENV_SET_OVERWRITE_REQUIRED"
	ENV_SET_VALUE_REQUIRED     Code = "ENV_SET_VALUE_REQUIRED"
	ENV_PULL_CONFLICT          Code = "ENV_PULL_CONFLICT"
	ENV_KEY_NOT_FOUND          Code = "ENV_KEY_NOT_FOUND"
	ENV_UNKNOWN_ENVIRONMENT    Code = "ENV_UNKNOWN_ENVIRONMENT"

	// Env vars — `one env switch` backend migration.
	ENV_BACKEND_INVALID   Code = "ENV_BACKEND_INVALID"
	ENV_BACKEND_UNCHANGED Code = "ENV_BACKEND_UNCHANGED"
	ENV_MIGRATE_CONFLICT  Code = "ENV_MIGRATE_CONFLICT"
	ENV_MIGRATE_PARTIAL   Code = "ENV_MIGRATE_PARTIAL"

	// Secrets — Infisical-specific. The legacy SOPS+age error codes
	// (SECRETS_AGE_*, SECRETS_DECRYPT_*, SECRETS_EDITOR_*, etc.) were
	// retired together with the SOPS implementation in the Infisical
	// migration. Agents that previously branched on those codes should
	// switch to INFISICAL_AUTH_FAILED / INFISICAL_NOT_CONFIGURED.
	INFISICAL_NOT_CONFIGURED           Code = "INFISICAL_NOT_CONFIGURED"
	INFISICAL_AUTH_MISSING             Code = "INFISICAL_AUTH_MISSING"
	INFISICAL_AUTH_FAILED              Code = "INFISICAL_AUTH_FAILED"
	INFISICAL_PROJECT_NOT_FOUND        Code = "INFISICAL_PROJECT_NOT_FOUND"
	INFISICAL_PROJECT_NAME_TAKEN       Code = "INFISICAL_PROJECT_NAME_TAKEN"
	INFISICAL_PROJECT_CREATE_FORBIDDEN Code = "INFISICAL_PROJECT_CREATE_FORBIDDEN"
	INFISICAL_NETWORK_ERROR            Code = "INFISICAL_NETWORK_ERROR"
	INFISICAL_API_ERROR                Code = "INFISICAL_API_ERROR"
	INFISICAL_FOLDER_NOT_FOUND         Code = "INFISICAL_FOLDER_NOT_FOUND"

	// Run.
	RUN_DOTENV_MISSING    Code = "RUN_DOTENV_MISSING"
	RUN_COMMAND_NOT_FOUND Code = "RUN_COMMAND_NOT_FOUND"

	// Serve — local HTTP UI for editing profiles.
	SERVE_PORT_BUSY       Code = "SERVE_PORT_BUSY"
	SERVE_BIND_FORBIDDEN  Code = "SERVE_BIND_FORBIDDEN"
	SERVE_TOKEN_INVALID   Code = "SERVE_TOKEN_INVALID"
	SERVE_PAYLOAD_INVALID Code = "SERVE_PAYLOAD_INVALID"
)

// Definition holds the metadata associated with a Code: a short summary
// (human-readable, single sentence) and zero or more default remediation
// steps that callers can adopt or extend.
type Definition struct {
	Summary     string
	Remediation []output.Remediation
}

// Codes is the registry. Documentation generators and tests both read
// from here, so any published error-code reference can be made
// authoritative by template-rendering this map.
var Codes = map[Code]Definition{
	ONE_CLI_ERROR:         {Summary: "Generic CLI failure with no specific code."},
	UNKNOWN_COMMAND:       {Summary: "First positional argument did not match any known subcommand.", Remediation: []output.Remediation{{Action: "show-help", Hint: "查看可用命令", Command: "one --help"}}},
	PROMPT_CANCELLED:      {Summary: "User cancelled an interactive prompt (Ctrl+C / ESC)."},
	OUTPUT_MARSHAL_FAILED: {Summary: "Internal: failed to marshal a result payload to JSON. Should never fire in practice."},

	NOT_ONE_PROJECT:            {Summary: "Current directory is not a One workspace (one.manifest.json is missing).", Remediation: []output.Remediation{{Action: "create-workspace", Hint: "当前目录缺少 one.manifest.json；请先创建工作区，或 cd 到已有工作区", Command: "one create <dir>"}}},
	NODE_VERSION_UNSUPPORTED:   {Summary: "Local Node version is below the supported minimum.", Remediation: []output.Remediation{{Action: "upgrade-node", Hint: "升级到 Node.js 18+"}}},
	INVALID_NAME:               {Summary: "Project / subproject name fails the ^[a-zA-Z0-9][a-zA-Z0-9_-]*$ pattern.", Remediation: []output.Remediation{{Action: "use-valid-name", Hint: "用 kebab-case；空格替换为 -"}}},
	INVALID_WORKSPACE_ROOTS:    {Summary: "one.manifest.json#workspace.roots is malformed."},
	PROJECT_NAME_REQUIRED:      {Summary: "Non-interactive create called without a project name.", Remediation: []output.Remediation{{Action: "provide-name", Hint: "把项目名作为位置参数", Command: "one create <project-name>"}}},
	EXISTING_TARGET_NOT_EMPTY:  {Summary: "Target directory exists and is non-empty; create only writes into empty / new directories.", Remediation: []output.Remediation{{Action: "use-different-dir", Hint: "换一个空的目标目录"}, {Action: "remove-target", Hint: "手动删除已存在的目录后重试"}}},
	TARGET_EXISTS:              {Summary: "Subproject directory already exists.", Remediation: []output.Remediation{{Action: "use-different-name", Hint: "换一个 --name"}}},
	WORKSPACE_NESTED_FORBIDDEN: {Summary: "Refusing to create a workspace inside an existing workspace; nesting one workspace inside another corrupts both manifests.", Remediation: []output.Remediation{{Action: "use-add", Hint: "在现有工作区里加项目，应该用 one add", Command: "one add <template> --name <subproject-name>"}, {Action: "create-elsewhere", Hint: "或换到工作区外的目录再 one create"}}},

	REGISTRY_FETCH_FAILED:    {Summary: "Failed to download the template registry."},
	REGISTRY_INVALID:         {Summary: "Registry JSON is malformed."},
	REGISTRY_NOT_FOUND:       {Summary: "Registry path does not exist."},
	NO_TEMPLATES:             {Summary: "Registry is empty."},
	TEMPLATE_NOT_FOUND:       {Summary: "Requested template ID is not in the registry.", Remediation: []output.Remediation{{Action: "list-templates", Hint: "查看所有可用模板 ID", Command: "one templates -o json"}}},
	TEMPLATE_REQUIRED:        {Summary: "Non-interactive add called without a template ID.", Remediation: []output.Remediation{{Action: "specify-template", Hint: "把 template ID 作为位置参数", Command: "one add <template-id> --name <subproject-name>"}}},
	SUBPROJECT_NAME_REQUIRED: {Summary: "Non-interactive add called without --name.", Remediation: []output.Remediation{{Action: "provide-name", Hint: "传入 --name", Command: "one add <template-id> --name <subproject-name>"}}},

	PRESET_INVALID:       {Summary: "Preset id failed v1 grammar (bad version / segment shape / unknown code).", Remediation: []output.Remediation{{Action: "regen-preset", Hint: "用 `one serve` 打开 dashboard 重新挑组合得到新的 preset id（dashboard 页面将在后续版本上线）"}, {Action: "check-syntax", Hint: "v1 形如 `1.bgok.fnav.ei` —— 前缀为版本号，段以 `.` 分隔，每段首字符是 f/b/l/e kind"}}},
	PRESET_FLAG_CONFLICT: {Summary: "Preset id and explicit flag declared conflicting values for the same field.", Remediation: []output.Remediation{{Action: "drop-conflicting-flag", Hint: "去掉与 --preset 冲突的显式 flag（preset 已经表达了该选择）"}}},

	MANIFEST_INVALID:          {Summary: "one.manifest.json is malformed."},
	MANIFEST_MISSING_OR_EMPTY: {Summary: "Workspace has no manifest, or the manifest declares no projects.", Remediation: []output.Remediation{{Action: "add-project", Hint: "新增一个项目", Command: "one add <template-id> --name <project-name>"}}},

	STATUS_FIX_FAILED: {Summary: "Workspace 后置同步失败：写入 manifest 后某个后端 sync 回滚或失败。", Remediation: []output.Remediation{{Action: "retry", Hint: "重试触发该错误的命令"}}},

	BACKEND_ID_UNKNOWN:                    {Summary: "one.manifest.json refers to a backend id that this build does not recognise."},
	DOMAIN_REQUIRED:                       {Summary: "A domain (container / deploy / dev / ci / env) is required but its section is missing in one.manifest.json."},
	DOMAIN_INVALID:                        {Summary: "Domain name is not one of the recognised domains (container / deploy / dev / ci / env)."},
	DOMAIN_NOT_REGISTERED:                 {Summary: "Domain is recognised but this build has no backend implementation for it."},
	DOMAIN_NOT_PER_SUBPROJECT:             {Summary: "This domain operates at workspace scope; -p / --project is not allowed.", Remediation: []output.Remediation{{Action: "drop-flag", Hint: "去掉 -p / --project 重试"}}},
	SUBPROJECT_NOT_FOUND:                  {Summary: "-p / --project named a project that does not exist in manifest.projects.", Remediation: []output.Remediation{{Action: "list-projects", Hint: "查看现有项目", Command: "cat one.manifest.json"}}},
	PATCH_CONFLICT:                        {Summary: "Two configuration fragments contributed conflicting patches to the same backend target."},
	BACKEND_INVOKE_FAILED:                 {Summary: "Backend's Invoke method returned an error."},
	BACKEND_NOT_ENABLED:                   {Summary: "A domain command was invoked in a workspace where that domain is not configured.", Remediation: []output.Remediation{{Action: "configure-domain", Hint: "在 one.manifest.json 的 domains 块中配置该域（domains.env.kind / projects[].domains.container 等），或选用声明它的模板再 one add"}}},
	BACKEND_VERB_NOT_SUPPORTED:            {Summary: "The active backend in this domain does not implement the requested verb (e.g. `one env pull` against the dotenv backend).", Remediation: []output.Remediation{{Action: "switch-backend", Hint: "切换到支持该 verb 的同 domain backend（例如 env 域改用 infisical）"}}},
	BACKEND_INTERFACE_MISMATCH:            {Summary: "Internal: the dispatched backend failed its capability assertion. Build-side bug; should never reach end users."},
	PROFILE_FILE_INVALID:                  {Summary: "~/.config/one/config.json or credentials.json failed to parse as JSON.", Remediation: []output.Remediation{{Action: "edit-profile-file", Hint: "手动检查并修复对应文件，或删除后重新 `one configure add <domain>/<backend> --profile <name>`", Command: "rm ~/.config/one/config.json ~/.config/one/credentials.json"}}},
	PROFILE_VERSION_UNSUPPORTED:           {Summary: "config.json or credentials.json schema version does not match this binary.", Remediation: []output.Remediation{{Action: "upgrade-cli", Hint: "升级 one cli 到最新版本，或删除两个文件后重建配置"}}},
	PROFILE_NOT_FOUND:                     {Summary: "Requested profile does not exist under the (domain/backend) section.", Remediation: []output.Remediation{{Action: "list-profiles", Command: "one configure list env/infisical"}, {Action: "add-profile", Hint: "创建新 profile", Command: "one configure add env/infisical --profile <name>"}}},
	PROFILE_ALREADY_EXISTS:                {Summary: "A profile with this name already exists. Re-run `one configure add <domain>/<backend> --profile <name>` to update existing credentials, or pick a different name."},
	PROFILE_NONE_CONFIGURED:               {Summary: "No profile resolved from --profile / workspace binding / machine default. The backend needs an endpoint to talk to.", Remediation: []output.Remediation{{Action: "add-profile", Hint: "创建第一个 profile（替换 <domain>/<backend> 为对应 pair，如 env/infisical / deploy/aws-s3 / container/docker）", Command: "one configure add <domain>/<backend> --profile work"}}},
	PROFILE_BACKEND_INVALID:               {Summary: "Profile.backend value is not recognised, or it doesn't belong to the declared domain."},
	PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED: {Summary: "Profile's credentialSource is set to a value this build does not implement (only `file` is wired up so far).", Remediation: []output.Remediation{{Action: "use-file-source", Hint: "把 config.json 中该 profile 的 credentialSource 改回 \"file\"（或删除该字段），并确保对应密钥写在 credentials.json"}}},
	IMAGE_REF_INCOMPLETE:                  {Summary: "Deploy / CI backend needs the container image ref but it is missing or incomplete (registry / name / tag)."},
	IMAGE_TAG_NOT_FOUND:                   {Summary: "Container push target image tag does not exist in the local Docker daemon.", Remediation: []output.Remediation{{Action: "build-image", Hint: "先构建要推送的镜像", Command: "one container build <subproject>"}}},
	IMAGE_TAG_REQUIRED:                    {Summary: "Container build needs a version tag but no subproject buildVersion, Git tag, or package version was available.", Remediation: []output.Remediation{{Action: "provide-tag", Hint: "显式指定镜像版本 tag", Command: "one container build <subproject> --build-version v0.1.0"}, {Action: "set-build-version", Hint: "或在 one.manifest.json 里设置 projects[].buildVersion"}, {Action: "create-git-tag", Hint: "或在当前提交上创建 Git tag", Command: "git tag v0.1.0"}}},
	CI_PROVIDER_UNKNOWN:                   {Summary: "one.manifest.json references an unknown CI provider."},
	CI_RENDER_FAILED:                      {Summary: "The selected CI provider returned an error while rendering the workflow."},
	K8S_PLATFORM_UNDETECTED:               {Summary: "Kubernetes node architecture could not be detected before building an image for deploy.", Remediation: []output.Remediation{{Action: "check-k8s", Hint: "确认 kubeconfig/context 可访问并能列出节点", Command: "kubectl get nodes -o wide"}}},
	K8S_PACKAGE_UNSUPPORTED:               {Summary: "A deploy backend selected a Kubernetes packaging form this build does not bundle."},
	REGISTRY_CREDENTIAL_MISSING:           {Summary: "Container push needs a registry, but none is configured.", Remediation: []output.Remediation{{Action: "build-local", Hint: "只需要本地镜像时，使用 build，不需要 push", Command: "one container build <subproject>"}, {Action: "setup-registry", Hint: "需要推送到镜像仓库时，先配置 registry", Command: "one configure add container/docker --profile <name> --use"}}},
	CONTAINER_KIND_UNKNOWN:                {Summary: "manifest declares an unrecognised container kind. Supported kinds: docker / dockerhub / ghcr / acr.", Remediation: []output.Remediation{{Action: "fix-manifest-kind", Hint: "把 projects[i].domains.container.kind 改成支持的 kind"}}},
	CONTAINER_PROFILE_INVALID:             {Summary: "Container profile is missing required fields for its kind (e.g. acr needs region, docker needs registry).", Remediation: []output.Remediation{{Action: "reconfigure-container", Hint: "重新配置 container profile", Command: "one configure add container/<kind> --profile <name> --use"}}},
	RELEASE_FLOW_MISMATCH:                 {Summary: "The release-flow backend's expected toolchain or repo state does not match the workspace."},
	ENV_PROFILE_NOT_FOUND:                 {Summary: "manifest.environments[<env>] was requested by a backend but is missing or empty."},
	LOCAL_ORCH_PORT_CONFLICT:              {Summary: "Two projects requested the same dev port and the dev runner could not auto-allocate a free one."},

	VERCEL_CLI_MISSING:     {Summary: "deploy/vercel 找不到 vercel CLI。", Remediation: []output.Remediation{{Action: "install-vercel-cli", Hint: "全局安装 vercel CLI（推荐用 pnpm/npm 全局）", Command: "npm i -g vercel"}, {Action: "install-vercel-cli-via-pnpm", Hint: "或使用 pnpm 全局安装", Command: "pnpm add -g vercel"}}},
	VERCEL_PROFILE_INVALID: {Summary: "deploy/vercel profile 缺少 API token。", Remediation: []output.Remediation{{Action: "configure-vercel", Hint: "在 vercel.com → Account Settings → Tokens 创建 API token，然后写入 profile", Command: "one configure add deploy/vercel --profile <name> --use --token $VERCEL_TOKEN"}}},
	VERCEL_DEPLOY_FAILED:   {Summary: "vercel CLI 退出码非 0；查看上游日志获取详情。", Remediation: []output.Remediation{{Action: "verify-token", Hint: "确认 API token 仍然有效，且对目标 team / project 有 deploy 权限"}, {Action: "verify-project-link", Hint: "首次部署需要 vercel link：cd 到项目目录手动跑一次 `vercel link --token $TOKEN`"}}},

	CLOUDFLARE_CLI_MISSING:     {Summary: "deploy/cloudflare 找不到 wrangler CLI。", Remediation: []output.Remediation{{Action: "install-project-wrangler", Hint: "在当前 subproject 目录安装 wrangler", Command: "pnpm add -D wrangler"}, {Action: "install-wrangler", Hint: "或全局安装 wrangler CLI", Command: "npm i -g wrangler"}}},
	CLOUDFLARE_PROFILE_INVALID: {Summary: "deploy/cloudflare profile 缺少 API token。", Remediation: []output.Remediation{{Action: "configure-cloudflare", Hint: "在 dash.cloudflare.com → My Profile → API Tokens 创建 API token，然后写入 profile", Command: "one configure add deploy/cloudflare --profile <name> --use --token $CLOUDFLARE_API_TOKEN"}}},
	CLOUDFLARE_DEPLOY_FAILED:   {Summary: "wrangler CLI 退出码非 0；查看上游日志获取详情。", Remediation: []output.Remediation{{Action: "verify-token", Hint: "确认 API token 仍然有效，且对目标 account / Worker 有 Edit Workers 权限"}, {Action: "verify-account-id", Hint: "多账号场景下 wrangler 需要 CLOUDFLARE_ACCOUNT_ID；在 profile 里设置 --account-id 或在 dash 里复制 Account ID"}}},

	EDGEONE_CLI_MISSING:     {Summary: "deploy/edgeone 找不到 edgeone CLI。", Remediation: []output.Remediation{{Action: "install-edgeone", Hint: "全局安装腾讯云 EdgeOne CLI", Command: "npm i -g edgeone"}, {Action: "install-edgeone-via-pnpm", Hint: "或使用 pnpm 全局安装", Command: "pnpm add -g edgeone"}}},
	EDGEONE_PROFILE_INVALID: {Summary: "deploy/edgeone profile 缺少 EdgeOne API token。", Remediation: []output.Remediation{{Action: "configure-edgeone", Hint: "创建 EdgeOne Pages API token 后写入 profile", Command: "one configure add deploy/edgeone --profile <name> --use --token $EDGEONE_API_TOKEN"}}},
	EDGEONE_DEPLOY_FAILED:   {Summary: "edgeone CLI 退出码非 0；查看上游日志获取详情。", Remediation: []output.Remediation{{Action: "verify-token", Hint: "确认 EdgeOne API token 仍然有效，且对目标 EdgeOne Pages 项目有部署权限"}, {Action: "verify-project", Hint: "首次部署需要先在 EdgeOne 控制台创建 Pages 项目；project name 写在 manifest.projects[i].domains.deploy.config.projectName"}}},

	AI_CONFIG_INVALID:     {Summary: "one.manifest.json#ai is malformed."},
	AI_CONFIG_MISSING:     {Summary: "Reserved for legacy AI provider gates; current workspaces always render for every supported provider so this code is no longer surfaced."},
	AI_GUIDES_FAILED:      {Summary: "Agent docs refresh failed; see surfaced error message."},
	AI_GUIDE_EXISTS:       {Summary: "Existing AGENTS.md / CLAUDE.md is not managed by One CLI."},
	AI_NO_SUBPROJECTS:     {Summary: "Workspace has no recognizable projects yet."},
	AI_PROVIDER_INVALID:   {Summary: "Unknown AI provider; only codex / claude-code are supported."},
	SKILLS_NOT_BUNDLED:    {Summary: "Bundled skill directory is missing inside the package."},
	SKILLS_INSTALL_FAILED: {Summary: "Could not copy bundled skill to the target agent skills directory (check permissions)."},

	ENV_INVALID_ENV_NAME:       {Summary: "Environment name fails ^[a-zA-Z0-9][a-zA-Z0-9-_]*$ (e.g. dev, staging, prod)."},
	ENV_INVALID_KEY:            {Summary: "Variable name fails POSIX env-var pattern (uppercase + underscore + digits, must not start with digit)."},
	ENV_SET_KEY_REQUIRED:       {Summary: "env set called without <KEY>."},
	ENV_SET_OVERWRITE_REQUIRED: {Summary: "Variable already exists with a different value.", Remediation: []output.Remediation{{Action: "confirm-overwrite", Hint: "加 --yes 确认覆盖"}}},
	ENV_SET_VALUE_REQUIRED:     {Summary: "Non-interactive env set called without <VALUE>."},
	ENV_PULL_CONFLICT:          {Summary: "Existing on-disk .env differs from the values pulled from Infisical.", Remediation: []output.Remediation{{Action: "force-overwrite", Hint: "覆盖本地 .env（destructive）", Command: "one env pull --env <env> --force", Destructive: true}}},
	ENV_KEY_NOT_FOUND:          {Summary: "Requested env var key does not exist at the given Infisical path/environment."},
	ENV_UNKNOWN_ENVIRONMENT:    {Summary: "请求的环境名不在 manifest.environments.names 列表中。", Remediation: []output.Remediation{{Action: "use-existing-env", Hint: "查看 one.manifest.json#environments.names 中已声明的环境，或改用 --env 指定其中一个"}, {Action: "create-via-set", Hint: "在 dotenv 后端，用 set 隐式创建：one env set <KEY> <VALUE> --env <name>"}, {Action: "register-env", Hint: "在 Infisical 后端，先在 UI 创建环境，再把名称加入 one.manifest.json#environments.names"}}},

	ENV_BACKEND_INVALID:   {Summary: "env switch 的 <backend> 不合法，必须是 dotenv 或 infisical。"},
	ENV_BACKEND_UNCHANGED: {Summary: "工作区已经在使用目标 backend，无需切换。"},
	ENV_MIGRATE_CONFLICT:  {Summary: "目标 backend 已有同名 key 但值不一致；为防止误覆盖，默认拒绝。", Remediation: []output.Remediation{{Action: "overwrite", Hint: "确认要覆盖，加 --overwrite 重跑", Command: "one env switch infisical --overwrite", Destructive: true}, {Action: "skip-sync", Hint: "或只切 manifest，不做数据迁移", Command: "one env switch infisical --no-sync"}}},
	ENV_MIGRATE_PARTIAL:   {Summary: "部分 key 同步失败；manifest 已切换，但未完成的 key 仍只在原 backend。", Remediation: []output.Remediation{{Action: "retry", Hint: "检查报错原因（网络 / 权限），修复后再跑同步：one env switch infisical（manifest 已切，等价 sync-only）"}}},

	INFISICAL_NOT_CONFIGURED:           {Summary: "one.manifest.json#domains.env is missing, or the workspace is not using env/infisical.", Remediation: []output.Remediation{{Action: "create-with-infisical", Hint: "新工作区在 create 时选择 Infisical", Command: "one create <dir> --env-provider infisical"}, {Action: "configure-profile", Hint: "已有工作区需确认 manifest.domains.env.kind=infisical，并配置 env/infisical profile", Command: "one configure add env/infisical --profile <name> --use"}}},
	INFISICAL_AUTH_MISSING:             {Summary: "No default env profile supplies Universal Auth credentials.", Remediation: []output.Remediation{{Action: "add-profile", Hint: "在 Infisical → Organization → Access Control → Identities 创建 Universal Auth machine identity，再用 client-id / client-secret 配 profile", Command: "one configure add env/infisical --profile <name> --client-id <id> --client-secret <secret> --use"}, {Action: "use-existing-profile", Hint: "或切到已配置的 profile", Command: "one configure use env/infisical --profile <name>"}}},
	INFISICAL_AUTH_FAILED:              {Summary: "Universal Auth login was rejected by Infisical (bad client id / secret, or rate limited).", Remediation: []output.Remediation{{Action: "rotate-credentials", Hint: "重新生成 client secret 或确认 client id 来自正确的 organization"}}},
	INFISICAL_PROJECT_NOT_FOUND:        {Summary: "Infisical project id does not exist or the machine identity has no access to it."},
	INFISICAL_PROJECT_NAME_TAKEN:       {Summary: "Infisical 项目名已被占用；auto-bind 会自动加随机后缀重试，但重试次数耗尽后会冒泡此错误。", Remediation: []output.Remediation{{Action: "use-explicit-name", Hint: "在 one.manifest.json#domains.env.config.projectName 写一个不冲突的项目名后重试 env 命令"}}},
	INFISICAL_PROJECT_CREATE_FORBIDDEN: {Summary: "机器身份没有 create-project 权限。", Remediation: []output.Remediation{{Action: "grant-admin-role", Hint: "在 Infisical 后台给该 machine identity 授予 organization-level 的 admin 角色，或先手动建项目并把 projectId 写入 manifest"}, {Action: "use-explicit-id", Hint: "手动在 UI 创建项目后，把 ID 写进 one.manifest.json#domains.env.config.projectId"}}},
	INFISICAL_NETWORK_ERROR:            {Summary: "Network error reaching the Infisical API. Check siteUrl + connectivity."},
	INFISICAL_API_ERROR:                {Summary: "Infisical API returned an unexpected error. See error.context for details."},
	INFISICAL_FOLDER_NOT_FOUND:         {Summary: "The requested Infisical folder does not exist in the requested environment.", Remediation: []output.Remediation{{Action: "check-env-name", Hint: "确认 --env 名是否拼对（dev / staging / prod 等）"}, {Action: "create-folder", Hint: "在该 folder 下写入第一个环境变量值时会自动创建", Command: "one env set --env <env> -p <name|path> KEY value"}, {Action: "verify-path", Hint: "或在 Infisical UI 里确认 folder 是否存在"}}},

	RUN_DOTENV_MISSING:    {Summary: "one run could not find a .env file for the resolved subproject.", Remediation: []output.Remediation{{Action: "pull-secrets", Hint: "先把 Infisical 环境变量拉到项目 .env", Command: "one env pull"}, {Action: "specify-subproject", Hint: "或显式指定项目（按 manifest 里的 name 或相对路径）", Command: "one run -p <name|path> -- <cmd>"}}},
	RUN_COMMAND_NOT_FOUND: {Summary: "one run could not locate the requested executable on PATH.", Remediation: []output.Remediation{{Action: "check-spelling", Hint: "确认命令名拼写正确"}, {Action: "use-package-runner", Hint: "对于 npm script，使用包管理器调用", Command: "one run -- npm run <script>"}}},

	SERVE_PORT_BUSY:       {Summary: "one serve 无法绑定请求的端口（被占用或权限不足）。", Remediation: []output.Remediation{{Action: "use-random-port", Hint: "改用随机端口（让内核分配空闲端口）", Command: "one serve --port 0"}, {Action: "pick-different-port", Hint: "或显式换一个空闲端口", Command: "one serve --port 17900"}}},
	SERVE_BIND_FORBIDDEN:  {Summary: "one serve 拒绝绑定到非 loopback 地址（profile 文件含敏感凭据，仅 127.0.0.1 / localhost 才安全）。", Remediation: []output.Remediation{{Action: "use-loopback", Hint: "改用 127.0.0.1（默认）", Command: "one serve --host 127.0.0.1"}}},
	SERVE_TOKEN_INVALID:   {Summary: "请求未携带有效的 session token。Token 在启动 `one serve` 时打印的 URL 中（?token=...），并在首次访问后写入 cookie。"},
	SERVE_PAYLOAD_INVALID: {Summary: "POST/PUT 请求体不是合法 JSON 或缺少必要字段。"},
}

// Definition returns the metadata for a code, or zero-value if unknown.
// Tests catch unknown codes; production callers can ignore.
func (c Code) Definition() Definition {
	return Codes[c]
}
