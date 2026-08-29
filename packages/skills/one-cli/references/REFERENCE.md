# Mode: Reference

Machine-readable manual for One CLI. Use this file as the current command
surface source of truth for agents and routing code.

For real workflows, prefer `bootstrap.md` and `add-feature.md`. Use this
file when you need exact commands, flags, schemas, or removed-command
replacements.

## Universal conventions

Every command supports `-o json` / `-o yaml` / `-o text`. The CLI auto-switches
to JSON when stdout is non-TTY. JSON output is pretty-printed (2-space indent);
YAML output uses the same envelope schema. Pass `-o text` to force
human-readable output even when piping (rare; useful for `tee` while keeping
colours). YAML is always opt-in — auto-detection never picks YAML.

- Success envelopes use `{"schema":"one-cli/<command>/<version>", ...}`.
  v0.5+: `create / container-info / container-build` bumped to `v2`
  with backend-named fields (`secrets_backend`, `container_backend`,
  `deploy_backend`); other commands still on `v1`.
- Errors are emitted to stderr as `one-cli/error/v1`; always read
  `error.code`, then `error.context`, then `error.remediation[]`.
- Exit code `0` means success or graceful cancel; exit code `1` means read the
  structured error payload.

## Current top-level commands

```bash
one create
one add
one templates
one env
one container
one dev
one deploy
one ci                               # optional GitHub Actions workflows
one run
one configure                        # local connections and preferences
one configure open                   # local settings page
one configure add <pair> --profile <name> [backend flags...] [--use]
one configure list [pair]            # omit pair → aggregate all sections
one configure current [pair]         # omit pair → aggregate all default profiles
one configure show <pair> --profile <name> [--reveal]
one configure use <pair> --profile <name>
one configure remove <pair> --profile <name>
one configure locale [auto|zh-CN|en-US]
one serve                            # loopback-only web UI for human profile editing
one skills install
```

## `one create [dir]`

Creates an empty workspace root. It does not create a project, install package
dependencies or Coding Agent Skills, or modify local AI-tool settings.

Flags:

- `-n, --name <name>`: workspace name; default is `basename(dir)`.
- `-y, --yes`: non-interactive defaults.
- `--env-provider <dotenv|infisical>`: advanced automation override. Ordinary
  creation always uses local dotenv without prompting.
- `--preset <id>`: scaffold workspace + projects + deploy + env from a
  reproducible preset id. Implies non-interactive mode and requires `[dir]`.
- `-o json`: structured output.

Workspace defaults written to `one.manifest.json` (schema v1):

- `domains.env = { kind: "dotenv" }`
- `environments = { names: ["dev","staging","prod"], default: "dev" }`
- CI is not configured automatically. `one add` writes the project's resolved
  dev command into `projects[].domains.dev.command` so `one dev` can read it.

`container` and `deploy` are not configured at create or ordinary add time.
They land in the project only after first deployment setup.

Schema: `one-cli/create/v2`.

```json
{
  "schema": "one-cli/create/v2",
  "project_name": "demo",
  "created_path": "/abs/demo",
  "created_in_place": false,
  "package_manager": "pnpm",
  "secrets_backend": "dotenv",
  "ci_enabled": false,
  "dev_enabled": true,
  "skills": {
    "status": "skipped",
    "reason": "manual-install"
  }
}
```

Run `one skills install` separately when the user explicitly wants it.

## `one add <template-id> --name <name>`

Adds a locally developable project from the technology-stack registry and
records it in `one.manifest.json#projects[]`.

Flags:

- `-n, --name <name>`: project name; required in non-interactive mode.
- `-y, --yes`: non-interactive mode.
- `--deploy-provider <id>`: advanced automation path that configures deployment
  immediately (`kustomize` /
  `aliyun-oss` / `tencent-cos` / `aws-s3` / `minio` / `rustfs` / `r2` /
  `vercel` / `cloudflare` / `edgeone`). Value must be compatible with the stack.
