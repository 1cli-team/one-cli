# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Removed (BREAKING — 删除 6 个 `ONE_*` 环境变量)

降低用户心智负担：删掉 6 个与 `--flag` / manifest 字段重复的 `ONE_*` 环境变量。原先的「flag / env / manifest 三条解析链并存」收敛回单一路径。

| 删除 | 替代路径 |
|---|---|
| `ONE_ENV` | `--env` / `manifest.environments.default` |
| `ONE_ENV_PROFILE` | `--profile` / `manifest.domains.env.profile` |
| `ONE_DEPLOY_PROFILE` | `--profile` / `manifest.domains.deploy.profile` |
| `ONE_CONTAINER_PROFILE` | `--profile` / `manifest.domains.container.profile` |
| `ONE_REGISTRY_URL` | 直接传 `Fetch(url)`；niche dev override，先删后续真有需求再加 `--registry-url` flag |
| `ONE_NO_UPDATE_CHECK` | 跳过条件还剩 CI 自动检测 / 非 TTY / dev 版本三道 |

**保留（8 个）**：`XDG_CONFIG_HOME` / `XDG_CACHE_HOME`（freedesktop 标准）、`PATH`（系统）、`CI` / `GITHUB_ACTIONS` / `GITLAB_CI` / `CIRCLECI` / `BUILDKITE`（自动检测）——都是被动检测或系统级，不需要用户主动设置。

**Breaking change**：在 shell 里 `export ONE_ENV=prod` / `export ONE_*_PROFILE=ci` 等用法会变成静默无效，需要改成显式 `--env prod` / `--profile ci`，或写进 manifest 对应字段。`ONE_NO_UPDATE_CHECK=1` 失效，但 CI / 管道 / `-o json` 仍自动跳过升级提示。

**API surface 变化**：`profile.Resolver.Resolve` 不再产出 `Source: "env"`；`profile.EnvVarFor` 已删除（仅在 `internal/` 中使用过）。

### Changed (BREAKING — `one configure` 命令顺序翻转为 verb-first)

把 `one configure <pair> <verb>` 翻转成 `one configure <verb> <pair>`，对齐
`git remote add origin` / `kubectl create deployment` 等主流 CLI 的
verb-noun-target 习惯。**不保留旧顺序的 alias** — 老脚本会报 `UNKNOWN_COMMAND`。

迁移映射（逐字替换）：

| 旧 | 新 |
|---|---|
| `one configure env/infisical add <name> [flags]` | `one configure add env/infisical <name> [flags]` |
| `one configure env/infisical list` | `one configure list env/infisical` |
| `one configure env/infisical current` | `one configure current env/infisical` |
| `one configure env/infisical show <name>` | `one configure show env/infisical <name>` |
| `one configure env/infisical use <name>` | `one configure use env/infisical <name>` |
| `one configure env/infisical remove <name>` | `one configure remove env/infisical <name>` |
| 同样对 `env/dotenv` / `deploy/s3` / `deploy/kustomize` / `container/docker` 翻转 | |

**同步变更：**

- `add` 把每个 backend 注册成 sub-subcommand，所以 `one configure add env/infisical --help` 只展示 infisical 专属的 flag（`--site-url` / `--client-id` / `--client-secret`），不再混入 s3 / docker 的 flag
- 新增 `one configure add`（无参，TTY）作为 `one configure` 交互式向导的快捷入口
- 新增聚合视图：`one configure list`（无 pair）滚动输出所有 5 个 section 的 profile；`one configure current`（无 pair）输出每个 section 的 active 指针
- 新增 JSON envelope schema：`one-cli/configure-list-all/v1`、`one-cli/configure-current-all/v1`
- 错误码 remediation、`apps/docs/**`、bundled skill 同步翻转

**未改：**

- 单 pair 的 schema 名（`one-cli/configure-{add,list,current,show,use,remove}/v1`）
- 配置文件名 `~/.config/one/{config,credentials}.json`
- 内部 Go 包 `internal/profile`、所有公共类型、REST surface (`/api/configure/*`)

### Changed (BREAKING — `one profile` → `one configure`)

把 `one profile` 命令树重命名为 `one configure`,对齐业界 CLI 子命令名(AWS / gcloud / azure)。**不保留 `one profile` alias** — 老脚本会报 `UNKNOWN_COMMAND`。

迁移映射(逐字替换):

| 旧 | 新 |
|---|---|
| `one profile env/infisical <verb>` | `one configure env/infisical <verb>` |
| `one profile env/dotenv <verb>` | `one configure env/dotenv <verb>` |
| `one profile deploy/s3 <verb>` | `one configure deploy/s3 <verb>` |
| `one profile deploy/kustomize <verb>` | `one configure deploy/kustomize <verb>` |
| `one profile container/docker <verb>` | `one configure container/docker <verb>` |

**同步变更:**

- 新增 `one configure`(无参,TTY)交互式向导:先选 (domain, backend) 再走对应 add 流程。AWS-CLI 风味的快捷入口
- REST API 路径 `/api/profile/*` → `/api/configure/*`(`one serve` web UI)
- JSON envelope schema 改名(11 处):
  - `one-cli/profile-{add,list,current,show,use,remove}/v1` → `one-cli/configure-{add,list,current,show,use,remove}/v1`
  - `one-cli/serve-profile-{config,section,upsert,remove,use}/v1` → `one-cli/serve-configure-{config,section,upsert,remove,use}/v1`
- `addResult` JSON envelope 字段:`profile_path` 拆为 `config_path` + `credentials_path`(承接 v4 schema 的双文件物理拆分)

**未改:**

- `--profile <name>` flag 名(它指代 *profile-the-noun*,不是命令)
- `ONE_<DOMAIN>_PROFILE` 环境变量名
- `manifest.<domain>.preferredProfile` 字段名
- 错误码常量名 `PROFILE_*`(只改文案中提到的命令)
- Go 包 `internal/profile` 名称 + 所有公共类型
- 配置文件名 `~/.config/one/{config,credentials}.json`

## [1.0.0]

### Changed (BREAKING — repository layout & Go module path)

仓库本身按 One CLI 推荐的 `apps/* + packages/*` 布局重排。**对外影响**：
公共 Go 包的 import 路径前缀变更，下游使用者需要更新 import。

- Go module path：
  - `github.com/torchstellar-team/one-cli` →
    `github.com/torchstellar-team/one-cli/packages/cli`
- 公共包 import 路径相应变化（其它路径同理）：
  - `github.com/torchstellar-team/one-cli/pkg/toolchain` →
    `github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain`
  - `github.com/torchstellar-team/one-cli/pkg/agentskills` →
    `github.com/torchstellar-team/one-cli/packages/cli/pkg/agentskills`
  - `github.com/torchstellar-team/one-cli/pkg/infra` →
    `github.com/torchstellar-team/one-cli/packages/cli/pkg/infra`
  - `github.com/torchstellar-team/one-cli/pkg/plugin` →
    `github.com/torchstellar-team/one-cli/packages/cli/pkg/plugin`

物理目录搬动（仅对 contributors 有感）：

- `cmd/`、`internal/`、`pkg/`、`tools/`、`testdata/`、`go.mod`、`go.sum`
  → `packages/cli/`
