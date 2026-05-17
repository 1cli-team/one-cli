---
title: one run
description: Inject project env vars into any command and execute it from the resolved project directory.
---

`one run` behaves like `infisical run` / `dotenv run`: it resolves a project, loads env vars from the workspace env provider, injects them into a child process, and runs the command you pass.

## Usage

```bash
one run [-p <name|path>] [--env-provider dotenv|infisical] [--env <env>] -- <cmd> [args...]
```

You can omit `--`, but scripts should keep it so child flags are not parsed as One CLI flags.

## Options

| option | purpose |
|---|---|
| `-p`, `--project <name|path>` | select a project; without it, One CLI infers from cwd |
| `--env-provider dotenv|infisical` | force a provider instead of using the workspace manifest |
| `--env <env>` | use a specific environment |
| `-o`, `--output <fmt>` | affects only One CLI output; child stdout/stderr pass through |

## Interactive Mode

`one run` has no wizard. It only resolves the project, environment, and child command from arguments. Keep the `--` separator in scripts because the child command can have its own flags.

## Examples

```bash
one run -- npm test
one run -p web -- npm run build
one run -p apps/web -- pnpm lint
one run --env-provider dotenv -- npm test
one run --env staging -- npm run e2e
```

The child process always runs from the resolved project directory, so commands find that project's `package.json`, `Taskfile.yml`, or Go module files.

## PATH and env

`one run` merges loaded variables into the child environment, overriding same-name shell variables. It also prepends:

```text
<project>/node_modules/.bin
<workspace>/node_modules/.bin
```

This lets pnpm / turbo workspaces invoke `vite`, `next`, `astro`, and similar binaries directly.

## Env provider

| provider | behavior |
|---|---|
| `dotenv` | read project `.env` overlays |
| `infisical` | fetch env vars from Infisical |
| empty | use the provider recorded in the workspace manifest |

`--env-provider infisical` requires an `env/infisical` profile. Use `--env-provider dotenv` for offline local runs.

## Common errors

| code | fix |
|---|---|
| `NOT_ONE_PROJECT` | run inside a workspace or project directory |
| `SUBPROJECT_NOT_FOUND` | pass a manifest `name` or `relativeDir` to `-p` |
| `RUN_COMMAND_NOT_FOUND` | check PATH, project `node_modules/.bin`, and workspace `node_modules/.bin` |
| `ENV_FILE_NOT_FOUND` | create a project `.env` or use `--env-provider infisical` |
| `INFISICAL_AUTH_MISSING` | run `one configure add env/infisical --profile <name> --use` |

## Next

- [Run with env vars](/en/tutorials/run-passthrough/)
- [one env](/en/docs/env-vars/)
- [one dev](/en/docs/dev/)
