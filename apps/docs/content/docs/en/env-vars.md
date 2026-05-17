---
title: one env
description: "Full reference for multi-environment variables: set / get / list / pull."
---

`one env` manages monorepo environment variables across environments. There are two backends:

- **dotenv** (default): local filesystem. Each project has overlays such as `.env`, `.env.<env>`, `.env.local`, and `.env.<env>.local`.
- **infisical**: one workspace shares one Infisical project; environments such as dev / staging / prod are sections inside that project.

For the full workflow and mental model, read [Environment variables guide](/en/tutorials/env-vars/).

## Environment Model

`manifest.environments.names` is the workspace environment list. `one create` defaults to `["dev","staging","prod"]`. `manifest.environments.default` is the fallback when `--env` is omitted; default is `dev`.

`--env` resolution:

```text
--env flag -> manifest.environments.default -> environments.names[0]
```

`one env set FOO bar --env qa` can create a new environment. TTY mode asks for confirmation; non-TTY / `--yes` appends directly to `manifest.environments.names`. `get`, `list`, and `pull` are read-only: unknown env names return `ENV_UNKNOWN_ENVIRONMENT`.

## Usage

```bash
one env set  <KEY[=VALUE]> [VALUE] [--env <env>] [-p <name|path>] [--yes]
one env get  <KEY>                 [--env <env>] [-p <name|path>]
one env list                       [--env <env>] [-p <name|path>]
one env pull                       [--env <env>] [-p <name|path>] [--force] [--dry-run]
```

`-p / --project` accepts a manifest project name or workspace-relative path:

```bash
one env set FOO=bar -p web
one env set FOO=bar -p apps/web
one env set FOO=bar              # auto-detects project from cwd
one env pull -p api              # only pull api
one env pull --env staging       # pull staging vars for all projects
```

The global output flag is `-o / --output`, with `json`, `yaml`, or `text`.

> There is no `one env init` subcommand today. Infisical project binding is attempted by `one create --env-provider infisical`. If profile, network, or permissions were not ready during create, the first `set/get/list/pull` retries lazy auto-bind.

Machine-level Infisical credentials are configured with [`one configure add env/infisical`](/en/docs/cli-overview/#machine-profiles). They do not go into the manifest.

## Interactive Mode

`one env` does not have a full wizard, but `one env set` can ask for confirmation in TTY mode:

- When writing to an environment that is not in `manifest.environments.names`, it asks whether to add that environment to the manifest.
- When overwriting an existing different value, it asks whether to overwrite.

Scripts, CI, and agents should pass `--yes` to skip confirmations. `get`, `list`, and `pull` are explicit-argument commands and do not open a wizard.

## dotenv Overlay

The dotenv backend reads files in this order:

```text
<project>/.env
<project>/.env.<env>
<project>/.env.local
<project>/.env.<env>.local
```

Later files override earlier files. `one env set` writes `.env.<env>`; `.local` files are read-only from One CLI's perspective and are maintained by developers.

`one create` writes this to `.gitignore` by default:

```text
.env
.env.*
!.env.example
```

## set

Write one key. Both argument forms work:

```bash
one env set DATABASE_URL "postgres://localhost/dev" --env dev -p api
one env set JWT_SECRET=dev-only-secret --env dev -p api --yes
```

Output schema: `one-cli/env-set/v1`

```json
{
  "schema": "one-cli/env-set/v1",
  "env": "dev",
  "key": "DATABASE_URL",
  "action": "created"
}
```

`action` can be `created`, `updated`, or `unchanged`. Existing different values require `--yes` to confirm overwrite.

## get

Read one key:

```bash
one env get DATABASE_URL --env dev -p api
DB_URL=$(one env get DATABASE_URL --env dev -p api -o json | jq -r .value)
```

Output schema: `one-cli/env-get/v1`

```json
{
  "schema": "one-cli/env-get/v1",
  "env": "dev",
  "key": "DATABASE_URL",
  "value": "postgres://..."
}
```

## list

List key names without values:

```bash
one env list --env dev -p api
```

Output schema: `one-cli/env-list/v1`

```json
{
  "schema": "one-cli/env-list/v1",
  "env": "dev",
  "keys": ["DATABASE_URL", "JWT_SECRET"]
}
```

## pull

Pull Infisical variables into local `.env`:

```bash
one env pull --env dev
one env pull --env dev -p api --dry-run
one env pull --env dev --force
```

Without `-p`, One CLI iterates through `manifest.projects[]`. Each project maps to an Infisical folder from `projects[].domains.env.path` or `relativeDir`, merges root -> ancestors -> self inheritance, and writes the resulting `.env` into that project directory.

Output schema: `one-cli/env-pull/v1`

```json
{
  "schema": "one-cli/env-pull/v1",
  "env": "dev",
  "dry_run": false,
  "written_count": 1,
  "skipped_count": 0,
  "per_subproject": [
    {
      "name": "api",
      "relative_dir": "services/api",
      "infisical_path": "/services/api",
      "env_file_path": "/abs/.../services/api/.env",
      "status": "written",
      "keys_written": ["DATABASE_URL", "JWT_SECRET"]
    }
  ]
}
```

## Manifest Configuration

Workspace env backend lives in `one.manifest.json#domains.env`; environments live at top-level `environments`:

```json
{
  "environments": {
    "names": ["dev", "staging", "prod"],
    "default": "dev"
  },
  "domains": {
    "env": {
      "kind": "infisical",
      "profile": "work",
      "config": {
        "projectId": "...",
        "projectName": "my-workspace",
        "rootPath": "/"
      }
    }
  }
}
```

Project path overrides live in `projects[].domains.env`:

```json
{
  "projects": [
    {
      "name": "charge",
      "relativeDir": "services/charge",
      "domains": {
        "env": {
          "path": "/teams/payments/charge",
          "inherits": true
        }
      }
    }
  ]
}
```

Values never go into the manifest. The manifest records backend, profile, folder path, and key names.

## Credential Safety

`one configure add env/infisical` writes `~/.config/one/config.json` and `~/.config/one/credentials.json` with mode `0600`. Do not put client id or client secret in the repo; inject them through your CI secret store.

## Common Errors

| Code | Recovery |
|---|---|
| `INFISICAL_NOT_CONFIGURED` | Confirm the workspace uses `--env-provider infisical` and has a default `env/infisical` profile |
| `INFISICAL_AUTH_MISSING` | Re-run `one configure add env/infisical --profile work ... --use` |
| `INFISICAL_AUTH_FAILED` | Regenerate the client secret in Infisical |
| `INFISICAL_PROJECT_NAME_TAKEN` | Change `domains.env.config.projectName` and rerun an env command to trigger lazy bind |
| `INFISICAL_PROJECT_CREATE_FORBIDDEN` | Grant admin role to the machine identity, or manually create the project and fill `domains.env.config.projectId` |
| `ENV_PULL_CONFLICT` | Local `.env` exists and differs; add `--force` after confirming overwrite |
| `ENV_KEY_NOT_FOUND` | Check path, env name, and spelling |
| `ENV_INVALID_KEY` | Keys must match `^[A-Za-z_][A-Za-z0-9_]*$` |
| `ENV_SET_OVERWRITE_REQUIRED` | Existing value differs; add `--yes` to confirm |
| `ENV_UNKNOWN_ENVIRONMENT` | `set` can create; read commands require the env to exist in `manifest.environments.names` |

Full table: [Error codes](/en/docs/error-codes/).

## Next

- [Environment variables guide](/en/tutorials/env-vars/) — mental model and complete workflow
- [`one create`](/en/docs/create/) — use `--env-provider infisical` during workspace creation