- `templates/` + 根 `registry.json` → `packages/templates/`
- `skills/` → `packages/skills/`
- `docs/` → `apps/docs/`
- `web/` → `apps/dashboard/`
- 二进制产物：`bin/one` → `packages/cli/bin/one`

终端用户面向的命令、JSON 输出 schema、模板、skills 与安装脚本
（`install.sh`）全部不变。CI 流水线、Taskfile 目标已同步更新。

## [0.8.0]

### Changed (BREAKING — command surface)

- **`one dev` 和 `one deploy` 收成 leaf 动词**。动词本身就是动作，
  不再有子命令：
  - `one dev start` → `one dev`
  - `one deploy apply` → `one deploy`
- **删除辅助子命令**：
  - `one dev path`：要看 `Procfile.dev` 路径直接
    `cat <workspace>/Procfile.dev` 或读 manifest。
  - `one deploy render`：预览将执行的 docker / kubectl / s3 argv 用
    `one deploy --dry-run`；预览渲染后的 K8s YAML 用
    `kubectl kustomize <overlay>`。
- **不保留兼容别名**。`one dev start` / `one dev path` /
  `one deploy apply` / `one deploy render` 全部报 cobra 的
  `unknown command`。CI / 脚本需要一次性切到新命令。

### Added

- `one dev -p / --project <name|path>`：只起指定 project 的 dev 进程
  （selector 语义跟 `one deploy -p` 一致，由
  `workspace.ResolveProjectFromSelector` 归一化）。

### Removed (Go API)

- `internal/infra/s3.Render` / `RenderInput` / `RenderResult` /
  `SchemaRender`
- `internal/infra/kustomize.Render` / `RenderInput` / `RenderResult` /
  `SchemaRender`
- `internal/localorch/process.Path` / `PathInput` / `PathResult` /
  `SchemaPath`

  这些都是 `internal/`，不影响下游 `pkg/` 导入者。

## [0.7.0]

### Changed (BREAKING — manifest schema)

- **Manifest 顶层字段统一改名为 "project" 术语**，和 v0.8 引入的
  `-p/--project` 选择器对齐：
  - `manifest.subprojects[]` → `manifest.projects[]`
  - `manifest.project`（workspace 身份 `{id, name}`）→ `manifest.workspace`
    （并入原 `manifest.workspace.roots`，新形状是
    `{ id, name, roots? }`）
  - 内嵌字段路径里的 `subprojects[i].xxx` 同步改成 `projects[i].xxx`
    （如 `projects[i].deploy.bucket`、`projects[i].container.namespace`）
- **manifest 版本号不变（仍是 v3），但旧字段名一律拒绝**——硬切换，
  不再带 v2 → v3 兼容代码（`legacyV2Envelope` 已删除）。老 manifest
  必须手动 jq 改 key 才能继续被新 CLI 读：

  ```bash
  jq '
    .workspace = (.project // {}) + {roots: ((.workspace.roots) // [])}
    | .projects = .subprojects
    | del(.project, .subprojects)
  ' one.manifest.json > one.manifest.json.new
  mv one.manifest.json.new one.manifest.json
  ```

- **Go API 同步重命名**（影响下游导入 `pkg/` 的项目）：
  - `workspace.ManifestSubproject` → `workspace.ManifestProject`
  - `workspace.ManifestProject`（workspace 身份）→ `workspace.ManifestWorkspace`
  - `workspace.WorkspaceCfg` → 删除（合并入 `ManifestWorkspace`）
  - `workspace.SubprojectEnvSection` / `…ContainerSection` /
    `…DeploySection` → `Project*Section`
  - `workspace.Subproject`（运行时类型）→ `workspace.Project`
  - `workspace.SetManifestProject(root, ManifestProject{...})` →
    `workspace.SetManifestWorkspaceIdentity(root, id, name)`
  - `*ForSubproject` 系列函数全部改名为 `*ForProject`
- **JSON envelope 字段同步改名**：`dockerfile.BuildEntry.Subproject` →
  `Project`、`InfoResult.Subprojects` → `Projects`，对应 JSON tag 从
  `"subproject"` / `"subprojects"` 改成 `"project"` / `"projects"`。
- **面向用户的中文文案**："子项目" 全部改成 "项目"。

### Changed

- **`one run` 不带任何位置参数时打印 help 并以 exit 0 退出**，与
  `one deploy` / `one env` 等父命令的行为一致；原本会报 cobra 默认
  `requires at least 1 arg(s)` JSON 错误。

## [0.6.0]

### Changed (BREAKING)

- **Profile 管理收敛到顶层 `one profile <domain>/<backend> <verb>`**。
  v0.5 把 profile 入口分散在两处（`one <domain> profile` 提供 CRUD、
  `one setup` 提供 onboarding），用户先在 setup 添加再去 `<domain>
  profile` 切换 / 列表 / 删除，记忆点割裂；新形态把生命周期统一到一个
  顶层命令，每个 `(domain, backend)` 是独立子树，verb 共享
  `add` / `list` / `current` / `show` / `use` / `remove`。
  - `one env profile <verb>` → `one profile env/infisical <verb>` 或
    `one profile env/dotenv <verb>`
  - `one deploy profile <verb>` → `one profile deploy/s3 <verb>` 或
    `one profile deploy/kustomize <verb>`
  - `one container profile <verb>` → `one profile container/docker <verb>`
  - `one setup env infisical [name]` → `one profile env/infisical add [name]`
  - `one setup deploy s3 [name]` → `one profile deploy/s3 add [name]`
  - `one setup deploy kustomize [name]` → `one profile deploy/kustomize add [name]`
  - `one setup container docker [name]` → `one profile container/docker add [name]`
  - `--backend` flag 全部下线（路径已经决定 backend）；同名跨 backend
    不再需要消歧。
  - `add` 是 upsert 语义：同名再跑 = 覆盖凭据（`status=updated`），
    用来轮换 token；之前 `<domain> profile add` 的"已存在则报错"行为
    被合并掉。
- **Skills 安装拆出为独立顶层 `one skills install`**（替代 `one setup skills`）。
  flag 集（`--agent` / `--yes`）保持不变，只是命令前缀变了。
- **`one setup` 命令树整体移除**。`internal/cmd/setupcmd/` 包随之删除；
  上层 `one env` / `one deploy` / `one container` 不再 import
  `profilecmd.BuildProfileCommand` 工厂（已移除）。
- **JSON envelope schema 改名跟着命令路径走**：
  - `one-cli/<domain>-profile-{add,list,use,remove,show,current}/v1` →
    `one-cli/profile-{add,list,use,remove,show,current}/v1`
    （payload 里增 `domain` + `backend` 字段表达原本刻在 schema 名字里的维度）。
  - `one-cli/setup-{env-infisical,deploy-s3,deploy-kustomize,container-docker}/v1`
    全部并入 `one-cli/profile-add/v1`。
  - `one-cli/setup/v1`（skill 安装）→ `one-cli/skills-install/v1`。
- **解析链不动**：`--profile` flag、`ONE_<DOMAIN>_PROFILE` 环境变量、
  `manifest.<domain>.preferredProfile`、`~/.config/one/profiles.json` 的
  v3 schema 都保持原样。现有工作区无需迁移。