- `-o json`: structured output.

Without the advanced flag, deployment/container fields and artifacts remain
absent until `one deploy <project>`.

Schema: `one-cli/add/v1`.

## `one ci`

Continuous integration is opt-in. `one create` and `one add` never generate a
workflow. The standalone commands are:

```bash
one ci                              # status, schema one-cli/ci-status/v1
one ci enable [project]             # schema one-cli/ci-enable/v1
one ci sync [project]               # schema one-cli/ci-sync/v1
one ci disable [project] [--yes]    # schema one-cli/ci-disable/v1
```

The current bundled service is GitHub Actions (`ci/github-actions`). `enable`
without a project covers all projects. `sync` without a project refreshes only
projects that already have a generated workflow. `disable` without a project
removes all canonical generated workflows and confirms in an interactive
terminal; cancellation exits 0 and leaves files unchanged. Non-interactive
calls must pass `--yes`; otherwise the command returns
`CI_DISABLE_CONFIRMATION_REQUIRED` without deleting a workflow.

`--provider ci/github-actions` and legacy `--project <name|path>` are advanced
automation options. CI state is represented by canonical workflow files under
`.github/workflows/`; it is not persisted in `one.manifest.json`.

## `one templates` (== `one templates list`)

Lists bundled templates. Bare `one templates` and `one templates list`
are equivalent — both emit the same envelope. The explicit `list` form
exists so future `templates show <id>` etc. can fit naturally.

Schema: `one-cli/templates/v1`.

## Reading workspace state

Bare `one` inside a workspace is the read-only overview; `one -o json` emits
`one-cli/workspace-summary/v1`. Read `one.manifest.json` when automation needs
the detailed model.
Key fields (schema v1):

- top-level: `version`, `workspace.{id,name}`, `environments.{names,default}`,
  `domains.{env,deploy,container}`, `projects[]`.
- `domains.<name>`: `{ kind, config? }` — workspace-level backend
  selection. `kind` is the bare backend id (`dotenv` / `infisical` /
  `kustomize` / `docker` / ...). `config` is a kind-specific JSON blob.
- `projects[]`: `name`, `relativeDir`, `templateId`, `toolchain`,
  `buildVersion`, `packageManager`, `domains`.
- `projects[].buildVersion`: per-project build version stored without a
  leading `v` (for example, `0.1.0`); container/deploy commands turn it into
  a Docker tag such as `v0.1.0`.
- `projects[].domains.env`: override-only override (`path` / `inherits` /
  `disabled` / `keys`). No `kind` — env backend is always inherited from
  workspace.
- `projects[].domains.container`: `{ kind?, image?, namespace? }` when a
  Dockerfile-driven build is configured. Empty `kind` inherits the workspace
  container backend and ultimately defaults to `docker`.
- `projects[].domains.deploy`: `{ kind: "kustomize" | "aliyun-oss" |
  "tencent-cos" | "aws-s3" | "minio" | "rustfs" | "r2" | "vercel" |
  "cloudflare" | "edgeone", config? }`. Per-kind fields land in
  `config` (`bucket` for S3-compatible backends, `projectId` for vercel,
  `workerName` for cloudflare, `env` for every kind). The `config.env` field draws from
  `manifest.environments.names`; unset / `"prod"` ships to production,
  anything else ships to a preview (or, for kustomize, selects the
  matching overlay directory).

To list deployment artifacts such as kustomize overlays, inspect the filesystem
next to the manifest. `one ci -o json` summarizes generated CI workflows. The
canonical source of truth is the manifest plus the filesystem. Dev commands are
recorded inside the manifest at `projects[].domains.dev.command`.

## `one env`

Dispatches to whichever backend `manifest.domains.env.kind` names.

Subcommands:

- `one env get <KEY>`
- `one env set <KEY> [VALUE]`
- `one env list`
- `one env pull`
- `one env switch <dotenv|infisical>`

Machine-level Infisical credentials live under
`one configure add env/infisical --profile <name>`. `dotenv` is the
local-file backend and does not need a machine profile.

