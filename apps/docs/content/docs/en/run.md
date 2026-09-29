---
title: one run
description: Execute native mise tasks with project environments and stream or TUI output.
---

`one run` uses the tasks that actually exist in mise. One selects projects, prepares dependencies and project environments, and displays output. A single mise process schedules the graph and runs the native commands.

```sh
one run
one run --verbose
one run --list -p web -o json
one run dev
one run build -p web -p api
one run test -p api -- -run TestHandler
one run build --dry-run -o json
one run dev --ui tui
one run dev --ui stream
```

Without a task name, the terminal list groups workspace entries and project namespaces and shows task names with descriptions. Root names omit `//:`; use the displayed name directly with `one run <task>`. Use `one run -p web` to filter project-related tasks and `one run --verbose` to include sources, cache and interactive settings. Narrow terminals wrap automatically; `-o json` / `-o yaml` still return complete task data.

## Task names and arguments

`one <task>` remains a shorthand for `one run <task>`. There is no separate built-in dev command. `one run dev` requires a root `dev` task; `one run dev -p web` requires that project's task. Missing tasks produce an error. One never falls back to `start`, `start:dev`, a Go directory, or a manifest command.

Built-in commands take precedence: `one env` manages environments, while `one run env` executes the mise task. Use `-- --help` to forward help to the command. Everything after `--` is forwarded to one selected task; selecting multiple projects with forwarded arguments is rejected. `:::` remains reserved for mise task separation.

`one exec web -- pnpm add axios` executes an arbitrary command with the project's mise tool environment and One variables. It does not select a task graph or install application dependencies.

## Native configuration

Creation, adding a project, and explicit `one init mise` map existing package scripts and Taskfile tasks into the root configuration:

```toml
[tasks."web:dev"]
dir = "apps/web"
run = "pnpm run dev"

[tasks."api:dev"]
dir = "services/api"
run = "task dev --"

[tasks.dev]
depends = ["web:dev", "api:dev"]
```

Execution, task lists, Dashboard queries and dry-run do not generate missing tasks or refresh workspace configuration. After adding a package script, explicitly refresh its mise declaration:

```sh
one init mise --dry-run -o json
one init mise
```

Keep the `# one:managed-v1` marker for incremental generation. User edits and comments are preserved; conflicting changes to the same generated field produce `MISE_CONFIG_CONFLICT`. Existing project mise configurations, aliases, executable `.mise/tasks/` scripts and `[task_config] includes` remain usable.

Lists and normal execution query mise's effective catalogue. Dry-run is static: it does not start mise, install tools, load remote variables or execute cache inputs. Dynamic configurations that cannot be resolved statically require a normal mise inspection.

Generated tasks can run directly through `mise run`; they no longer call private One commands. Direct mise execution uses mise's own environment and does not fetch Infisical through One.

## Project variables

`one run` determines ownership from each task's effective working directory, using the deepest registered project path. A custom root task named `serve-backend` can receive the api environment by declaring `dir = "services/api"`. A root aggregate does not receive a union of its children's variables.

With an Infisical binding, One loads each project's values once per invocation. `--env` selects a declared environment, defaulting to `dev`. Remote folders derive from project paths and include shared and ancestor variables. Each task receives only its project's snapshot; values override matching shell and mise variables. Values remain in memory, including transmission through an authenticated loopback session. No secret values are written to TOML, `.env` or context JSON.

One bundles the mise environment adapter and creates temporary `.one-run-*/bindings.toml` metadata in the relevant config scopes. These files contain task references, never variable values, and are removed on exit. `one init mise` and new workspaces ignore `.one-run-*/` in Git. No plugin rows need to be maintained in user task definitions.

Before launching, One checks that commands, directories, dependencies and bindings remain consistent. A profile that replaces a whole task after the runtime metadata can override its binding; One fails before starting it. Keep the command in the base or local configuration and use profiles for environment settings in that case.

## Cache and concurrency

