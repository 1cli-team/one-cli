---
title: one run
description: Run workspace tasks with official Process Compose and mise tool environments.
---

`one run` reads supported static tasks from `mise.toml`, prepares dependencies and project variables, and runs the graph with official **Process Compose 1.122.0**. mise continues to install tools and prepare their versions. One does not modify mise or implement a second task scheduler.

```sh
one run
one run --verbose
one run --list -p web -o json
one run dev --ui tui
one run build -p web --ui stream
one run test -p api -- -run TestHandler
one run build --dry-run -o json
```

`one <task>` is shorthand for `one run <task>`. Built-in commands take precedence. Root names omit `//:` in the terminal list; project tasks use names such as `web:build`. Missing tasks are errors. Execution and discovery never generate missing tasks; refresh declarations explicitly with `one init mise` after changing package scripts or Taskfiles.

## Tool installation

One reuses an installed Process Compose executable when its version is exactly 1.122.0. Otherwise, it asks mise to install `process-compose@1.122.0`. The first run therefore needs network access; subsequent runs can reuse the installation. It uses the official binary, with no patch or Rust build.

```sh
mise use process-compose@1.122.0
```

A workspace may declare `process-compose = "1.122.0"` in `[tools]`. One supplies a temporary configuration for each invocation; users do not need to maintain `process-compose.yaml`. The native HTTP API is disabled, so One runs do not reserve port 8080.

## Supported task configuration

Creation and `one init mise` still map package scripts and Taskfiles into root tasks:

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

The first integration supports the following static subset:

| Configuration | Behavior |
| --- | --- |
| `run` as a string or string array | Shell commands execute in order; an unsuccessful step stops the task. |
| `depends` | Selects dependencies, each of which must complete successfully before the task starts. Shared dependencies run once. |
| `wait_for` | Adds the same success condition only for matching tasks already selected; it does not select tasks. |
| `dir`, aliases, project scopes | Resolves working directories and canonical task identities before execution. |
| Executable file tasks and local TOML includes | Reads definitions without executing them during discovery. |
| Plain task `env`, unset values, `tools`, `shell` | Prepares task-specific tools and environment before the graph starts. |
| `sources`, `outputs`, `--force` | Timestamp freshness, including recursive `**` globs; no artifact restoration. |
| `raw`, `interactive`, `--ui raw` | Exclusive terminal access for a graph with one executable task. |

Task templates, parameterized dependencies, `depends_post`, task environment plugins, usage specifications and executable cache inputs are unsupported. One reports the selected definition and unsupported field **before starting task commands**. There is no fallback to mise task scheduling. Unselected tasks with unsupported fields can remain in the workspace. This is a supported subset, not complete mise task compatibility.

Commands may call native leaf tools such as pnpm, Go or Task. Existing scripts that themselves call `mise run` retain that explicit nested scheduler; replace those script calls when migrating a workspace completely.

Local and profile configurations are read statically. Discovery, normal planning and dry-run use the same graph. Dry-run does not install tools, fetch variables, evaluate templates or run cache-input commands.

Keep the `# one:managed-v1` marker for incremental generation. Generated-field conflicts still produce `MISE_CONFIG_CONFLICT`. The repository's bootstrap, CI and Git hooks may continue to use unmodified `mise run` independently of One execution.

## Arguments and raw mode

Everything after `--` is forwarded as arguments to the last command of one selected entry, or directly to a file task. Dependencies do not receive these arguments. Multiple selected entries with arguments are rejected. `:::` remains reserved.

POSIX shell argument forwarding preserves spaces, quotes, Unicode and metacharacters. On Windows, file tasks accept arguments; inline tasks with arguments require an explicit PowerShell shell. One rejects cmd.exe forwarding before execution rather than silently expanding argument text.

`--ui raw` uses Process Compose's foreground execution and preserves stdin/stdout/stderr for one executable task. Graphs with multiple executable tasks must use stream or TUI. `one exec web -- pnpm add axios` remains available for arbitrary commands with project tool versions and One variables.

## Project variables and results

Task ownership follows the deepest registered project directory containing its effective working directory. Root aggregates do not receive a union of child variables.

When projects enable environment variables, One makes a single recursive request to Infisical per invocation and builds immutable project snapshots in memory. A later invocation fetches fresh values. `--env` selects a declared environment, defaulting to `dev`. Each project inherits root, ancestor, and project-directory variables in that order, with closer folders overriding matching keys. Variables from other projects or deeper subdirectories are excluded. Values override matching shell, mise and task variables. Each leaf receives only its project's snapshot, including empty values. Child processes inherit the same environment.

