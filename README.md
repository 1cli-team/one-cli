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
- keep local settings and deployment choices out of random notes
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
| `one dev [project]` | Run every project, or one selected project, locally |
| `one deploy [project]` | Choose a target on first deploy, then deploy |
| `one env` | Review and manage environment variables |
| `one configure` | Manage local connections and preferences |
| `one serve` | Inspect Workspaces and Projects; manage local Profiles and bindings |
| `one ci [enable\|sync\|disable]` | Optionally manage generated GitHub Actions workflows |

Full command docs live at [1cli.dev](https://1cli.dev).

## Work With AI Assistants

One CLI is designed to make AI-assisted project work less fragile.

After installing the bundled skills:

```bash
one skills install
```

you can ask an assistant for project-level changes in natural language, for example:

> Create a product workspace with a web app and an API.

> Add a docs site to this project.

> Add a mobile app next to the existing backend.

The assistant can use One CLI commands instead of inventing a folder structure from scratch.

## Local Settings

Some projects need environment values, deployment accounts, or image registry settings. One CLI keeps those in your local user config, not inside the project files you share with the team.

For a guided browser-based setup:

```bash
one configure open
```

The page only binds to your local machine by default, so it is a better place for sensitive values than a chat window or a shared document. Workspace code, Project fields, Backend choices, and `one.manifest.json` are view-only in the Dashboard. Its writable surface is limited to machine Profiles and the Profile name selected for a Workspace or Project in an environment.

Profile definitions and credentials live in `~/.config/one/config.json` and `credentials.json`. They are machine-global, so Profile CRUD in Settings is not environment-scoped. The Dashboard UI offers Development, Preview, and Production binding contexts; those selections live separately in `~/.config/one/profile-bindings.json`, keyed by canonical Workspace root and environment. The core/API can also store safe custom environment IDs for non-UI workflows. These files never upgrade or modify the repository manifest.

## Project Map

Every One CLI project has a `one.manifest.json` file at the root. Most users do not need to edit it by hand.

Think of it as the project map. It records which parts exist, where they live, and which starter created them. One CLI reads it when you add, run, deploy, or inspect parts of the project. `one serve` never writes it; repository changes stay in the normal code-review workflow.

## Repository Layout

If you want to work on One CLI itself, the repository is organized like this:

| Path | Purpose |
|---|---|
| `packages/cli` | The One CLI app |
| `packages/templates` | Starters used by `one add` |
| `packages/skills` | Guidance installed for AI assistants |
| `apps/docs` | Documentation website |
| `apps/dashboard` | Local Workspace, Project, and Profile Dashboard opened by `one serve` |
| `assets` | Brand assets, including the logo |

Common contributor commands:

```bash
pnpm install
task check
task build
task test
task verify-docs
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