Backend support:

- `dotenv`: local `.env` get/set/list. It intentionally does not support
  pull because there is no remote source.
- `infisical`: get/set/list/pull. Requires a configured `env/infisical`
  profile and valid Infisical credentials for remote operations.

Schemas: `one-cli/env-{get,set,list,pull,switch}/v1`.

## `one container`

Operates on per-project `projects[].domains.container` sections
(Dockerfile-driven; today only one canonical container builder).

Subcommands:

- `one container info [-d <workspace>]`: list all container targets and artifact
  state. Schema: `one-cli/container-info/v2`.
- `one container build [project] [-p <name|path>] [-d <workspace>] [--tag <tag>] [--dry-run] [--profile <name>]`:
  build one target or all targets. Bare build is local by default and uses
  `<workload>:<version>` without logging into a registry. The optional
  positional argument or `-p` selects one project by manifest name or
  relative path. A registry profile is used only when `--profile` or a
  manifest-pinned container profile is present; then the image tag becomes
  `<registry>/[<namespace>/]<workload>:<version>` and `docker login` runs before
  the first build. Version defaults to `projects[].buildVersion` (stored
  without `v`, rendered as a Docker tag like `v0.1.0`); TTY mode prompts for
  Current / Patch / Minor / Major / Custom version, and successful real builds
  write the selected version back. Explicit `--tag` is for scripts and CI.
- `one container push [project] [-p <name|path>] [-d <workspace>] [--tag <tag>] [--dry-run] [--profile <name>]`:
  push to the registry. Requires a container profile. If the registry-qualified
  image tag is absent locally but the matching bare local tag exists, push first
  runs `docker tag <workload>:<version> <registry>/.../<workload>:<version>`,
  then pushes. Schema: `one-cli/container-push/v1`.

Container registry credentials are managed under
`one configure add container/<kind> --profile <name>` (top-level profile tree).

Profile resolution order (build / push):
1. `--profile <name>` (one-shot override)
2. `~/.config/one/config.json#workspaces[workspaceId].projects[project].profiles[container/kind]`
3. `~/.config/one/config.json#workspaces[workspaceId].profiles[container/kind]`
4. `~/.config/one/config.json#container/<kind>.default` (machine default)

Configure once with `one configure add container/<kind> --profile <name>`;
the tag and login flow then "just work" across `build` and `push`.

Container commands become available after deployment setup (or after the
advanced `one add --deploy-provider ...` automation path) writes the project's
container configuration.

## `one dev`

Reads `projects[].domains.dev.command` from `one.manifest.json` and
runs every project in parallel through the One CLI built-in supervisor.
No third-party Procfile runner is required.

```
one dev [project] [--dry-run]
```

The positional project restricts the supervisor to one project. `-p /
--project` remains compatible for older scripts (selector
accepts the manifest project name or its relative path; same shape as
`one deploy -p`). `--dry-run` prints the resolved commands without
launching them.

If Node dependencies are missing, an interactive terminal offers to run the
detected package manager's install command and then continues. Non-interactive
calls return `DEPENDENCIES_NOT_INSTALLED` with the exact install command.

Each project's command is internally wrapped as
`one run -p <relativeDir> -- <command>` so the workspace's env backend
(dotenv or Infisical) injects variables into the dev process. The
supervisor handles signal forwarding, process-group cleanup (npm/node
grandchildren), and TTY coloring per workload.

The dev command is derived at `one add` time:

- Node projects: `scripts.dev` → `scripts.start:dev` → `scripts.start`
- Go projects: `go run ./cmd/server`
- Nothing matches: the project is skipped by `one dev`

The manifest is the source of truth — re-running `one add` will
overwrite the command, but editing `projects[].domains.dev.command`
directly is supported. There is no auto-sync from `package.json`.

## `one deploy`

Leaf verb — running it dispatches per project using
`projects[].domains.deploy.kind`.