Infisical recursive reads support at most 20 directory levels. Projects beyond that depth fail before the request. A response containing a secret without an absolute folder path also fails, preventing incomplete or incorrectly scoped variables from being distributed.

Environment values and commands are sent to private leaf processes through an authenticated loopback channel. Generated YAML contains graph metadata and worker identities; it contains no injected variable values. No environment plugins or temporary binding TOML are generated. The channel and private configuration directory are closed and removed when the invocation ends.

Startup progress goes to stderr. Cancellation interrupts preparation and execution. The scheduler preserves the failing command's exit code; SIGINT and SIGTERM return 130 and 143 respectively. Selected tasks report `succeeded`, `failed`, `cancelled`, `cached` or `skipped`; an unstarted dependent is skipped. A child that prints a secret still prints it in logs: One does not filter child output.

JSON/YAML plans use `one-cli/task-plan/v1` with `runtime: process-compose`; results use `one-cli/task-result/v1`. Child logs go to stderr in structured mode.

## Freshness and concurrency

There is no artifact cache or `--cache` option. Existing `cache.enabled` metadata does not enable artifact storage. Plain `sources` and `outputs` skip a task only when every output exists and is at least as new as its inputs and task definition. Missing or unreadable inputs/outputs force execution. Paths are relative to `dir`; selected output locations must not overlap.

```toml
[tasks."web:build"]
dir = "apps/web"
run = "pnpm run build"
sources = ["src/**/*", "package.json", "../../pnpm-lock.yaml"]
outputs = ["dist"]
```

`--force`, forwarded arguments and raw mode bypass freshness. Tasks receiving remote variables, and tasks downstream of them, also bypass freshness so rotated, removed and empty variables cannot reuse stale outputs.

Process Compose starts tasks as their dependencies permit. It has no global slot limit. The compatibility `--concurrency` flag rejects limits smaller than the selected command-task count; omit it for normal scheduling.

## One TUI and copying

`--ui tui` uses One's interface with official Process Compose running headless. The upper sidebar contains tasks that have actually started and have not finished, ordered by start time. Silent tasks appear too; waiting tasks and aggregates do not appear as running processes. The lower sidebar retains the complete dependency tree, including running tasks and completed results. Shared dependencies use ↪ and waiting edges use ◇. No service declarations or upstream modifications are needed.

Tab switches between sidebar and logs. Sidebar arrows select, collapse/expand and locate a running task in the tree; Enter toggles branches or jumps to shared dependencies. Deep branches start collapsed. The two sidebar sections scroll independently, with stable heights. When a selected running task finishes, selection moves to its tree node while preserving its logs, search and reading position. Below 70 columns, Tab switches between full-width logs and sidebar.

Logs wrap to the pane width. Use `/` to search, Home/End to browse history or follow the tail, and `f` to resume following. The selected task's complete identity appears in the heading; `y` then `n` also copies the full identity.

Task prefixes use the compact `[task name] body` format without name padding; body indentation is preserved. Prefixes use distinct colors from a 12-color palette, allocated consistently for the selected task graph, including stdout and stderr. Larger graphs reuse colors. Log bodies retain their own colors; copied text has no ANSI styling. Explicit color-off environment settings disable prefix coloring.

Drag in the log pane to select text, then press **y** to copy. The view freezes while selecting, but processes and log collection continue. With no selection, **y** opens a menu: **v** copies visible logs, **n** the complete task name, and **l** the current task's log history (or matching records when searching). Copies remove ANSI and UI decoration, preserve Unicode and real newlines, and do not insert newlines for soft wraps. Switching task clears a selection; selecting another occurrence of the same task preserves it. Resize reflows the frozen text.

Press **c** for a fixed full-width log snapshot with mouse reporting disabled. In VS Code, drag normally and use the terminal's copy shortcut (Cmd+C on macOS). **Esc** returns with the prior reading/follow state; **f** returns and follows new output. Ctrl+C always cancels the invocation. Search input treats y/c as text. Local macOS uses pbcopy, Windows uses Set-Clipboard, and Linux uses wl-copy/xclip when available. Remote sessions use OSC 52 and display “request sent”, since terminal support has no reliable acknowledgement. Oversized OSC requests fail explicitly; content is never silently truncated. Clipboard errors preserve the selection and leave tasks running.

One stores **complete session history** in a private temporary log journal, removed when the invocation ends. Only indexes and a bounded render cache stay in memory. All tasks finishing closes the interface automatically, including during copying or searching. Cancellation and failure restore the terminal too.

`auto` selects TUI for multiple tasks in an interactive terminal and stream otherwise. CI and structured output use stream. `--ui` overrides the optional `taskUI: "tui"` or `"stream"` preference in `~/.config/one/preferences.json`.
