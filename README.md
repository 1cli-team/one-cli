<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./assets/logo-inverted.svg">
    <img src="./assets/logo.svg" alt="One CLI" width="260">
  </picture>
</p>

<p align="center">
  Start a real project, add the parts you need, and give your AI assistant a clear map to work from.
</p>

---

# One CLI

One CLI helps you start and grow product projects without repeating the same setup work every time.

Use it when your project may need more than a single app: a website, an API, docs, a mobile app, a desktop app, shared libraries, local settings, and a way for AI assistants to understand the project.

One CLI gives you an empty workspace first. You can add apps, services, documentation sites, and shared libraries as the product grows.

## Quick Start

Install on macOS or Linux:

```bash
curl -fsSL https://1cli.dev/install.sh | bash
```

Install on Windows 10/11 x64 from PowerShell:

```powershell
irm https://1cli.dev/install.ps1 | iex
```

Both installers verify the release checksum and add `one` to the normal per-user binary location.

Create a workspace and add a project:

```bash
one create my-app
cd my-app
one add react-spa --name web
one dev -p web
```

That gives you a workspace, a first app, and a local way to run it.

## Why Use It

One CLI is useful when you want to:

- start from a clean project foundation
- add a frontend, backend, docs site, mobile app, desktop app, or library later
- keep environment configuration and local settings organized
- let an AI assistant help without guessing how the project is arranged
- use the same simple commands across different kinds of projects

It is not trying to replace your package manager, editor, or hosting provider. It gives the project a shared shape so people, scripts, and AI assistants can work with it more safely.

## What You Can Build

One CLI includes starters for common product work:

| Need | Starters |
|---|---|
| Web apps | Next.js, React SPA, Astro |
| Backends | NestJS API, Go API |
| Documentation | Starlight docs |
| Mobile apps | Expo |
| Desktop apps | Electron |
| Shared libraries | TypeScript library, Go library |

See the available starters:

```bash
one templates
```

Add one to an existing One CLI workspace:

```bash
one add nestjs-api --name api
```

## Daily Workflow

| Command | What it helps you do |
|---|---|
| `one create <workspace>` | Create an empty workspace |
| `one add <starter>` | Add another app, service, docs site, or library |
| `one env` | Review and manage environment variables |
| `one login` | Sign in to Infisical with your browser |
| `one serve` | Inspect workspaces, manage the current account and shared credentials |
| `one run [task]` | Discover static mise.toml tasks and execute them through Process Compose |
| `one exec <project> -- <command>` | Execute a command with the selected project environment |

`dev`, `build`, `test`, and `lint` are task names. They all use the same shorthand: `one <task>` → `one run <task>`. For example, `one dev` runs `one run dev`, and `one build -p web` runs `one run build -p web`.

