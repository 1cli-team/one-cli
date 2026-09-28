---
title: one dev
description: Run development tasks and their prerequisites through mise.
---

`one dev` is equivalent to `one run dev`. Both execute the effective mise task graph, including your overrides in `mise.toml`.

```sh
one dev
one dev -p web -p api
one dev -p apps/web --dry-run -o json
one dev -p web --ui raw
one dev -p web -- --port 4300
```

Without `-p`, One invokes the root `dev` task. The generated aggregate includes projects with a dev task. Repeat `-p` to select projects by name or relative path. Pass task arguments after `--`, selecting one project when forwarding development arguments.

## Commands and overrides

Generated adapters use `projects[].dev.command` when set, otherwise the project's native `dev` script or Taskfile task. Project `mise.toml` can override the task; root `mise.toml` can replace the workspace aggregate. Custom mise commands use mise's environment. Generated adapters receive One's frozen project environment.

## Dependency preparation

`one dev` prepares the selected projects before starting any development commands. Interactive and non-interactive calls use the same preparation flow.

- Node dependencies are prepared at the workspace root. With pnpm 10.14 or later, One asks pnpm to verify the installed workspace state and reuses a matching installation, including one created by a manual `pnpm install`. When installation is needed, it runs `pnpm install --no-frozen-lockfile` so new projects and dependency changes can update the lockfile. Older pnpm versions use One's installation cache. Other package managers keep their existing lockfile policy.
- `one build` keeps the strict installation policy: an existing pnpm dependency lockfile uses `--frozen-lockfile`. If it is stale, install dependencies and review the lockfile changes before building.
- Go preparation downloads the fixed module build list or resolves workspace dependencies, maintaining checksums as needed. It does not run `go mod tidy` or `go work sync` automatically.
- A failed preparation stops startup. Package-manager diagnostics are streamed once; canceling stops the preparation process. `one exec` does not install dependencies.


## Logs, input, and exit

Development allocates enough concurrency for the selected graph by default. Multiple services use prefixed mise logs; `--concurrency` must leave enough slots for all development services. Upstream finite builds finish before their dependent service starts.

`--ui raw` preserves native terminal input for one service and disables artifact caching. mise raw/interactive tasks take exclusive terminal access; run interactive services separately. Multiple development services cannot use raw output. The previous TUI, project picker, `--keep-going`, and single-service restart controls have been removed.

Ctrl+C or SIGTERM stops the invocation and its child processes. If one task fails, the invocation stops and returns the child exit code. JSON/YAML output uses `one-cli/task-result/v1`, with child logs on stderr. `--dry-run` uses static `one-cli/task-plan/v1` output without installing tools, loading secrets, or writing configuration.

See [one run](/en/docs/run/) for common flags, task configuration, caching, and command-name conflicts.
