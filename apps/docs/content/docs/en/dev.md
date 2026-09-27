---
title: one dev
description: Start every developable project, or one selected project.
---

`one dev` reads each project's development command from the manifest and runs it through One CLI's built-in supervisor.

## Usage

```bash
one dev [project] [--dry-run]
```

## Options

| option | purpose |
|---|---|
| positional `project` | start one project by manifest `name` or `relativeDir` |
| `-p`, `--project <name|path>` | legacy selector for scripts and CI |
| `--dry-run` | print the supervisor command without starting processes |
| `-o`, `--output <fmt>` | `json` / `yaml` / `text` |

## Dependency preparation

`one dev` prepares the selected projects before starting any development commands. Interactive and non-interactive calls use the same preparation flow.

- Node dependencies are prepared at the workspace root. With pnpm 10.14 or later, One asks pnpm to verify the installed workspace state and reuses a matching installation, including one created by a manual `pnpm install`. When installation is needed, it runs `pnpm install --no-frozen-lockfile` so new projects and dependency changes can update the lockfile. Older pnpm versions use One's installation cache. Other package managers keep their existing lockfile policy.
- `one build` keeps the strict installation policy: an existing pnpm dependency lockfile uses `--frozen-lockfile`. If it is stale, install dependencies and review the lockfile changes before building.
- Go preparation downloads the fixed module build list or resolves workspace dependencies, maintaining checksums as needed. It does not run `go mod tidy` or `go work sync` automatically.
- A failed preparation stops startup. Package-manager diagnostics are streamed once; canceling stops the preparation process. `one run` does not install dependencies.

## Runner

One CLI's built-in supervisor starts all developable projects by default, or one positional project.

```bash
one dev
one dev web
one dev apps/web --dry-run
```

## Common errors

| code | fix |
|---|---|
| `RUN_COMMAND_NOT_FOUND` | check the native tools or mise configuration |
| `ONE_CLI_ERROR` | fix the reported dependency error, then retry |
| `SUBPROJECT_NOT_FOUND` | use a project `name` or `relativeDir` |

## Next

- [Local dev orchestration](/en/tutorials/dev-local/)
- [one run](/en/docs/run/)
- [Workspace manifest](/en/docs/manifest/)
