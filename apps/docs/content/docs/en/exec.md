---
title: one exec
description: Inject project env vars into any command and execute it from the resolved project directory.
---

`one exec` resolves a project, injects its Infisical variables when configured, and runs your command from the project directory.

## Usage

```bash
one exec [-p <name|path>] [--env <env>] -- <cmd> [args...]
```

You can omit `--`, but scripts should keep it so child flags are not parsed as One CLI flags.

## Options

| option | purpose |
|---|---|
| `-p`, `--project <name|path>` | select a project; without it, One CLI infers from cwd |
| `--env <env>` | use a specific environment |
| `-o`, `--output <fmt>` | affects only One CLI output; child stdout/stderr pass through |

## Interactive Mode

`one exec` has no wizard. It only resolves the project, environment, and child command from arguments. Keep the `--` separator in scripts because the child command can have its own flags.

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

This lets pnpm / turbo workspaces invoke `vite`, `next`, `astro`, and similar binaries directly.

## Infisical injection

The top-level manifest `env` binding enables Infisical injection. Without a binding, or with `projects[].env.disabled: true`, the command inherits the shell environment. One CLI does not load `.env` files. Authentication or fetch failures stop execution instead of falling back to local files. Use `one login` to authenticate.

## Common errors

| code | fix |
|---|---|
| `NOT_ONE_PROJECT` | run inside a workspace or project directory |
| `SUBPROJECT_NOT_FOUND` | pass a manifest `name` or `relativeDir` to `-p` |
| `RUN_COMMAND_NOT_FOUND` | check PATH, project `node_modules/.bin`, and workspace `node_modules/.bin` |
| `INFISICAL_AUTH_MISSING` | run `one login` |

## Next

- [Run with env vars](/en/tutorials/run-passthrough/)
- [one env](/en/docs/env-vars/)
- [one dev](/en/docs/dev/)


## Global credentials

```bash
one exec --global --env dev --path /oss --keys OSS_ACCESS_KEY_ID,OSS_ACCESS_KEY_SECRET -- upload-assets
one exec --global --env dev --path /oss --dry-run -- upload-assets
```

Environment and folder must be explicit. Only that folder is read; `--keys` fetches only selected variables. Dry-run does not read credentials. Global mode does not load project environments or implicitly resolve repository binaries. It preserves the current directory. Exact-value output masking is best effort, not a sandbox.
