---
title: one create
description: Create a new One workspace root skeleton.
---

`one create` creates an empty workspace. It does not ask for projects or deployment, install Coding Agent Skills, or modify local AI-tool settings. Add projects later with `one add`.

## Usage

```bash
one create [dir] [options]
```

## Arguments

| Argument | Description |
|---|---|
| `dir` | Target directory. Use `.` to create in the current directory with `basename(cwd)` as the name. The target must not exist or must be empty |
| `-n, --name <name>` | Workspace name. Defaults to `basename(dir)` |
| `-y, --yes` | Non-interactive mode; uses defaults and requires an explicit `dir` |
| `--env-provider <dotenv\|infisical>` | Env backend selection. Defaults to `dotenv`; pass `infisical` explicitly when needed |
| `-o, --output <fmt>` | `json` / `yaml` / `text`; default is TTY-aware auto detection |

## Interactive Mode

Running `one create` with no arguments opens terminal questions for:

1. Target directory, such as `./my-app`; use `.` for the current directory.
2. Workspace name, optional; when empty, One CLI uses the target directory basename.

`one create` does not ask about deploy / container, and it no longer asks whether to switch to Infisical. The default env backend is `env/dotenv`; use `--env-provider infisical` to choose Infisical at create time.

For scripts, CI, and agents, use non-interactive commands:

```bash
one create my-app --yes
one create my-app --yes --env-provider infisical
```

## Default Workspace Capabilities

`one create` no longer asks users to manually select many plugins.

**Workspace defaults, enabled without terminal questions**

| Capability | Default | Behavior |
|---|---|---|
| Environment variables | Local `.env` files | Switch to Infisical with `--env-provider infisical` or later with `one env switch infisical` |
| Local development | `one dev` | Runs project development commands through One CLI's built-in supervisor |

Continuous integration is not configured automatically. Creating a workspace
does not write files under `.github/workflows/`. After adding a project, enable
it explicitly with `one ci enable <project>` if needed.

**Deployment is delayed**

Create does not write deployment or container configuration. Ordinary `one add`
also leaves it unset. The first `one deploy <project>` asks for a compatible
deployment target and local connection.

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
  "ci_enabled": false,
  "dev_enabled": true,
  "skills": {
    "status": "skipped",
    "reason": "manual-install"
  }
}
```

`secrets_backend` is the stable environment-source ID (`dotenv` / `infisical`).
`ci_enabled` remains in the response for wire compatibility and is `false` by
default; `dev_enabled` is `true`. Deployment configuration is added later.

`skills.status` is `"skipped"`; run `one skills install` separately if wanted.

## Examples

### Interactive

```bash
one create
# Asks for target directory and optional workspace name
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
| `PROJECT_NAME_REQUIRED` | Pass the workspace directory as the positional argument in non-interactive mode |
| `BACKEND_ID_UNKNOWN` | Invalid `--env-provider`; legal values are `dotenv` / `infisical` |
| `WORKSPACE_NESTED_FORBIDDEN` | Do not create a workspace inside an existing workspace; use another directory or `one add` |

Full table: [Error codes](/en/docs/error-codes/).
