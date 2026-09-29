---
title: Quick start
description: Create a workspace, add a Web project, and let one dev prepare dependencies and start it.
---

Start with an empty workspace, add a React Web project, and run it with One CLI.

> Not installed yet? See [Installation](/en/docs/installation/).

## 1. Create a workspace

```bash
one create my-app
cd my-app
```

One generates workspace directories, `one.manifest.toml`, `mise.toml`, and `AGENTS.md`. Run the remaining commands from `my-app/`.

## 2. Add a Web project

```bash
one add react-spa --name web
```

The project lives in `apps/web/`. One registers it in the Manifest and adds its template tasks to the root `mise.toml`. Run `one templates` to browse other starters.

## 3. Start the dev service

```bash
one dev -p web
```

One prepares mise, project tools, and dependencies as needed, then starts the Web project. The first run needs a network connection and downloads may take some time; later runs reuse matching dependencies. You do not need to install pnpm manually or activate mise first.

Open the `Local: http://localhost:.../` address from the logs in your browser. This example needs no Infisical login or variable binding. Press Ctrl+C to stop the service.

## 4. Discover tasks and build

```bash
one run
one build -p web
```

`one run` lists workspace tasks. `one dev` and `one build` are shorthand for `one run dev` and `one run build`.

To manage projects and dev services in your browser, run:

```bash
one serve
```

Dashboard lets you add projects, manage variables, start dev services, and view their consoles.

## Next

- [Your first workspace](/en/tutorials/first-workspace/): understand generated files and directory conventions.
- [Task management](/en/docs/run/): select multiple projects, customize tasks, and follow logs.
- [Environment variables](/en/tutorials/env-vars/): sign in to Infisical and save project variables.
- [Work with AI](/en/docs/ai-native/): use project guidance, command help, and structured results.
