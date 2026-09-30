---
title: CLI overview
description: Daily commands and advanced entry points.
---

| Command | Purpose |
| --- | --- |
| `one create` | Create a workspace |
| `one add` | Add a project |
| `one env` | Manage project variables |
| `one login` / `one whoami` / `one logout` | Single browser session |
| `one env --global` | Discover global variable locations and environments |
| `one exec` | Run a command with injected variables |
| `one serve` | Open the Dashboard |
| `one locale` | Local language preference |
| `one upgrade` | Update One CLI to the latest stable release |
| `one init mise` / `one init hooks` | Workspace tool configuration |
| `one run` | List and execute workspace tasks |
| `one templates` | List project templates |
| `one hk` | Run Git hook checks |

`dev`, `build`, `test`, and `lint` are ordinary workspace task names. `one <task>` is shorthand for `one run <task>`: `one dev` runs `one run dev`, and `one build` runs `one run build`. See [tasks](/en/docs/run/) for the shared flags and task discovery.

Discover the full catalogue with `one help --all`, then read command-specific `--help`. Agents discover metadata and execution options through the CLI, without copying commands from the Dashboard. See [login and shared credentials](/en/docs/login/).
