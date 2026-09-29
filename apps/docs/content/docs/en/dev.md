---
title: dev task
description: Run existing development tasks through mise.
---

`dev` is an ordinary mise task. Use `one run dev`; `one dev` is only the generic task shorthand, with no separate built-in implementation.

```sh
one run dev
one run dev -p web -p api
one run dev -p apps/web --dry-run -o json
one run dev --ui tui
one run dev -p web -- --port 4300
```

Without `-p`, the root dev task must exist. With `-p`, the selected project's dev task must exist. Missing tasks fail immediately; One does not infer commands from manifest settings, start scripts, or Go source directories. `one init mise` explicitly projects existing package scripts and Taskfile tasks into native mise commands. Edit mise tasks directly for custom commands.

## Preparation and environment

Named tasks share one preparation policy. Node dependencies are prepared at the workspace root. With pnpm 10.14 or later, One verifies installed dependencies and reuses matching installations, including manual installs. When needed, it runs `pnpm install --no-frozen-lockfile`. Go preparation resolves the fixed module build list without automatically running `go mod tidy` or `go work sync`. Preparation failure prevents task startup; `one exec` does not install dependencies.

One assigns a task to the registered project containing its effective working directory. Enabled projects receive their own Infisical snapshot for the invocation. Parallel projects do not share variable maps, and child processes inherit the correct project environment. No env plugin declaration or secret value is required in your mise file.

## Terminal and exit

Automatic concurrency provides one slot per graph node; an explicit `--concurrency` is honored. With a low limit, long-running tasks may occupy slots needed by other services. Finite prerequisite tasks finish before dependent services start.

Use `--ui stream` for prefixed logs or `--ui tui` for task selection, search, scrolling, and follow mode. Interactive/raw tasks require exclusive terminal access. `--ui raw` delegates terminal handling to mise and disables artifact caching. TUI and stream have identical scheduling and exit behavior.

Ctrl+C or SIGTERM stops the invocation and its child processes. Child logs are unchanged; One itself does not print injected variable values. JSON/YAML output sends child logs to stderr. Dry-run reads static configuration without loading secrets or writing files.

See [one run](/en/docs/run/) for configuration, cache behavior, and terminal preferences.