Full command docs live at [1cli.dev](https://1cli.dev).

## Work With AI Assistants

New workspaces include one `AGENTS.md` in the active One CLI language. The editable [English](packages/cli/internal/modules/creation/templates/AGENTS.en-US.md) and [Chinese](packages/cli/internal/modules/creation/templates/AGENTS.zh-CN.md) Markdown templates cover execution through `one run`, complex scripts in `scripts/*.mts`, and variable management through One. Publishing credentials belong in global variables configured on the Dashboard’s Shared credentials page; tasks inject them with `one exec --global`. Existing guidance remains team-owned when the language changes or projects are added.

You can ask an assistant for project-level changes in natural language, for example:

> Create a product workspace with a web app and an API.

> Add a docs site to this project.

> Add a mobile app next to the existing backend.

The assistant can read `one.manifest.toml` and project README files, then use One CLI commands with `-o json` to make project changes.

## Local Settings

One CLI manages variables in Infisical and injects them directly into commands. Workspace bindings live in the manifest `[env.infisical]` table; `.env` files are not loaded or exported. Run `one login` to sign in with your browser; the single session is stored in the OS keyring, with no plaintext fallback. Use `one whoami` to inspect status and `one logout` to remove the local session.

Run `one serve` for account settings, workspaces, and shared credentials. Workspace binding changes use a reviewed, revision-checked TOML draft; project settings display derived configuration. Remote variable edits take effect immediately; lists omit values and reveal/copy fetch plaintext only on demand.

Choose shared credential storage with `one env bind --global`. Agents discover environments and folders through `one env --global` and `one env list --global`, then execute with `one exec --global --env dev --path /folder --keys KEY -- command`. One does not print injected values. Child logs preserve their original content and formatting.

## Project Map

Every One CLI project has a `one.manifest.toml` file at the root. Most users do not need to edit it by hand.

Think of it as the project map. It records which parts exist, where they live, and which starter created them. One CLI reads it when you add, run, build, or inspect parts of the project. `one serve` writes it only after an explicit reviewed, revision-checked Dashboard action; other repository changes stay in the normal code-review workflow.

## Repository Layout

If you want to work on One CLI itself, the repository is organized like this:

| Path | Purpose |
|---|---|
| `one.manifest.toml` | The four projects managed by One CLI itself |
| `packages/cli` | The One CLI app and its public Go packages |
| `packages/kernel` | Shared Go kernel |
| `packages/templates` | Starters used by `one add` |
| `mise.toml` | Task definitions and tool versions |
| `apps/docs` | Documentation website |
| `apps/dashboard` | Local workspace, account, and global-variable Dashboard opened by `one serve` |
| `assets` | Brand assets, including the logo |

This repository is also a One CLI workspace: `dashboard`, `docs`, `cli`, and `kernel`. Template directories under `packages/templates` and test fixtures are source assets, not registered projects. `packages/cli` keeps its existing location because it also exports Go packages with that module path.

Bootstrap a fresh checkout without requiring an installed `one`:

```bash
mise trust
mise install
mise run install
```

If upgrading an existing checkout, run `mise run install` again before using the new commands. This rebuilds the local version and points the `one` launcher at `packages/cli/bin/one`, avoiding an older installed release.

Then use the workspace commands:

```bash
one                             # Inspect this workspace
one run                         # List root and project tasks
one run dev                     # Dashboard API + Vite UI for this workspace
one run dev -p docs              # Documentation at http://localhost:3000
one run build -p cli             # Prepare embedded resources and build the CLI
one run test -p kernel           # Test the shared Go kernel
one run check                   # The complete repository gate; one check is shorthand
one serve                       # Manage this repository in the Dashboard
```

`one run dev -p dashboard` starts the Vite UI; run `one run dev -p cli` in another terminal for its API, or use the combined `one run dev` task. Both the development API and `one serve` use this repository as their workspace. No Infisical binding is required to build, test, or start these projects.

Root tasks remain native mise commands, so CI and first-time installation can still use `mise run build`, `mise run check`, and `mise run install`. Project tasks come from package scripts and Taskfiles. Only the root `mise.toml` is used: project tasks are namespaced as `cli:build` or `dashboard:dev` and select their directory with `dir`. CLI tasks declare their embedded-resource prerequisites there; CLI tests also build the binary used by E2E tests. Run `one init mise` after changing the project task catalogue to refresh the tracked native tasks. Running or listing tasks does not regenerate configuration. Dashboard discovers access links from process output; dev commands are defined in mise.toml and executed by Process Compose.

Read [CONTRIBUTING.md](./CONTRIBUTING.md) before opening a pull request.

## Documentation

- [Installation](https://1cli.dev/docs/installation/)
- [First project tutorial](https://1cli.dev/tutorials/first-workspace/)
- [Templates](https://1cli.dev/en/docs/templates/)
- [Command reference](https://1cli.dev/docs/cli-overview/)
- [Error codes](https://1cli.dev/docs/error-codes/)

## License

MIT.

### Tasks and development

`one <task>` is shorthand for `one run <task>`. Built-in commands take precedence; use `one run env` for a task that shares a built-in name.

```sh
one run test -p api               # Explicit task entry
one test -p api                   # Same task through shorthand
one dev -p web -p api             # Concurrent development services
one dev -p web --ui raw           # Native terminal input for the selected service
one build -p web -p api
```

One uses official **Process Compose 1.122.0** for scheduling; mise installs tools and prepares version environments. `one run dev --ui tui` opens One's interface: running tasks above a complete dependency tree, with logs on the right. Tasks stay in the tree while running and retain their result after completion. No service declarations are needed. Tab switches between the sidebar and logs; arrows navigate, `/` searches, and `f` follows the latest output. Use `--ui stream` for streaming output.

Log prefixes use compact `[task name] body` formatting without name padding and receive consistent task colors. Log bodies retain their original styling and indentation. stdout and stderr share the same task color; copies contain plain text.

Drag in the log pane and press `y` to copy the selection. With no selection, `y` opens a menu: `v` copies visible logs, `n` the full task name, and `l` log history (or current search matches). `c` shows a fixed, full-width log snapshot and releases mouse reporting, so VS Code can select normally and use Cmd+C. Esc returns to the interface. Tasks continue during copying. Local clipboard writes report their result; remote OSC 52 requests report only that a request was sent. One retains the complete session log in a private temporary journal, removed when execution ends.

Ctrl+C or a task failure stops the invocation and its child processes. The terminal restores automatically when execution ends. Raw mode requires a graph with one executable task and preserves terminal input. Structured results stay on stdout and child logs go to stderr. Save `"taskUI": "tui"` or `"taskUI": "stream"` in `~/.config/one/preferences.json` for a personal default.

Process Compose has no global concurrency limit. One rejects `--concurrency` values below the number of command tasks. `one run build --force` bypasses timestamp freshness; artifact caching is disabled. Unsupported selected task fields fail before commands start. See the [task guide](apps/docs/content/docs/en/run.md) for the static compatibility subset, variables, and terminal behavior.
