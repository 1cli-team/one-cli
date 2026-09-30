---
title: one env
description: Manage Infisical variables with set, unset, and list; inject them directly into commands.
---

Infisical is One CLI's only managed environment source. Variables are fetched for command execution and injected into the child process. One CLI does not read, write, or export `.env` files.

## Setup

New workspaces have no Infisical binding and can run without signing in. Run `one login`, then save the first variable with `one env set` to initialize storage. Dashboard also initializes only when the first variable is saved; opening, refreshing, and cancelling never create a project.

The binding lives in the `[env.infisical]` table of `one.manifest.toml`:

```toml
[env.infisical]
siteUrl = "https://app.infisical.com"
projectId = "your-project-id"
environments = ["dev", "staging", "prod"]
```

The manifest records only binding information and environment slugs. Values stay in Infisical; authentication uses the browser session stored in the system keyring.

## Commands

```bash
one env
one env set DATABASE_URL -p api
one env set API_URL=https://example.com -p web --env dev --yes
one env set SIGNING_PRIVATE_KEY -p api --env dev --stdin < /secure/private-key.pem
one env list -p web --env dev
```

Use `--stdin` for multiline PEM files and other secrets without passing their contents as arguments. It works for both project variables and `--global` shared credentials, preserves embedded line breaks, and removes only one final line terminator. Input is limited to 1 MiB; combining `--stdin` with an argument value is rejected. Existing-variable overwrites still require confirmation or `--yes`.

`set KEY` prompts for a hidden value in a terminal. Scripts pass a value explicitly. `--yes` confirms overwrites and new environment names. `list` returns names only. The CLI has no plaintext read command; use `one exec` to inject values into a child process. `-o json` or `-o yaml` provides structured output with the stable schemas `one-cli/env-set/v1` and `one-cli/env-list/v1`.

`-p / --project` accepts a project name or workspace-relative path. Without it, One CLI infers the project from the current directory. At the workspace root, variables are shared; interactive `set` offers a scope selection.

## Environments and folders

`env.infisical.environments` declares remote environment slugs. The default is always `dev`; `--env` selects another declared slug. A new binding defaults to `dev`, `staging`, and `prod`. These identifiers match Infisical's environment slugs, not its display names. Declaring a name does not create an environment remotely.

After a successful remote write, `set` can register a new environment locally with confirmation. The environment must already exist in Infisical. `list` rejects undeclared names with `ENV_UNKNOWN_ENVIRONMENT`. Listing, reading, and `unset` require an existing binding; unbound workspaces show guidance to save the first variable.

The shared folder is `/`. Project folders derive from their `path`: `services/api` maps to `/services/api`. Variables merge from `/` through `/services` to `/services/api`, with closer folders winning. Parallel tasks receive independent project environments. There are no project-level path, inheritance, key-list, or disable settings.

## Run with variables

```bash
one exec -p api --env dev -- go run ./cmd/server
one run dev --env dev
```

With an `[env.infisical]` binding, `one run` and `one exec` fetch variables for the selected project and override matching shell variables. Without a binding, they inherit the shell environment. Authentication or fetch failures stop execution. One does not print fetched values; child stdout, stderr, ANSI colors, and formatting pass through unchanged.

## Shared credentials

Shared credentials are independent of workspaces. Select storage with `one env bind --global`, browse names with `one env list --global --env dev --path /`, and inject an explicit scope with `one exec --global --env dev --path /folder -- command`. See [login and shared credentials](/en/docs/login/).

## Common errors

| Code | Recovery |
|---|---|
| `INFISICAL_NOT_CONFIGURED` | Sign in and save the first variable to create and connect a storage project |
| `INFISICAL_AUTH_MISSING` / `INFISICAL_AUTH_FAILED` | Run `one login` and check access to the bound project |
| `INFISICAL_PROJECT_NAME_TAKEN` | Select an existing project in Dashboard, or use a different workspace name |
| `INFISICAL_PROJECT_CREATE_FORBIDDEN` | Select an existing accessible project in Dashboard |
| `ENV_KEY_NOT_FOUND` | Check key spelling, environment, and folder |
| `ENV_INVALID_KEY` | Use names matching `^[A-Za-z_][A-Za-z0-9_]*$` |
| `ENV_SET_OVERWRITE_REQUIRED` | Confirm the replacement with `--yes` |
| `ENV_UNKNOWN_ENVIRONMENT` | Register the environment with `set`, or select an existing name |

See [Manifest v2](/en/docs/manifest/) for the configuration structure and [the walkthrough](/en/tutorials/env-vars/) for a first setup.

## Workspaces with the same name

Remote projects default to the workspace name. On a name conflict, One adds a short suffix and displays the actual name. Identically named workspaces do not share variables merely because of their names: writes target the stored `env.infisical.projectId`. Copying or cloning a configuration that already contains a binding reuses that remote project. If saving a variable fails after creation, the binding is retained and retries reuse it.
