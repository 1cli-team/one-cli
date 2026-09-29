---
title: one exec
description: Inject project env vars into any command and execute it from the resolved project directory.
---

`one exec` resolves a project, injects its Infisical variables when configured, and runs your command from the project directory.

One does not print fetched variable values. Child stdout, stderr, colors, formatting, and exit codes pass through unchanged.

## Usage

```bash
one exec [-p <name|path>] [--env <env>] -- <cmd> [args...]
```

The `--` separator is required. You can also select the project with a positional argument, such as `one exec web -- pnpm build`.

## Options

| option | purpose |
|---|---|
| `-p`, `--project <name|path>` | select a project; without it, One CLI infers from cwd |
| `--env <env>` | select an environment; project mode defaults to `dev` |
| `--dry-run` | show the directory, arguments, and runtime without preparing tools, fetching secrets, or running the command |
| `-o`, `--output <fmt>` | affects only One CLI output; child stdout/stderr pass through |

## Interactive Mode

`one exec` has no wizard. It only resolves the project, environment, and child command from arguments. Use the required `--` separator before the child command and its flags.

## mise tool environment

With a One-generated root `mise.toml`, `one exec` prepares the project tool environment through mise. One uses a compatible system mise or downloads its managed version when needed. It does not install project dependencies; use `one run` for tasks that need dependency preparation.

The environment order is shell → mise → One project variables, with later values taking precedence. Project directory, arguments, standard IO, and the child exit code are preserved. No shell activation is needed.

See [runtime installation](/en/docs/installation/) and [workspace tools](/en/docs/login/) for setup and offline use.

## Examples

```bash
one exec -- pnpm test
one exec -p web -- pnpm run build
one exec -p apps/web -- pnpm lint
one exec --env staging -- pnpm run e2e
```

The child process always runs from the resolved project directory, so commands find that project's `package.json`, `Taskfile.yml`, or Go module files.

## PATH and env

`one exec` merges loaded variables into the child environment, overriding same-name shell variables. It also prepends:

```text
<project>/node_modules/.bin
<workspace>/node_modules/.bin
```

This lets Node workspaces invoke `vite`, `next`, `astro`, and similar binaries directly.

## Infisical injection

The manifest `[env.infisical]` binding enables Infisical injection. Without a binding, the command inherits the shell environment. One CLI does not load `.env` files. Authentication or fetch failures stop execution instead of falling back to local files. Use `one login` to authenticate.

## Common errors

| code | fix |
|---|---|
| `NOT_ONE_PROJECT` | run inside a workspace or project directory |
| `SUBPROJECT_NOT_FOUND` | pass a manifest project name or `path` to `-p` |
| `RUN_COMMAND_NOT_FOUND` | check PATH, project `node_modules/.bin`, and workspace `node_modules/.bin` |
| `INFISICAL_AUTH_MISSING` | run `one login` |

## Next

- [Run with env vars](/en/tutorials/env-vars/)
- [one env](/en/docs/env-vars/)
- [one dev](/en/docs/dev/)


## Global credentials

```bash
one exec --global --env dev --path /oss --keys OSS_ACCESS_KEY_ID,OSS_ACCESS_KEY_SECRET -- upload-assets
one exec --global --env dev --path /oss --dry-run -- upload-assets
```

Environment and folder must be explicit. Only that folder is read; `--keys` fetches only selected variables. Dry-run does not read credentials. Global mode does not load project environments or implicitly resolve repository binaries. It preserves the current directory. One does not print injected values; child output passes through unchanged, including any variable values the child prints.
