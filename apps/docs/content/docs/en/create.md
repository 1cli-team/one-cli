---
title: one create
description: Create a new One workspace root skeleton.
---

`one create` creates the workspace skeleton and installs skills. By default it creates only the workspace root; add projects later with `one add`.

## Usage

```bash
one create [dir] [options]
```

## Arguments

| Argument | Description |
|---|---|
| `dir` | Target directory. Use `.` to create in the current directory with `basename(cwd)` as the name. The target must not exist or must be empty |
| `-n, --name <name>` | Project name. Defaults to `basename(dir)` |
| `-y, --yes` | Non-interactive mode; uses defaults and requires an explicit `dir` |
| `--env-provider <dotenv\|infisical>` | Env backend selection. Defaults to `dotenv`; pass `infisical` explicitly when needed |
| `-o, --output <fmt>` | `json` / `yaml` / `text`; default is TTY-aware auto detection |

## Interactive Mode

Running `one create` with no arguments opens terminal questions for:

1. Target directory, such as `./my-app`; use `.` for the current directory.
2. Project name, optional; when empty, One CLI uses the target directory basename.

`one create` does not ask about deploy / container, and it no longer asks whether to switch to Infisical. The default env backend is `env/dotenv`; use `--env-provider infisical` to choose Infisical at create time.

For scripts, CI, and agents, use non-interactive commands:

```bash
one create my-app --yes
one create my-app --yes --env-provider infisical
```

## Automatically Enabled Backends

`one create` no longer asks users to manually select many plugins.

**Workspace defaults, enabled without terminal questions**

| Domain | Default backend | Behavior |
|---|---|---|
| `env` | `env/dotenv` | Reads and writes `.env` files; switch to Infisical with `--env-provider infisical` or later with `one env switch infisical` |
| `ci` | `ci/github-actions` | Writes `.github/workflows/` |
| `dev` | `dev/process` | Writes `Procfile.dev`, consumable by mprocs / overmind-style runners |

**Deploy / container are template-driven**

They are not written at create time. `one add <template>` enables them based on template defaults:

| Template | Auto-enabled backend |
|---|---|
| `go-api` / `nestjs-api` / `nextjs-app` | `container=docker` + `deploy=kustomize` |
| `react-spa` / `astro-site` / `starlight-docs` | `deploy=aws-s3` |
| `expo-mobile` / `ts-library` / `go-lib` / `electron-app` | no deploy / container default |

## `--env-provider` Semantics

`--env-provider <dotenv|infisical>` explicitly selects the env backend:

```bash
one create my-app -y --env-provider infisical
```

Configure a machine-level Infisical profile first:

```bash
one configure add env/infisical --profile work \
  --client-id $INFISICAL_UNIVERSAL_AUTH_CLIENT_ID \
  --client-secret $INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET \
  --use
```

`one create --env-provider infisical` tries to auto-bind or create an Infisical project. If the profile, network, or permission is not ready, workspace creation still succeeds; the first `one env set/get/list/pull` retries lazy auto-bind.

## Output

```json
{
  "schema": "one-cli/create/v2",
  "project_name": "my-app",
  "created_path": "/abs/path/my-app",
  "created_in_place": false,
  "package_manager": "pnpm",
  "secrets_backend": "dotenv",
  "ci_enabled": true,
  "dev_enabled": true,
  "skills": {
    "status": "completed",
    "installed_to": ["/Users/example/.claude/skills"],
    "skill_count": 2
  }
}
```

`secrets_backend` is the env-domain backend name (`dotenv` / `infisical`). `ci_enabled` and `dev_enabled` are always `true`; CI workflow and `Procfile.dev` are always synced. Container and deploy backends are template-driven and live in `projects[].domains.{container,deploy}`.

`skills.status` can be:

- `"completed"`: skills installed.
- `"failed"`: workspace creation succeeded, but skill installation failed. Run `one skills install` later.

## Examples

### Interactive

```bash
one create
# Asks for target directory and optional project name
```

### Non-interactive

```bash
one create my-app --yes
```

### Use Infisical As Secrets Backend

```bash
one create my-app --yes --env-provider infisical
```

### Create In Current Directory

```bash
mkdir my-app && cd my-app
one create . --yes
```

### Create Skeleton And Add First Project

```bash
one create my-app --yes
cd my-app
one add nestjs-api --name api --yes
pnpm install
```

## Common Errors

| Code | Recovery |
|---|---|
| `EXISTING_TARGET_NOT_EMPTY` | Choose an empty directory, or delete the target manually and retry |
| `INVALID_NAME` | Names must match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`; replace spaces with `-` |
| `PROJECT_NAME_REQUIRED` | Pass the positional directory/name in non-interactive mode |
| `BACKEND_ID_UNKNOWN` | Invalid `--env-provider`; legal values are `dotenv` / `infisical` |
| `WORKSPACE_NESTED_FORBIDDEN` | Do not create a workspace inside an existing workspace; use another directory or `one add` |
| `SKILLS_INSTALL_FAILED` | Check write permission for agent skill directories, or run `one skills install` manually |

Full table: [Error codes](/en/docs/error-codes/).