### Removed (BREAKING — profile / setup refactor)

- `one env profile` / `one deploy profile` / `one container profile` 整个
  子树移除。
- `one setup` 整个命令树移除（`one setup` / `one setup skills` /
  `one setup env infisical` / `one setup deploy s3` /
  `one setup deploy kustomize` / `one setup container docker`）。
- 旧 `BuildProfileCommand(domain)` 工厂移除；`profilecmd` 现在 export
  自己的顶层 cobra 注册（`one profile`）和 backend builders。

### Changed (BREAKING)

- **`-o` 输出标志整理为 kubectl 风格的格式名**：
  - 旧值 `-o tty` 改为 `-o text`（语义不变：强制人类可读输出）。
  - 新增 `-o yaml` 输出 YAML envelope（与 JSON 同 schema，键名沿用
    `json:"..."` tag）。
  - `-o json` 现在输出 pretty-printed JSON（2-space 缩进）。snapshot
    比较是结构化的（`json.Unmarshal` + `reflect.DeepEqual`），下游
    消费方用 `jq` / 任何 JSON parser 都不受影响。
  - 删除 `ONE_OUTPUT` 环境变量；改用 `-o json` / `-o yaml` / `-o text`。
    pipe → JSON、TTY → 人类格式的自动检测仍然保留（YAML 是 opt-in only）。
  - 未识别值仍 fall through 到 auto（kubectl-style leniency）；
    旧脚本里的 `-o tty` 在 pipe 时会落到 JSON、在 TTY 时落到人类格式，
    与原意巧合一致，无声切换通常无影响。

### Removed (BREAKING)

- **`one status` 命令以及它的 `--fix` drift 修复链路被移除**。CLI
  不再输出 `one-cli/status/v2` / `one-cli/status-fix/v2` 信封；消费方
  请直接读取 `one.workspace.json` / `one.manifest.json`。drift 检测与
  自动修复需要重新跑对应的 per-domain 命令（`one add` /
  `one container build` / `one deploy render` 等），它们各自负责
  rerender 自己的 artefacts。
- 内部包 `internal/sync` 与 `internal/workspace.Status()` 一并移除
  （仅 `status` 使用过它们）。
- 错误码 `STATUS_FIX_FAILED` 保留（仍由 `create` / `add` 在后置同步失败时
  抛出），但 remediation 文案不再指向已删除的 `one status --fix`。
- 用户文档：`docs/content/docs/reference/status.md`、
  `docs/content/docs/guides/maintenance.md` 与
  `skills/one-cli/references/fix.md` 一并删除，引用站点导航与
  skill INDEX 已同步收敛。

### Added

- **`one setup` is now a parent command** with credential-configuration
  subcommands beyond the original skill installer:
  - `one setup skills` — install / refresh bundled skill (the v0.4
    `one setup` behaviour, now under an explicit verb).
  - `one setup env infisical [name]` — write an Infisical machine-level
    profile (siteUrl + Universal Auth credentials). Schema:
    `one-cli/setup-env-infisical/v1`.
  - `one setup deploy s3 [name]` — write an S3-protocol object-storage
    profile (endpoint + ak/sk; covers Aliyun OSS / AWS S3 / MinIO /
    RustFS / Cloudflare R2). Schema: `one-cli/setup-deploy-s3/v1`.
    Bucket is per-subproject; see `one add`.
  - `one setup deploy kustomize [name]` — write a Kustomize profile
    (kubeconfig context). Schema: `one-cli/setup-deploy-kustomize/v1`.
    k8s namespace + overlay path are workspace-scoped manifest fields;
    see `one add`.
  - `one setup container docker [name]` — write a container registry
    profile (registry + username + password) for Aliyun ACR / Docker
    Hub / GHCR / GitLab / Harbor. Schema:
    `one-cli/setup-container-docker/v1`. Registry namespace is
    per-subproject; see `one add`.
  All four credential subcommands share a payload shape with `status`
  (`completed` for fresh entries, `updated` for replacements) — re-run
  with the same `[name]` to rotate credentials.
- **`profile.Upsert(domain, name, profile, setActive)`** API: insert
  or replace, no `PROFILE_ALREADY_EXISTS` error. The CRUD `add` verb
  keeps strict-add semantics; `setup` uses Upsert.
- **DomainContainer profile family**:
  - `~/.config/one/profiles.json#container/docker.profiles[name]`
    carries `{registry, credentials: {username, password}}`. Registry
    namespace is per-subproject (manifest field; see below).
  - New `one container profile <list|add|use|show|remove|current>`
    CRUD (parameterised on `profile.DomainContainer`).
  - `manifest.container.preferredProfile` and per-subproject
    `subprojects[i].container.profile` for resolver pinning.
- **Per-subproject + workspace-level deploy-target manifest fields**:
  - `subprojects[i].container.namespace` — registry namespace prefix
    in the image tag (`<registry>/<namespace>/<workload>:dev`). Lives
    per-subproject because one credential commonly hosts multiple
    workloads under different namespaces.
  - `subprojects[i].deploy.bucket` — S3 bucket name. Lives
    per-subproject because one credential reaches many buckets.
  - `manifest.deploy.namespace` + `manifest.deploy.kustomizationPath`
    — k8s namespace and overlay base path. Workspace-shared because a
    workspace usually deploys all its services to one namespace from
    one overlay tree.
  - `one add` prompts for the relevant ones interactively when the
    template's defaults include the matching backend; non-interactive
    `--yes` mode leaves them empty (hand-edit the manifest later).
- **`one container push [subproject]`** — push to the resolved
  registry. Schema: `one-cli/container-push/v1`. Requires a container
  profile.
- **`one container build`** now consumes the container profile when
  configured: image tag becomes `<registry>/[<namespace>/]<workload>:dev`
  (namespace from `subprojects[i].container.namespace`) and
  `docker login --password-stdin` runs once before the first build.
  `--profile <name>` flag for one-shot override.

### Wire format BREAKING

- **Bare `one setup` no longer installs skills**. It now prints the
  list of `setup` subcommands and exits 0 with no side effects. To
  install / refresh the bundled skill, use `one setup skills` (same
  flag set, same `one-cli/setup/v1` schema). Migration: any script
  doing `one setup` should be edited to `one setup skills`. The curl
  installer prints the new command in its post-install hint.

- **`~/.config/one/profiles.json` schema v2 → v3**. Each (domain,
  backend) is now its own top-level section, keyed by `<domain>/<backend>`
  (e.g. `env/infisical`, `deploy/s3`, `deploy/kustomize`,
  `container/docker`). Active pointer is per-section, so
  `deploy/s3.active` and `deploy/kustomize.active` coexist — flipping
  to a kustomize profile no longer silently breaks s3 deploys. v2
  files auto-migrate on first read; the next save persists v3. The
  `Profile.backend` discriminator field is gone (the section key is
  the discriminator). Profile names can repeat across backends within
  one domain; `one <domain> profile use|show|remove <name>` adds a
  new `--backend` flag that disambiguates when the same name exists
  under multiple backends.

  Wire-format consumers:
  - `one <domain> profile list` envelope: `active` field changed
    from `string` to `{backend → activeName}` map.
  - `one <domain> profile current` envelope: same `active` map shape.
  - `one <domain> profile show / use / remove` envelope: gains a
    `backend` field naming which (domain, backend) section was
    affected.