Ordinary native tasks retain mise's cache declarations. Known deterministic template builds receive sources, outputs and declared environment inputs. No One fingerprint command is required:

```toml
[tasks."web:build"]
dir = "apps/web"
run = "pnpm run build"
sources = ["src/**/*", "package.json", "tsconfig.json", "../../pnpm-lock.yaml"]
outputs = ["dist"]
cache = { enabled = true, env = ["NODE_ENV"] }
```

Tasks with injected remote variables, and executable tasks downstream of them, bypass artifact caching and freshness skipping. One prints a short notice. This ensures changed, removed and empty variables cannot reuse stale outputs. One does not keep a cross-invocation secret cache.

```sh
one run build --cache local-only
one run build --cache off --force
one run build --cache read-only
one run build --concurrency 4
```

Cache modes also include `read-write` and `write-only`; configure remote cache backends directly in mise. Source and output paths are relative to `dir`, and output locations must not overlap across selected tasks. Extra command arguments disable artifact caching and force execution.

Automatic concurrency allocates a slot per graph node, independently of the task name. An explicit limit is respected; long-running prerequisites still follow mise's dependency semantics.

## Terminal preferences

Set the optional `taskUI` preference in `~/.config/one/preferences.json` (`XDG_CONFIG_HOME` is respected):

```json
{
  "version": 1,
  "locale": "auto",
  "taskUI": "tui"
}
```

`taskUI` accepts `stream` or `tui`. `--ui` overrides the preference. With `auto`, multiple tasks in an interactive terminal use TUI; a single task uses stream. CI, non-terminal output, and JSON/YAML use stream. `--ui raw` preserves native terminal input and delegates raw output to mise; interactive tasks require exclusive terminal access.

TUI preserves mise task prefixes, Task command echoes, ANSI colors and text styles, indentation, and blank lines. Long lines wrap to the current pane width, including Chinese text and emoji. Resizing keeps the original log position while you read history. Common carriage-return, backspace, and line-erasing progress output is rendered within the log pane; full-screen interactive applications should use `--ui raw`.

The sidebar shows the current invocation as a dependency tree, with entry tasks at the root. Shared dependencies expand once; later occurrences use `↪` and show the same task logs. With tree focus, ↑/↓ selects a task, ←/→ collapses or expands a branch, and Enter or Space toggles it. On a shared reference, Enter or → jumps to its first occurrence. At widths below 70 columns, Tab switches between a full-width tree and logs.

In the log pane, use the mouse wheel or ↑/↓ to scroll, PgUp/PgDn to page, Home to reach the first log, and End or `f` to follow the latest output. Scrolling up pauses following while new logs continue to arrive. Tab switches keyboard focus between logs and the dependency tree; `[` / `]` also switch tasks, and `a` shows all logs. Each task keeps its reading position. `/` opens a history search, Enter applies it, and Esc clears the filter.

The complete session history is stored in a private temporary file, with a compact in-memory index and a bounded cache of rendered pages. Earlier logs remain available instead of being discarded. The temporary file is deleted on normal exit; storage failures are reported and stop the run. When mise finishes, whether successfully or with an error, TUI automatically closes and returns the original exit code. This also applies while scrolling or searching; no keypress is required. During execution, Ctrl+C or `q` stops the process tree and restores the terminal. A short log tail remains in the parent terminal after exit.

For common tools that disable colors when piped, TUI advertises color support without replacing explicitly configured color preferences. Tools that require a real TTY for their interface still need raw mode.

Task labels describe whether output has been seen. They do not infer readiness, success or cache hits from child text. The overall result follows mise's exit code; per-task completion remains `unknown` because mise does not expose reliable structured lifecycle events.

One forwards child logs without scanning or replacing secret values. One's own messages and structured results do not print injected values. A child that prints a value will show it in stream, TUI, Dashboard and any native log cache.

JSON/YAML plans use `one-cli/task-plan/v1`; results use `one-cli/task-result/v1`. Child logs go to stderr in structured mode.