```
one deploy [project] [--provider <target>] [--profile <connection>] [--env <env>] [--dry-run]
```

The positional project filters to a single project; `-p / --project` remains
for compatibility. On first deployment, interactive use asks for project,
target category, compatible implemented service, and a local connection. A
"configure later" choice exits 0 without changing the workspace and prints
the exact recovery command. Scripts pass `--provider` and `--profile`.
`--env <name>` overrides the deploy target environment for every
project this run (must exist in `manifest.environments.names`); without
it, each project uses its own `projects[i].domains.deploy.config.env`
pin and defaults to `prod` when unset. `--dry-run` prints the docker /
kubectl / s3 argv that would execute, without touching registries,
clusters, or buckets.

Target env mapping per backend:

| Backend | `env="prod"` (default) | other env names |
|---|---|---|
| `vercel` | `vercel deploy --prod` (production tier) | preview deploy (`--environment=preview`) |
| `cloudflare` | `wrangler deploy` (implicit production) | `wrangler deploy --env=<env>` (named environment in wrangler.toml) |
| `edgeone` | `edgeone pages deploy` (production) | `edgeone pages deploy --env=preview` |
| `kustomize` | overlay = `kustomize/overlays/prod` | overlay = `kustomize/overlays/<env>` (overrides `manifest.domains.deploy.config.kustomizationPath`) |
| S3-compatible (`aliyun-oss` / `tencent-cos` / `aws-s3` / `minio` / `rustfs` / `r2`) | unchanged | unchanged |

Endpoint credentials are managed under
`one configure <verb> deploy/<backend>` (use one of the six S3-compatible
ids, or `deploy/kustomize` / `deploy/vercel` / `deploy/cloudflare` /
`deploy/edgeone`).

Profile resolution per target:

1. `--profile <name>`
2. `~/.config/one/config.json#workspaces[workspaceId].projects[project].profiles[deploy/backend]`
3. `~/.config/one/config.json#workspaces[workspaceId].profiles[deploy/backend]`
4. `~/.config/one/config.json#deploy/<backend>.default`

Manifest files never store local profile names. Use
`one configure use <pair> --profile <name> --workspace` to bind a
profile for the current workspace, or add `--project <name|path>` for a
single project.

Backend behavior:

- `kustomize`: auto-builds and pushes
  the project image, syncs the overlay image/namespace, then runs
  `kubectl apply -k`. With `--dry-run` it prints docker build / docker
  push / kubectl argv without contacting the cluster.
- S3-compatible backends (`aliyun-oss` / `tencent-cos` / `aws-s3` /
  `minio` / `rustfs` / `r2`; one of them is the default for static
  frontends, depending on the template): all six share the same
  implementation. Runs the build, walks `dist/`, ensures the bucket
  exists (HeadBucket, then CreateBucket when missing), and uploads
  through the S3-compatible endpoint. With `--dry-run` prints the upload
  plan argv.

Deploy targets (k8s namespace override, kustomize overlay path, S3 bucket
override) are NOT on the deploy profile — the profile only carries
machine-level identity:

| Field | Lives in | Why |
|---|---|---|
| `manifest.workspace.id` | workspace manifest | default k8s namespace and S3 bucket for new workspaces |
| `manifest.domains.deploy.config.namespace` | workspace manifest | optional explicit k8s namespace override |
| `manifest.domains.deploy.config.kustomizationPath` | workspace manifest | overlay path is workspace-shared |
| `projects[i].domains.deploy.config.bucket` | per-project manifest | optional S3 bucket override |
| `profile container/<kind>.namespace` | machine profile | default registry owner / org |
| `projects[i].domains.container.namespace` | per-project manifest | optional override for that workload |

Kustomize namespace defaults to `workspace.id`; set
`manifest.domains.deploy.config.namespace` only when the cluster
namespace must differ. Kustomize overlay path defaults to
`kustomize/overlays/prod`; change
`manifest.domains.deploy.config.kustomizationPath` only when the
workspace uses a different overlay. S3 bucket defaults to `workspace.id`;
set `projects[i].domains.deploy.config.bucket` only when object storage
uses a different bucket name.