- **`profile.Resolve(ResolveInput{...})`** Go API: `ResolveInput`
  gains a required `Backend` field. Callers always know the backend
  they want at call site (envcmd → `"infisical"`, containercmd →
  `"docker"`, deploycmd → the subproject's declared backend), so the
  resolver no longer guesses based on stored metadata. Removed the
  post-resolve "profile.Backend mismatch" error path: storage layout
  makes the mismatch impossible.

- **Profile schema v3 dropped four deploy-target fields** that were
  workspace / subproject metadata, not machine-level identity:
  - `S3Profile.bucket` → `subprojects[i].deploy.bucket`
  - `KustomizeProfile.namespace` → `manifest.deploy.namespace`
  - `KustomizeProfile.kustomizationPath` →
    `manifest.deploy.kustomizationPath`
  - `ContainerProfile.namespace` → `subprojects[i].container.namespace`
  Rationale: profiles are machine-shared (one Aliyun account hits many
  workspaces); pinning the bucket / namespace inside the profile would
  force a profile-per-bucket pattern. The matching `--bucket`,
  `--namespace`, `--kustomization-path`, `--registry-namespace` flags
  on `one setup deploy s3 / kustomize / container docker` and
  `one <domain> profile add` are gone. The `BuildS3Profile`,
  `BuildDockerProfile`, `PromptKustomize` Go APIs lose those
  parameters as well. Legacy v3 profiles with these fields read
  cleanly (Go's json unmarshal silently drops unknown fields) and
  re-save without them.

### Wire format BREAKING

- **Error code constants renamed**: `PLUGIN_*` → `BACKEND_*` / `DOMAIN_*`.
  Old constants are kept as Go-side aliases (compile-time compatibility),
  but emitted `error.code` strings are the new names. Mapping:
  - `PLUGIN_ID_UNKNOWN` → `BACKEND_ID_UNKNOWN`
  - `PLUGIN_FAMILY_REQUIRED` → `DOMAIN_REQUIRED`
  - `PLUGIN_FAMILY_INVALID` → `DOMAIN_INVALID`
  - `PLUGIN_FAMILY_NOT_REGISTERED` → `DOMAIN_NOT_REGISTERED`
  - `PLUGIN_FAMILY_NOT_PER_SUBPROJECT` → `DOMAIN_NOT_PER_SUBPROJECT`
  - `PLUGIN_SUBPROJECT_NOT_FOUND` → `SUBPROJECT_NOT_FOUND`
  - `PLUGIN_PATCH_CONFLICT` → `PATCH_CONFLICT`
  - `PLUGIN_INVOKE_FAILED` → `BACKEND_INVOKE_FAILED`
  - `PLUGIN_NOT_ENABLED` → `BACKEND_NOT_ENABLED`
  - `PLUGIN_VERB_NOT_SUPPORTED` → `BACKEND_VERB_NOT_SUPPORTED`
  - `PLUGIN_INTERFACE_MISMATCH` → `BACKEND_INTERFACE_MISMATCH`
  - `PROFILE_PLUGIN_INVALID` → `PROFILE_BACKEND_INVALID`
  Agents matching on `error.code` must update to the new strings.
- **JSON envelope schemas bumped to v2** for commands whose v1 shape
  carried plugin-named fields:
  - `one-cli/create/v1` → `one-cli/create/v2`: drops `enabled_plugins`,
    adds `secrets_backend` (string), `ci_enabled` (bool), `dev_enabled`
    (bool).
  - `one-cli/status/v1` → `one-cli/status/v2`: per-subproject
    `infra.container_plugin` (namespaced id) → `infra.container_backend`
    (bare name e.g. `"docker"`); `infra.deploy_plugin` →
    `infra.deploy_backend` (bare name e.g. `"kustomize"` / `"s3"`).
  - `one-cli/status-fix/v1` → `one-cli/status-fix/v2` (rides along).
  - `one-cli/container-info/v1` → `one-cli/container-info/v2`:
    `container_plugin` (namespaced) → `container_backend` (bare); per
    subproject `plugin_id` → `backend`.
  - `one-cli/container-build/v1` → `one-cli/container-build/v2` (rides
    along). Other commands (`add`, `templates`, `env-*`, `dev-*`,
    `deploy-*`, profile commands) stay on v1.
- **`one env|deploy profile` envelopes** rename `plugin` field to
  `backend` (bare backend name). The `--plugin` flag is hidden alias of
  `--backend`; legacy v1 profile files auto-migrate on read.

### Removed (BREAKING)

- **`one plugins` command** — replaced by per-domain semantic fields in the
  manifest (`env.backend`, `subprojects[].container`, `subprojects[].deploy`).
- **`pkg/plugin/` package** — deleted entirely. External importers were
  expected to be limited to internal plumbing; the package is now gone.
  Each backend (dotenv / infisical / dockerfile / kustomize / s3 / processorch /
  github-actions ci) exposes its own input / output types and top-level
  functions. Callers use inline `switch` on the backend name instead of a
  registry lookup.
- **Plugin wrapper files** — `plugin.go` and `verbs.go` adapter files in
  every backend package are gone; the underlying business logic moved to
  `sync.go` / `ops.go` (or stayed where it was, e.g. `internal/ci/ci.go`).
- **`internal/cmd/dispatch/`** package — replaced by direct backend calls.

### Changed (BREAKING)

- **manifest schema v2 → v3**:
  - The flat `plugins` map is gone. Domain selections live as
    `manifest.env.backend` (string), `manifest.ci` / `manifest.dev`
    (presence = enabled), `subprojects[].container` (presence = Dockerfile
    owned), and `subprojects[].deploy` (`{target, profile}`).
  - v2 manifests are auto-migrated on read; the next write persists v3.
- **profile schema v1 → v2**:
  - `Profile.plugin` (e.g. `"env/infisical"`) is now `Profile.backend`
    (e.g. `"infisical"`). v1 files are auto-migrated.
  - `--plugin <id>` flag on `one env|deploy profile add` is replaced by
    `--backend <bare-name>`. The `--plugin` flag remains hidden as an
    alias for one minor cycle.
  - `one env|deploy profile list / show` JSON envelopes use `backend`
    instead of `plugin` field name.
- **registry.json v1 → v2**:
  - `default_plugins: ["container/docker", ...]` → `defaults: {container: "docker", ...}`
  - `allowed_plugins: {deploy: ["deploy/kustomize"]}` → `compat: {deploy: ["kustomize"]}`
  - v1 registries are auto-migrated on read.

### Added

- **`one create --secrets <dotenv|infisical>`** flag — semantic replacement
  for the legacy `--enable env/<backend>` form. The legacy `--enable` flag
  is hidden but still accepted.

### Removed

- New top-level commands for plugin-level workflows:
  `one infisical` / `one dotenv` (secrets), `one docker` / `one compose`
  / `one k8s` / `one procs` (infra), `one prd` / `one design`
  (productivity). Each lives behind a `cliexts.Register` hook in its
  own package and is gated by `cmdgate.RequirePluginEnabled`.

### Added

- New top-level commands for plugin-level workflows:
  `one infisical` / `one dotenv` (secrets), `one docker` / `one compose`
  / `one k8s` / `one procs` (infra), `one prd` / `one design`
  (productivity). Each lives behind a `cliexts.Register` hook in its
  own package and is gated by `cmdgate.RequirePluginEnabled`.

### Deprecated

- `one env` is now an alias for `one infisical` and emits a stderr
  warning on every verb invocation. **Removal target: 0.6.** Update
  scripts to call `one infisical <verb>` directly.

### Removed

- `one plugins invoke <id> <action>` (no longer needed; every plugin
  with verbs now exposes its own top-level command).
- `pkg/plugin.Invokable` interface + `ActionSpec` struct (no consumers
  after the cliexts unification).

## [0.4.2] — 2026-05-05 — install.sh version-aware upgrade

`install.sh` now reads the version of any existing `one` binary
before deciding what to do. Upgrades are silent, downgrades are
refused unless explicitly forced, and re-running with the same
version is a no-op instead of an error. Documentation also clarifies
that `one setup` is a separate step users have to run after install
to wire the bundled skill into their coding agents — `install.sh`
intentionally does not run it.

### Changed

- **`install.sh` decision matrix**:
  - First install (no existing binary): proceed
  - Target newer than current: upgrade silently (e.g.
    `upgrading v0.4.0 → v0.4.2`)
  - Target same as current: skip with an info message; set
    `ONE_FORCE=1` to force a reinstall (useful for repairing a
    corrupted binary)
  - Target older than current: refuse with a clear error; set
    `ONE_FORCE=1` to allow the downgrade
  - Existing file at target path that won't report a version: fall
    back to the old "force or fail" behavior
- **`ONE_FORCE` semantics shifted from "always overwrite" to
  "downgrade / same-version / unrecognised-binary only"**. Plain
  upgrades no longer require `ONE_FORCE`. The script's header
  comment and the website's installation doc both spell out the new
  matrix.

### Added

- `version_compare()` and `current_version()` helpers in
  `install.sh`. Pre-release suffixes (`-rc1` etc.) are stripped
  before comparison — pin via `ONE_VERSION` if you need finer
  control.
- `internal/cli/install_sh_test.go` sentinels covering the new
  helpers and the `downgrade blocked` error string.
- Post-install `one setup` instructions in the website's
  installation guide. `install.sh` deliberately stops at putting
  `one` on PATH; users run `one setup` themselves to install the
  bundled skill into Claude Code / Cursor / Codex / etc.

## [0.4.1] — 2026-05-05 — Drop npm distribution

The `qzkpwoxtl` npm package is no longer published. The single
distribution channel is now the curl-based installer
(`website/public/install.sh`, served from Aliyun OSS) plus direct
downloads from GitHub Releases. The npm shim — `scripts/one-bin.cjs`
+ `scripts/npm-postinstall.cjs` — is gone, along with the entire
root `package.json` and the stale `pnpm-lock.yaml` left over from the
pre-v0.3.0 TypeScript implementation.

### Removed (BREAKING)

- **npm distribution discontinued.** `pnpm add -g qzkpwoxtl` /
  `npm install -g qzkpwoxtl` are no longer supported install paths.
  Existing `qzkpwoxtl@0.3.0` installations keep working (postinstall
  pulls v0.3.0 binaries from GitHub Releases), but no `qzkpwoxtl@0.4.x`
  will be published.
- `scripts/` directory (npm bin shim + postinstall hook) deleted.
- Root `package.json` deleted. The `docs:*` convenience scripts are
  gone — run `pnpm --dir website dev` (or `cd website && pnpm dev`)
  directly. Version source moved to a top-level `VERSION` file.
- Root `pnpm-lock.yaml` deleted. It was a stale leftover from the
  TypeScript implementation that ended at v0.3.0; the current Go
  source has no Node dependencies at all.

### Changed

- `Taskfile.yml` reads version from `VERSION` (`cat VERSION`) instead
  of `package.json#version`. Goreleaser still injects the actual git
  tag via `-X main.version={{.Version}}` for releases.
- `website/public/install.sh` "unsupported OS" fallback now points
  users at GitHub Releases instead of the npm package.
- README, CLAUDE.md, `skills/one-cli/SKILL.md`, `docs/tech-stack.md`,
  `docs/error-codes.md`, and the website's installation guide all
  swap install instructions from `pnpm add -g qzkpwoxtl` to the curl
  one-liner / manual download.
- `.goreleaser.yaml` release-notes header drops the npm package
  reference.

## [0.4.0] — 2026-05-05 — Manifest v2 + env auto-create

Two breaking changes ship together. Workspace-level state collapses
into `one.manifest.json` (schema v2) so `package.json` is no longer
touched by any One CLI command, and `one env init` now provisions the
Infisical project itself instead of demanding a pre-created
`--project-id`. Also adds a PR-time test gate (`task pre-push`
mirrored as `.github/workflows/test.yml`) and broad unit + E2E
snapshot coverage across the seven critical user flows.

### Changed (BREAKING)

#### `one env init` auto-creates the Infisical project

`--project-id` is no longer required. The default flow calls
Infisical's `POST /api/v2/workspace` using the workspace's
`manifest.project.name`, retrying with a 4-char hex suffix on
collision (up to 5 attempts). Resolved id + name land in
`one.manifest.json#env`; `manifest.project` is back-filled when
missing. `--project-id` still works for binding to a pre-existing
project; `--project-name` overrides the auto-create name.
`--skip-verify` is mutually exclusive with the auto-create branch.

`one env pull` stops filtering by `.env.example` — the two-layer
security contract collapses to one (Infisical folder isolation).
Subproject discovery moves from filesystem-walk to
`manifest.subprojects`, and the workspace-root target is dropped
(secrets belong to the subprojects they're scoped for). Empty
manifest now surfaces `MANIFEST_MISSING_OR_EMPTY` pointing at
`one status --fix` / `one add`.

The manifest gains an optional top-level `project` block —
workspace-level identity (id + name) generated at scaffold time,
preserved through `RebuildManifest` so `status --fix` can never
wipe it. New helper `SetManifestProject` for back-fill.

New error codes: `INFISICAL_PROJECT_NAME_TAKEN`,
`INFISICAL_PROJECT_CREATE_FORBIDDEN`, `MANIFEST_MISSING_OR_EMPTY`.

Migration: workspaces that relied on `.env.example` as an allowlist
will see all Infisical keys flow into `.env`. Move secrets at the
Infisical folder level if they shouldn't appear in a particular
subproject.

#### Workspace config consolidated into `one.manifest.json` (manifest v2)

The entire `one` field is removed from `package.json`. All workspace-level
One CLI state — `packageManager`, `ai.providers`, Infisical `env` config,
`workspace.roots` scan-dir override, and per-subproject env overrides —
now lives in `one.manifest.json` (schema bumped from v1 to v2).

- Workspace marker switched from "package.json has `one` field" to
  "`one.manifest.json` exists". `IsOneProjectRoot` is now an alias for
  `HasManifest`.
- `one create` no longer stamps an `one` block into the new workspace's
  `package.json`; it only writes `one.manifest.json` (with `packageManager`
  + `ai.providers` pre-filled).
- `one env init` writes `one.manifest.json#env` instead of
  `package.json#one.env` (or the legacy `one.secrets`). The on-disk
  `package.json` is no longer touched by any One CLI command.
- Per-subproject env overrides (path / inherits / disabled) move from
  the subproject's own `package.json#one.env` to the matching
  `one.manifest.json#subprojects[].env` entry. The legacy `"env": false`
  shorthand is replaced by `"env": {"disabled": true}`.
- Workspace scan-dirs override moves from `package.json#one.workspace.roots`
  to `one.manifest.json#workspace.roots`.
- AI provider config moves from `package.json#one.ai.providers` (which
  accepted `string | []string | {providers}`) to
  `one.manifest.json#ai.providers` (strict `[]string`).
- v1 manifests are no longer accepted — `MANIFEST_INVALID` fires on the
  version check. There are no v1 users in the wild, so no migration
  command is provided.
- Public types: `workspace.OneSection`, `workspace.WorkspaceCfg`
  (now under the `Manifest` struct), `workspace.OneMarkerKey`, and
  `workspace.PackageJSON.One` are removed. `env.LoadSubprojectConfig`
  signature changes from `(subprojectDir)` to `(projectRoot, relativeDir)`.

## [0.3.0] — Go rewrite + Infisical-backed secrets

This is a major release. Two parallel breaking changes ship together:
the entire CLI is now a single Go binary (the TypeScript implementation
is gone), and the secrets workflow has switched from SOPS+age to
Infisical. Every public command (`create`, `add`, `templates`, `status`,
`secrets`) was re-implemented in Go and verified at byte
parity against the legacy TS implementation before deletion. The secrets
subcommand surface was redesigned for monorepo-friendly Infisical paths.

### Changed (BREAKING)

#### Implementation: TypeScript → Go

- The CLI is now distributed as a single static Go binary downloaded
  from the GitHub Release at install time. The npm package
  `qzkpwoxtl@0.3.0` ships a 100-line `scripts/one-bin.cjs` shim plus a
  `scripts/npm-postinstall.cjs` that fetches the platform-matching
  archive (`darwin/linux/windows × amd64/arm64`).
- `dist/index.mjs` (the TS bundle) is gone. So is `src/`, `tests/`,
  `vitest.config.ts`, `tsconfig.json`, `tsdown.config.ts`, and every TS
  runtime + dev dependency that used to live in `package.json`.
- Local development now uses [go-task](https://taskfile.dev/) — see the
  README's "本地开发" section. `pnpm test` / `pnpm build` no longer exist.
- `--version` output remains identical; JSON output schemas are
  byte-identical to the legacy TS for every command.

#### Secrets: SOPS+age → Infisical

- `one secrets` now talks to Infisical via the official Go SDK. The
  legacy SOPS+age pipeline (`.sops.yaml`, `.secrets/keys/age.txt`,
  `.secrets/.env.<env>.enc`) is gone.
- New configuration shape — `package.json#one.secrets`:
  ```json
  {
    "one": {
      "secrets": {
        "provider": "infisical",
        "projectId": "<infisical-project-id>",
        "siteUrl": "https://app.infisical.com",
        "environments": ["dev", "staging", "prod"],
        "defaultEnv": "dev",
        "rootPath": "/"
      }
    }
  }
  ```
- New auth model: Universal Auth via `INFISICAL_UNIVERSAL_AUTH_CLIENT_ID`
  and `INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET` environment variables. We
  never store credentials on disk.
- New subcommand layout:
  - `one secrets init --project-id <id>` writes the workspace config and
    (by default) verifies the Infisical project is reachable.
  - `one secrets set <KEY> <VALUE>` / `one secrets get <KEY>` /
    `one secrets list` operate on a single key. `--path` defaults to the
    Infisical folder derived from the cwd: `cd services/user-api &&
    one secrets set DATABASE_URL ...` writes to `/services/user-api`.
  - `one secrets pull` replaces `secrets decrypt`. Walks every
    subproject, fetches the inheritance chain (root → ancestors → self)
    from Infisical, filters through each subproject's `.env.example`,
    and writes a deterministic `.env`.
- `one secrets edit` was removed (use Infisical's web UI for batch
  editing).
- `one secrets apply` was renamed to `one secrets pull` for accuracy
  and to avoid colliding with the K8s/Terraform "apply" semantics.

#### New error codes (machine identifiers)

- `INFISICAL_NOT_CONFIGURED`, `INFISICAL_AUTH_MISSING`,
  `INFISICAL_AUTH_FAILED`, `INFISICAL_PROJECT_NOT_FOUND`,
  `INFISICAL_NETWORK_ERROR`, `INFISICAL_API_ERROR`.
- `SECRETS_PULL_CONFLICT` (replaces `SECRETS_DECRYPT_CONFLICT`).
- `SECRETS_KEY_NOT_FOUND`.

#### Retired error codes

- `SECRETS_AGE_KEYGEN_FAILED`, `SECRETS_AGE_KEY_INVALID`,
  `SECRETS_COMMAND_MISSING`, `SECRETS_COMMAND_TIMEOUT`,
  `SECRETS_DECRYPT_FAILED`, `SECRETS_DECRYPT_CONFLICT`,
  `SECRETS_EDITOR_NOT_SET`, `SECRETS_EDIT_FAILED`,
  `SECRETS_ENCRYPT_FAILED`, `SECRETS_ENV_FILE_NOT_FOUND`. Agents that
  branched on these codes need to migrate to the `INFISICAL_*` codes
  or the new `SECRETS_PULL_*` codes above.

#### New JSON schemas

- `one-cli/secrets-init/v1` — payload reshaped: `project_id`,
  `site_url`, `environments`, `default_env`, `root_path`, `auth_status`,
  `written_to`.
- `one-cli/secrets-pull/v1` — replaces `one-cli/secrets-decrypt/v1`.
  Includes `per_subproject[]` with `name`, `relative_dir`,
  `infisical_path`, `env_file_path`, `status`, `keys_written`.
- `one-cli/secrets-get/v1`, `one-cli/secrets-list/v1` — new.

### Migration

For workspaces previously using SOPS+age:

1. Run `one secrets init --project-id <new-infisical-project-id>` to
   stamp the new config block into `package.json`.
2. Manually re-create your secret values inside Infisical (the SDK
   doesn't have a "convert .env.enc" path; we don't ship a migration
   tool because the canonical move is "encrypt at the new source").
3. Delete `.sops.yaml`, `.secrets/`, and any `.env.*.enc` files.
4. Make sure `INFISICAL_UNIVERSAL_AUTH_CLIENT_ID` and
   `INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET` are set in CI.
5. `one secrets pull --env <env>` populates the subproject `.env` files.

The two-layer security model is preserved: Infisical folders provide
the first isolation layer, and `.env.example` is still the per-subproject
allowlist that determines which keys land in each `.env`.

### Distribution

- Binaries cut by `goreleaser` on every `v*` tag via the new
  `.github/workflows/release-go.yml` workflow.
- npm package shape on disk after install:
  ```
  node_modules/qzkpwoxtl/
  ├── scripts/
  │   ├── one-bin.cjs          (the npm bin entry)
  │   └── npm-postinstall.cjs  (downloaded the binary)
  ├── bin-go/
  │   └── one                  (Go binary, platform-matched)
  ├── README.md
  └── CHANGELOG.md
  ```
- `ONE_BINARY_PATH` env var lets CI / dev use a pre-placed binary.
- `ONE_SKIP_POSTINSTALL=1` skips the download step entirely (for
  air-gapped environments).

### Added — interactive prompts (charmbracelet/huh)

The Go port previously bailed at `if !flags.yes` for every command that
needed user input, forcing humans into the same flag-only path as agents.
This release restores the wizard-style flow the TS build had:

- `one create` prompts for project name (when missing), Docker / K8s
  toggles (when neither `--docker`/`--no-docker` etc. set), and shows a
  3-way "cancel / overwrite / ignore" picker when the target directory
  already has content.
- `one add` prompts for template selection (rendered with toolchain +
  category badges and the registry's description) and subproject name.
- `one secrets set <KEY>` prompts for the value with masked echo when
  the second positional is omitted, instead of failing with
  `SECRETS_SET_VALUE_REQUIRED`.
- `one secrets init` prompts for `--project-id` when missing.

Triggers are deliberate three-state: `--yes` always disables prompts,
`--json` (or any pipe → non-TTY → auto JSON) always disables prompts,
and only `TTY && !--yes` activates them. CI / agent paths are
unaffected; existing snapshot tests passed without modification.

`PROMPT_CANCELLED` (Ctrl-C from a prompt) emits the JSON envelope but
exits 0 — graceful cancel matches the TS behaviour. New `output.Error`
field `Exit0` carries that signal from `internal/prompt` to `main()`.

### Also in 0.3.0 — command surface refactor (rolled in from the unreleased pivot)

These changes were staged on `master` before the Go rewrite started and
ship together with 0.3.0. The notes here originally lived under an
`[Unreleased]` heading; nothing else changed.

#### Changed (BREAKING)

- Renamed three commands to remove naming ambiguity:
  - `one list` → `one templates`
  - `one inspect` → `one status`
  - `one secrets apply` → `one secrets decrypt`
  (`secrets decrypt` was further renamed to `secrets pull` in the
  Infisical migration above.)
- JSON schema ids bumped accordingly:
  - `one-cli/list/v1` → `one-cli/templates/v1`
  - `one-cli/inspect/v1` → `one-cli/status/v1`
  - `one-cli/secrets-apply/v1` → `one-cli/secrets-decrypt/v1`
  (Then `secrets-decrypt/v1` → `secrets-pull/v1`.)
- Old names now error with `UNKNOWN_COMMAND`. Update scripts and skills.
- Removed implicit `one <project-name>` fallback to `one create`. Unknown
  first arg now errors with `UNKNOWN_COMMAND` instead of treating the arg
  as a project name. Use `one create <project-name>` explicitly.
- Merged `--no-interactive` into `--yes` on `create` and `add`. Single flag
  now means "non-interactive: skip prompts, use defaults". TTY auto-detect
  still applies — pipe / non-TTY automatically enables non-interactive mode.
  The `--interactive` and `--no-interactive` flags are gone.

### Fixed

- Typo: 模版 → 模板 across registry, docs, error messages.
- `one add --dir` description was wrong ("骨架项目目录（用于 add 调试）" →
  "工作区目录（默认当前目录）").
- Cleaned the marketing parenthetical from `one secrets` description.



This unreleased range turns the CLI from a "human terminal app with some
`--json` flags" into an AI Native tool whose primary consumer is an AI agent
(Claude Code, Codex, etc.). Humans still get the existing TTY experience;
agents get structured JSON I/O, machine-readable error recovery, a small
command surface, and workflow skills.

### Added

#### Phase 1 — JSON-first CLI (`a5835c8`)

- New `src/utils/output.ts` module with `printResult(schema, data)`,
  `printError(error)`, `tty()` helper, and a global output mode
  (`tty` / `json`) detected at startup.
- Output mode is decided by, in priority order: `--json` flag,
  `ONE_OUTPUT=json|tty` env var, then `process.stdout.isTTY`.
- All 14 commands now accept `--json` and emit a versioned schema
  (`one-cli/<command>/v1`) at every successful exit point.
- `OneCliError` extended with optional `context: Record<string, unknown>`
  and `remediation: Remediation[]` fields, plus a `toJSON()` method that
  serializes errors as `{schema: "one-cli/error/v1", error: {...}}` to
  stderr in JSON mode.
- Structured remediation added at high-traffic throw sites:
  `NOT_ONE_PROJECT`, `TEMPLATE_NOT_FOUND`, `TEMPLATE_REQUIRED`,
  `SUBPROJECT_NAME_REQUIRED`, `TARGET_EXISTS`, `PROJECT_NAME_REQUIRED`,
  `EXISTING_TARGET_NOT_EMPTY`, `INVALID_NAME`, `DOCTOR_FAILED`,
  `NODE_VERSION_UNSUPPORTED`.

#### Phase 2 — `one inspect` (`ffa3b81`)

- New `one inspect` command that returns the full structured state of a
  workspace as JSON. Schema: `one-cli/inspect/v1`.
- Includes per-subproject `scripts[]`, `manifest_tracked`, and an
  `infra` block (`dockerfile / compose_service / k8s_workload /
  github_workflow`).
- Includes a top-level `available_actions[]` field that ranks
  recommended next steps with concrete commands and reasons. This is
  the AI Native value-add: agents read it instead of guessing.
- New `src/core/inspect.ts` consolidates manifest + report + status
  logic into a single inspectable result.
- 4 new unit tests covering empty / fresh / drift / scripts scenarios.

#### Phase 3 — Workflow skills (`77ae545`)

- Replaced the single 90-line `skills/one-cli/SKILL.md` with 5 focused
  workflow skills, each in the agent-skills format with frontmatter,
  numbered workflow, error recovery table, and 3+ examples:
  - `skills/one-bootstrap/` — create a new workspace from scratch
  - `skills/one-add-feature/` — add a subproject to an existing workspace
  - `skills/one-fix/` — diagnose and auto-fix workspace drift
  - `skills/one-upgrade/` — safe dependency upgrades with dry-run + tests
  - `skills/one-reference/` — passive command + JSON schema + error code
    reference
- `skills/` is now in the npm `files` array so the directory ships with
  the package; `one create` installs the bundled skills into
  `~/.claude/skills/`.

#### Phase 4 — Simplified human help (`cafaa08`)

- Custom `src/index.ts` help renderer intercepts root `--help` and shows
  only 5 curated commands: `create`, `add`, `list`, `inspect`, and
  `status`.
- New `--help-all` flag also shows the hidden advanced `secrets` command.
- Hidden commands still work — they're just not in the default help.

#### Per-template engineering standards (`4dec51d`)

- Each `templates/<id>/ai/common.md` expanded from ~5 lines of high-level
  Chinese principles to 110-150 lines of structured English engineering
  standards covering architecture boundaries, engineering discipline,
  testing conventions, code style, anti-patterns, and a mandatory
  quality-gates checklist.
- 7 templates × ~120 lines = 941 lines of new engineering content.
- Standards are stack-specific: NestJS (Controller/Service/Repository,
  DTOs, BusinessException, pino), React (hooks at top level, design
  tokens, zustand), Next.js (Server vs Client Component budget,
  Server Actions), Astro (static-first, client:* sparingly), Starlight
  (lean into conventions, MDX content collections), Expo RN (NativeWind,
  expo-secure-store, expo-router file routing), ts-library (semver
  enforcement, JSDoc on all public exports).
- Picked up automatically when `one add` refreshes `AGENTS.md` /
  `CLAUDE.md`.

#### Lean `one create` and removed init module layer

- `one create` now creates the workspace and installs bundled agent
  skills directly. It no longer exposes `--modules`, `--add`, `--all`,
  or `--minimal`.
- Removed the internal `first-subproject`, `ai-guides`, and `install`
  module layer. Add subprojects with `one add`; install dependencies
  with the package manager.
- `one add` refreshes `AGENTS.md` / `CLAUDE.md` after adding a
  subproject. `one status --fix` repairs missing AI guides.
- The `one-bootstrap` skill now teaches agents the explicit flow:
  `one create`, then one or more `one add` calls, then `pnpm install`.

#### Secrets module overhaul (`ddcfd7c`)

- **NEW commands**: `one secrets list` (lists envs + key names without
  leaking values), `one secrets get` (read single value),
  `one secrets unset` (remove a key).
- New JSON schemas: `one-cli/secrets-list/v1`, `one-cli/secrets-get/v1`,
  `one-cli/secrets-unset/v1`.
- All 13 secrets errors now carry `context` and `remediation` fields.

### Changed

- `one create` no longer creates the `EXAMPLE_KEY=change_me` placeholder
  in `.secrets/.env.dev.enc`. Init creates only the config files; users
  add their first secret explicitly.
- `one secrets edit` no longer pre-fills new env files with
  `NEW_KEY=replace_me`.
- `one secrets materialize` now compares dotenv files **semantically**
  (parse + compare records) instead of byte-by-byte, so trailing
  newlines / BOM / `\r\n` no longer trigger spurious "conflict".
- `assertCommandAvailable` is now cached for the lifetime of a single
  CLI invocation — no more repeated `sops --version` /
  `age-keygen --version` per secrets call.
- `runProcess` (in core/secrets.ts) now has a 60s default timeout for
  non-interactive child processes. Interactive `secrets edit` calls
  remain unbounded.
- `.secrets/.gitignore` is now additive (preserves user-added entries
  via merge instead of overwrite).
- The 7 templates' shipped `pnpm-lock.yaml` files were deleted; the
  `template-check` policy reversed from `MISSING_LOCKFILE` (error) to
  `LOCKFILE_PRESENT` (warning) to discourage re-shipping them.
- The 5 missing `packageManager: "pnpm@10.14.0"` fields were added to
  `nestjs-api`, `expo-mobile`, `starlight-docs`, `astro-site`,
  `nextjs-app`.
- Dependency version style unified: `nextjs-app` switched pinned
  `next/react/react-dom` to caret; `starlight-docs` switched
  pinned `@astrojs/check` to caret; `ts-library` replaced 3 `latest`
  deps with concrete caret versions; `react-spa` switched
  `typescript: ~5.9.3` → `^5.9.3` and removed a stray `pnpm` self-
  reference from devDependencies.

### Security

- **`materialize` per-target filtering**: previously
  `one secrets materialize` wrote the **entire** decrypted secret
  bundle to every subproject with a `.env` or `.env.example`. This
  meant a backend's `DATABASE_URL` / `JWT_SECRET` ended up in
  `apps/web/.env`, `apps/dashboard/.env`, etc., and could trivially be
  bundled into a frontend client build via `import.meta.env` /
  `NEXT_PUBLIC_*` / similar. The bundle is now filtered per-target by
  reading each directory's `.env.example` as the explicit allowlist.
  Subprojects without `.env.example` are skipped entirely (with a
  reason reported in the result).
- **`ensureAgeKey` forces 0600** on the freshly-generated private
  key, regardless of the user's umask. Previously the file's
  permissions depended on `age-keygen`'s default which respects
  umask — on systems with `umask 022` the key was group-readable.
- **`.secrets/.gitignore` is created BEFORE the private key**, so a
  fresh key cannot transiently exist in a tracked location.
- **Workspace root `.gitignore`** now lists `.secrets/keys/` as a
  belt-and-suspenders backup to the nested gitignore.
- **Sensitive paths sanitized**: `templates/expo-mobile/app.json`'s
  Android package id changed from `com.caorushizi.rntemplate` to
  `com.example.rntemplate`; the demo `setUser("caorushizi")` in
  `templates/expo-mobile/src/app/(tabs)/index.tsx` changed to
  `"demo-user"`.

### Removed

- **`one sync` command** — completely subsumed by `one status --fix`.
  `runSyncCommand` and `syncAllSubprojects` were moved from
  `src/commands/sync.ts` to `src/core/sync.ts` for internal reuse by
  `status --fix`; the user-facing command is gone.
- **`one secrets run`** — the function existed in `src/commands/secrets.ts`
  with a unit test, but the subcommand was never registered in citty's
  `subCommands` object. The entire surface (`runSecretsRunCommand`,
  `runWithSecrets`, `extractCommandAfterDoubleDash`, error codes
  `SECRETS_RUN_*`) was removed as dead code.
- **7 `pnpm-lock.yaml`** files in the templates (one per template).
- **`templates/nestjs-api/.oxlintrc.json`** and `.oxfmtrc.json` — both
  contained literally `{}` with zero customization, only causing
  inconsistency with the other 6 templates that rely on oxc defaults.

### Fixed

- The `react-spa/.gitignore` had `.env.sentry-build-plugin`
  duplicated on consecutive lines.

### Refactored

- `src/index.ts` shrank from 811 lines to 72 lines by moving
  `defineCommand` blocks into the respective `commands/*.ts` files
  and replacing the manual if/else dispatch with citty's native
  subcommand routing.
- New helpers `src/utils/args.ts` (`pickString`, `pickBoolean`,
  `JSON_FLAG_DEF`) and `src/utils/safe-run.ts` (`executeSafely`)
  eliminate boilerplate previously repeated in every command.
- `create` and `add` gained explicit `--yes`, `--docker`, `--k8s`,
  `--overwrite`, `--ignore`, `--interactive` / `--no-interactive`
  flags for non-interactive use, with TTY auto-detection and clear
  errors when required input is missing in non-interactive mode.
- `one create`'s "existing target dir is not empty" prompt grew a
  three-option choice (cancel / overwrite / ignore) matching
  create-vite's UX.

## [0.1.0] — 2026-04-07

Initial public release on npm under the name `qzkpwoxtl` (deliberately
obscure name; the brand is "one cli", the npm package is for internal
team use). See git history for the early scaffolding work.
