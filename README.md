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

One CLI gives you a ready project folder first. You can add more pieces later as the product grows.

## Quick Start

Install on macOS or Linux:

```bash
curl -fsSL https://one.torchstellar.com/install.sh | bash
```

Windows users can download a build from [GitHub Releases](https://github.com/torchstellar-team/one-cli/releases/latest).

Create a project:

```bash
one create my-app
cd my-app
one add react-spa --name web
one dev
```

That gives you a project folder, a first app, and a local way to run it.

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

Add one to an existing One CLI project:

```bash
one add nestjs-api --name api
```

## Daily Workflow

| Command | What it helps you do |
|---|---|
| `one create <name>` | Start a new project folder |
| `one add <starter>` | Add another app, service, docs site, or library |
| `one templates` | See what you can add |
| `one configure` | Save local settings for environments, deployment, and images |
| `one serve` | Open a local browser page for sensitive settings |
| `one dev` | Run the project locally |
| `one run -- <cmd>` | Run a command with the right local environment |
| `one deploy` | Deploy selected parts of the project |
| `one skills install` | Teach supported AI assistants how to use One CLI |

Full command docs live at [one.torchstellar.com](https://one.torchstellar.com).

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
one serve
```

The page only binds to your local machine by default, so it is a better place for sensitive values than a chat window or a shared document.

## Project Map

Every One CLI project has a `one.manifest.json` file at the root. Most users do not need to edit it by hand.

Think of it as the project map. It records which parts exist, where they live, and which starter created them. One CLI reads it when you add, run, or deploy parts of the project.

## Repository Layout

If you want to work on One CLI itself, the repository is organized like this:

| Path | Purpose |
|---|---|
| `packages/cli` | The One CLI app |
| `packages/templates` | Starters used by `one add` |
| `packages/skills` | Guidance installed for AI assistants |
| `apps/docs` | Documentation website |
| `apps/dashboard` | Local settings UI opened by `one serve` |
| `assets` | Brand assets, including the logo |

Common contributor commands:

```bash
task build
task test
task verify-docs
```

Read [CONTRIBUTING.md](./CONTRIBUTING.md) before opening a pull request.

## Documentation

- [Installation](https://one.torchstellar.com/docs/getting-started/installation/)
- [First project tutorial](https://one.torchstellar.com/tutorials/first-workspace/)
- [Templates](https://one.torchstellar.com/docs/templates/)
- [Command reference](https://one.torchstellar.com/docs/reference/cli-overview/)
- [Error codes](https://one.torchstellar.com/docs/reference/error-codes/)

## License

MIT.