Profile add examples (machine-level only):

```bash
one configure add deploy/kustomize --profile prod-k8s \
  --kubeconfig ~/.kube/config \
  --kubeconfig-context prod --use

one configure add deploy/aliyun-oss --profile web-prod \
  --endpoint https://oss-<region>.aliyuncs.com \
  --region <region> \
  --access-key-id "$AK" --access-key-secret "$SK" --use
# Other S3-compatible ids use the same flag shape — swap aliyun-oss for
# tencent-cos / aws-s3 / minio / rustfs / r2 and adjust endpoint/region.

one configure add container/ghcr --profile ghcr \
  --namespace "$GITHUB_USER" \
  --username "$GITHUB_USER" --password "$GHCR_PAT" --use
```

## `one run`

Injects project env vars and executes a command.

Usage:

```bash
one run [-p <name|path>] [--env-provider dotenv|infisical] \
  [--env <name>] -- <cmd> [args...]
```

`-p / --project` selects a project by manifest name (`-p web`) or relative
path (`-p apps/web`); without it, the cwd is used (must be inside a
project). `--env-provider` can force `dotenv` or `infisical`; when omitted,
the workspace's selected env backend is used. The child process always runs
from the resolved project directory.

## `one configure`

Top-level command for the profile lifecycle. Sole entry point for
adding, listing, switching, inspecting, and removing endpoint
credentials.

Tree shape (verb-first): `one configure <verb> <pair> --profile <name>`.
Each verb is a top-level subcommand of `configure`; `<pair>` is a
positional arg (`env/infisical` / `deploy/aliyun-oss` /
`deploy/tencent-cos` / `deploy/aws-s3` / `deploy/minio` /
`deploy/rustfs` / `deploy/r2` / `deploy/kustomize` / `deploy/vercel` /
`deploy/cloudflare` / `deploy/edgeone` / `container/docker` /
`container/dockerhub` / `container/ghcr` / `container/acr`). `add`
further nests each backend as a sub-subcommand so its `--help` only
shows that backend's flags. `env/dotenv` has no profile command.

| Pair | Purpose |
|---|---|
| `env/infisical` | Infisical SaaS endpoint + Universal Auth credentials |
| `deploy/aliyun-oss` | Aliyun OSS endpoint + AK/SK (S3 protocol) |
| `deploy/tencent-cos` | Tencent COS endpoint + AK/SK (S3 protocol) |
| `deploy/aws-s3` | AWS S3 region + AK/SK (endpoint blank for the SDK default) |
| `deploy/minio` | MinIO self-host endpoint + AK/SK (path-style addressing) |
| `deploy/rustfs` | RustFS self-host endpoint + AK/SK (path-style addressing) |
| `deploy/r2` | Cloudflare R2 endpoint + AK/SK |
| `deploy/kustomize` | kubeconfig context for `kubectl apply` |
| `deploy/vercel` | Vercel API token (+ optional team scope) |
| `deploy/cloudflare` | Cloudflare API token (+ optional account scope) |
| `deploy/edgeone` | Tencent EdgeOne Pages API token |
| `container/docker` | generic registry endpoint + Basic-auth credentials |
| `container/dockerhub` | Docker Hub credentials |
| `container/ghcr` | GitHub Container Registry credentials |
| `container/acr` | Aliyun ACR credentials |

### Verbs

