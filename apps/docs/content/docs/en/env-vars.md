---
title: one env
description: Manage Infisical variables with set, unset, and list; inject them directly into commands.
---

Infisical is One CLI's only managed environment source. Variables are fetched for command execution and injected into the child process. One CLI does not read, write, or export `.env` files.

## Setup

New workspaces have no Infisical binding and can run without signing in. Run `one login`, then use `one env set` or `one env list` to initialize the binding on first use. You can also select an existing Infisical project in Dashboard workspace settings.

The binding lives in the top-level `env` field of `one.manifest.json`:

```json
{
  "env": {
    "siteUrl": "https://app.infisical.com",
    "projectId": "your-project-id",
    "projectName": "my-workspace",
    "rootPath": "/"
  },
  "environments": {
    "names": ["dev", "preview", "prod"],
    "default": "dev"
  }
}
```

The manifest records binding information, folder paths, and declared key names. Values stay in Infisical; authentication uses the browser session stored in the system keyring.

## Commands

```bash
one env
one env set DATABASE_URL -p api
one env set API_URL=https://example.com -p web --env dev --yes
one env list -p web --env dev
```

`set KEY` prompts for a hidden value in a terminal. Scripts pass a value explicitly. `--yes` confirms overwrites and new environment names. `list` returns names only. The CLI has no plaintext read command; use `one exec` to inject values into a child process. `-o json` or `-o yaml` provides structured output with the stable schemas `one-cli/env-set/v1` and `one-cli/env-list/v1`.

`-p / --project` accepts a project name or workspace-relative path. Without it, One CLI infers the project from the current directory. At the workspace root, variables are shared; interactive `set` offers a scope selection.

## Environments and folders

`environments.names` declares the available environments. The default is `environments.default`, falling back to the first name. New workspaces declare `dev`, `preview`, and `prod` with `dev` as default. `--env` overrides this choice.

`set` can register a new environment with confirmation. `list` rejects names not declared in the manifest with `ENV_UNKNOWN_ENVIRONMENT`. These commands can still initialize an absent Infisical binding.

Projects use their `relativeDir` as the default Infisical folder. Override this in `projects[].env`:

```json
{
  "name": "api",
  "relativeDir": "services/api",
  "env": {
    "path": "/teams/payments/api",
    "inherits": true,
    "keys": ["DATABASE_URL"]
  }
}
```

With inheritance enabled (the default), variables merge from root through ancestor folders to the project folder; closer folders win. Set `inherits` to `false` to read only the project folder. `disabled: true` disables that project's Infisical injection.

## Run with variables

```bash
one exec -p api --env dev -- go run ./cmd/server
one run dev --env dev
```

With an `env` binding, generated task adapters and `one exec` fetch Infisical variables and override matching shell variables. Without a binding, or with project injection disabled, they inherit the shell environment. Authentication or fetch failures stop execution; there is no local-file fallback. Custom mise tasks use mise's environment. One masks known injected values in stdout, stderr, and task-cache replay, including multiline values and their JSON-escaped forms.

## Shared credentials

Shared credentials are independent of workspaces. Select storage with `one env bind --global`, browse names with `one env list --global --env dev --path /`, and inject an explicit scope with `one exec --global --env dev --path /folder -- command`. See [login and shared credentials](/en/docs/login/).

## Common errors

| Code | Recovery |
|---|---|
| `INFISICAL_NOT_CONFIGURED` | Bind an Infisical project in Dashboard or sign in and initialize it through an env command |
| `INFISICAL_AUTH_MISSING` / `INFISICAL_AUTH_FAILED` | Run `one login` and check access to the bound project |
| `INFISICAL_PROJECT_NAME_TAKEN` | Set a distinct `env.projectName`, or bind the existing project explicitly |
| `INFISICAL_PROJECT_CREATE_FORBIDDEN` | Select an existing accessible project in Dashboard |
| `ENV_KEY_NOT_FOUND` | Check key spelling, environment, and folder |
| `ENV_INVALID_KEY` | Use names matching `^[A-Za-z_][A-Za-z0-9_]*$` |
| `ENV_SET_OVERWRITE_REQUIRED` | Confirm the replacement with `--yes` |
| `ENV_UNKNOWN_ENVIRONMENT` | Register the environment with `set`, or select an existing name |

See [manifest migration](/en/docs/manifest/) for older workspaces and [the walkthrough](/en/tutorials/env-vars/) for a first setup.
