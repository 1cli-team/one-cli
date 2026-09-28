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
one dev web
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
| `one dev [projects...]` | Run all or selected projects; native output for one task, TUI for multiple tasks |
| `one build [projects...]` | Build all or selected projects in dependency order; optional bounded concurrency |
| `one env` | Review and manage environment variables |
| `one login` | Sign in to Infisical with your browser |
| `one serve` | Inspect workspaces, manage the current account and shared credentials |
| `one run [task]` | Discover and execute workspace tasks through mise |
| `one exec <project> -- <command>` | Execute a command with the selected project environment |

Full command docs live at [1cli.dev](https://1cli.dev).

## Work With AI Assistants

New workspaces include bilingual `AGENTS.md` guidance. Agents can inspect `one run --list -o json`, preview tasks with `one run build --dry-run`, and consult command-specific help.

You can ask an assistant for project-level changes in natural language, for example:

> Create a product workspace with a web app and an API.

> Add a docs site to this project.

> Add a mobile app next to the existing backend.

The assistant can read `one.manifest.json` and project README files, then use One CLI commands with `-o json` to make project changes.

## Local Settings

One CLI manages local dotenv and Infisical variables. Run `one login` to sign in with your browser; the single session is stored in the OS keyring, with no plaintext fallback. Use `one whoami` to inspect status and `one logout` to remove the local session.

Run `one serve` for account settings, workspaces, and shared credentials. Workspace and project configuration changes share one reviewed, revision-checked Manifest draft. Remote variable edits take effect immediately; lists omit values and reveal/copy fetch plaintext only on demand.

Choose shared credential storage with `one env bind --global`. Agents discover environments and folders through `one env --global` and `one env list --global`, then execute with `one exec --global --env dev --path /folder --keys KEY -- command`. Explicit scope and best-effort masking reduce accidental exposure; they do not isolate arbitrary programs running as the same OS user. Use least-privilege remote permissions.

## Project Map

Every One CLI project has a `one.manifest.json` file at the root. Most users do not need to edit it by hand.

Think of it as the project map. It records which parts exist, where they live, and which starter created them. One CLI reads it when you add, run, build, or inspect parts of the project. `one serve` writes it only after an explicit reviewed, revision-checked Dashboard action; other repository changes stay in the normal code-review workflow.

## Repository Layout

If you want to work on One CLI itself, the repository is organized like this:

| Path | Purpose |
|---|---|
| `packages/cli` | The One CLI app |
| `packages/templates` | Starters used by `one add` |
| `mise.toml` | Workspace scheduling, tools, and artifact cache declarations |
| `apps/docs` | Documentation website |
| `apps/dashboard` | Local workspace, account, and global-variable Dashboard opened by `one serve` |
| `assets` | Brand assets, including the logo |

Common contributor commands:

```bash
pnpm install
mise run check
mise run build
mise run test
mise run verify-docs
```

Read [CONTRIBUTING.md](./CONTRIBUTING.md) before opening a pull request.

## Documentation

- [Installation](https://1cli.dev/docs/installation/)
- [First project tutorial](https://1cli.dev/tutorials/first-workspace/)
- [Templates](https://1cli.dev/templates/)
- [Command reference](https://1cli.dev/docs/cli-overview/)
- [Error codes](https://1cli.dev/docs/error-codes/)

## License

MIT.

### Development and build terminals

```sh
one dev web api                   # Run a selected set of projects in parallel
one dev --select                  # Search and select projects interactively
one dev web                       # Keep the project's native colors, progress, and input
one dev web api --keep-going      # Keep peers running if a project exits
one dev web api --ui=stream       # Use continuous prefixed logs
one build web api --concurrency=4 # Build ready tasks concurrently, respecting local dependencies
```

Development uses native output for one project and the supervisor TUI for multiple projects. Finite tasks use mise streaming output; `one build web --ui raw` preserves interactive input and disables artifact caching. Structured results stay on stdout and child logs go to stderr.

In the development TUI, use ↑/↓ to select a project, Enter to send input, and Ctrl+] to return to navigation. PgUp/PgDn scroll history, f resumes following, / searches, and h hides the project list. Use r to restart and s to stop the selected project. Ctrl+C stops the session and its process trees.

Build concurrency defaults to 1. Local Node dependencies are included automatically and build before their consumers. `one run build --cache off --force` always executes the build. See the [task guide](apps/docs/content/docs/en/run.md) for cache declarations and Actions examples.

The development TUI supports Linux and macOS. Windows uses native single-project output and streaming multiple projects.