| Verb | Purpose | Schema |
|---|---|---|
| `add <pair> --profile <name>` | upsert; `status=completed` for fresh, `status=updated` when overwriting an existing entry | `one-cli/configure-add/v1` |
| `list [pair]` | list profiles; omit pair to roll up every section | `one-cli/configure-list/v1` (single) / `one-cli/configure-list-all/v1` (aggregate) |
| `current [pair]` | print the default profile name; omit pair to roll up every section | `one-cli/configure-current/v1` / `one-cli/configure-current-all/v1` |
| `show <pair> --profile <name> [--reveal]` | dump the profile (credentials masked unless `--reveal`) | `one-cli/configure-show/v1` |
| `use <pair> --profile <name>` | flip the section's default pointer, or bind a workspace with `--workspace` / `--project` | `one-cli/configure-use/v1` |
| `remove <pair> --profile <name>` | delete a profile (clears default if it pointed there) | `one-cli/configure-remove/v1` |

Bare `one configure` (or `one configure add` with no pair) drops into
an interactive wizard: pick `<pair>`, then walk the corresponding
backend's prompts. Non-TTY callers must pass the pair explicitly.

### add: per-backend flag set

Each backend's `add` accepts both interactive prompts (TTY) and
non-interactive flag overrides:

- `one configure add env/infisical --profile <name> --site-url <url> --client-id <id> --client-secret <s> [--use]`
- `one configure add deploy/<kind> --profile <name> --endpoint <url> --region <r> --access-key-id <ak> --access-key-secret <sk> [--force-path-style] [--use]` where `<kind>` is one of `aliyun-oss` / `tencent-cos` / `aws-s3` / `minio` / `rustfs` / `r2`
- `one configure add deploy/kustomize --profile <name> --kubeconfig <path> --kubeconfig-context <ctx> [--use]`
- `one configure add container/docker --profile <name> --registry <host> --namespace <owner> --username <u> --password <p> [--use]`
- `one configure add container/dockerhub|container/ghcr --profile <name> --namespace <owner> --username <u> --password <p> [--use]`
- `one configure add container/acr --profile <name> --region <region> --namespace <owner> --username <u> --password <p> [--use]`

Add is upsert: re-running with the same name overwrites the entry
(useful for credential rotation). `--use` forces the default pointer
to flip; otherwise the first profile in a section auto-becomes default.

`add` writes only machine-level fields. Deploy targets belong to the workspace
manifest: k8s namespace defaults to `project.id`, kustomize overlay path
defaults to `kustomize/overlays/prod`, and S3 bucket defaults to `project.id`
for S3-backed templates. Container namespace is allowed in the
container profile as the default owner/org and can still be overridden per
project.

### add output shape

```json
{
  "schema": "one-cli/configure-add/v1",
  "status": "completed | updated",
  "domain": "env | deploy | container",
  "backend": "infisical | aliyun-oss | tencent-cos | aws-s3 | minio | rustfs | r2 | kustomize | vercel | cloudflare | edgeone | docker | dockerhub | ghcr | acr",
  "name": "work",
  "default": true,
  "config_path": "~/.config/one/config.json",
  "credentials_path": "~/.config/one/credentials.json"
}
```

### On-disk format

Schema v1 splits AWS-style into two files (both mode 0600):

```
~/.config/one/
├── config.json         非敏感:endpoint / region / default 指针 / workspace 绑定 / credentialSource
├── credentials.json    敏感:clientSecret / accessKeySecret / registry password
└── cache/              短期 token 缓存(Infisical access token 等)
    └── <domain>/<backend>/<profile>.json
```

`config.json` 顶层结构(每个 (domain, backend) 一节,default 指针互不
干扰):

```json
{
  "version": 1,
  "workspaces": {
    "workspace-id": {
      "name": "my-workspace",
      "root": "/abs/path/to/workspace",
      "profiles": { "env/infisical": "work" },
      "projects": {
        "web": { "profiles": { "deploy/aws-s3": "web-prod" } }
      }
    }
  },
  "env/infisical":      { "default": "work",     "profiles": { "work": {...} } },
  "deploy/aliyun-oss":  { "default": "web-prod", "profiles": { "web-prod": {...} } },
  "deploy/aws-s3":      { "default": "us-east",  "profiles": { "us-east": {...} } },
  "deploy/kustomize":   { "default": "prod-k8s", "profiles": { "prod-k8s": {...} } },
  "container/ghcr":     { "default": "prod",     "profiles": { "prod": {...} } }
}
```

`credentials.json` 镜像同样的 (domain, backend, name) 结构,但只放
secret 字段。Sections 没有 secret(deploy/kustomize)
不出现在 credentials.json 中。

Empty sections are dropped from the file. This schema is a hard break:
older configs (including the legacy `active` pointer) are rejected with
PROFILE_VERSION_UNSUPPORTED. Profile names can recur across backends
within the same domain (e.g. `prod` under both `deploy/aliyun-oss` and
`deploy/kustomize`) — the per-(domain, backend) command path keeps them
distinct without disambiguation flags.

## `one skills`

Top-level command for managing bundled skills on the local
machine.

The bundled payload currently includes `one-cli` for create/add/dependency
bootstrap/reference workflows and `one-migrate` for adopting existing
projects into a One CLI workspace. These skills target coding agents such
as Codex, Claude Code, Cursor, Gemini CLI, and GitHub Copilot. Dependency
bootstrap is intentionally agent-side: Node projects install with the
detected package manager, while Go projects use Go module commands.

| Verb | Purpose | Schema |
|---|---|---|
| `one skills install` | install / refresh bundled skills on detected agents | `one-cli/skills-install/v1` |

Install flags:

- `one skills install`: interactive multi-select for detected agents.
- `one skills install --yes`: install to all detected agents.
- `one skills install --agent <id>`: repeatable, install only to those agents.

## Error code matrix

| Code | Where | Recovery |
|---|---|---|
| `NOT_ONE_PROJECT` | workspace commands | cd into a workspace or pass `-d` |
| `PROJECT_NAME_REQUIRED` | `create` non-interactive | pass `[dir]` |
| `SUBPROJECT_NAME_REQUIRED` | `add` non-interactive | pass `--name <name>` |
| `TEMPLATE_REQUIRED` | `add` non-interactive | pass template id |
| `TEMPLATE_NOT_FOUND` | `add` | read `error.context.available_templates` |
| `TARGET_EXISTS` | `add` | rename or delete existing target |
| `EXISTING_TARGET_NOT_EMPTY` | `create` | choose an empty target directory |
| `WORKSPACE_NESTED_FORBIDDEN` | `create` | create outside an existing workspace, or use `one add` |
| `INVALID_NAME` | `create` / `add` | match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$` |
| `PLUGIN_NOT_ENABLED` | env/dev/container/deploy verbs | configure the relevant section in `one.manifest.json` (`domains.env.kind`, `projects[].domains.container`, etc.) or pick a template that declares it |
| `PLUGIN_VERB_NOT_SUPPORTED` | env/deploy verbs | the active backend does not implement that verb (e.g. `one env set` on `dotenv`) |
| `PROFILE_NONE_CONFIGURED` | env/deploy remote ops | add/use a matching profile |
| `PROFILE_PLUGIN_INVALID` | profile/deploy | use a profile whose backend matches the target backend |
| `STATUS_FIX_FAILED` | `create` / `add` post-write sync | retry the failing command; if persistent, inspect `error.context` |
| `CI_DISABLE_CONFIRMATION_REQUIRED` | `ci disable` non-interactive | review the selected projects, then rerun with `--yes` |
| `CI_NOT_ENABLED` | `ci sync` | run `one ci enable <project>` first |
| `CI_PROVIDER_UNKNOWN` | `ci enable` | use an ID from `error.context.available_providers` |
| `SKILLS_INSTALL_FAILED` | `create` / `skills install` | rerun `one skills install`, fix agent directory permissions |
| `UNKNOWN_COMMAND` | root | use the current command catalog above |
| `INFISICAL_AUTH_MISSING` | env/remote run | configure env profile or credentials |
| `ENV_FILE_NOT_FOUND` | `one run --env-provider dotenv` | create project `.env` or use `--env-provider infisical` |

Always prefer `error.context` and `error.remediation[]` over hard-coded
recovery text.
