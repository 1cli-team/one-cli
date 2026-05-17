---
title: Quick start
description: Create and start a usable One workspace in five minutes with create / add / the first project.
---

Use this page to get a first usable One workspace and start the first project.

**For**: first-time users after installation, anyone checking whether the environment works, and people who want a fast feel for the concepts.

**You will finish with**: a Web project you can open in your browser.

> Not installed yet? Start with [Installation](/en/docs/installation/).
> Want the production-grade end-to-end path? Jump to [Create a production-ready workspace](/en/tutorials/first-workspace/).

## Step 1: Create The Workspace

```bash
one create my-app
cd my-app
```

This creates the `my-app/` directory. Run the remaining commands from inside that directory.

## Step 2: Add A Web Project

The quick start uses one Web project that can be opened directly in a browser:

```bash
one add react-spa --name web
```

`one add` does not download the packages the project uses. The next step does that.

## Step 3: Download Dependencies And Start The Project

Before the project can run for the first time, download the packages it uses. Copy this command and run it from the workspace root; the first run can take a little while:

```bash
pnpm install
```

After the download finishes, start the Web project:

```bash
pnpm -C apps/web dev
```

Open the `Local: http://localhost:.../` URL printed by the terminal. This Web example does not require preset `.env` values, so the quick start does not need env setup.

## Done

You now have your first Web project running:

| Command | What it did |
|---|---|
| `one create` | Created the workspace |
| `one add` | Added one Web project |
| `pnpm install` | Downloaded the packages the project uses |
| `pnpm -C apps/web dev` | Started the first Web project |

Continue by goal: use `one env` for environment variables and `one deploy` for production deploys. Container image build / push is a lower-level deploy step; only open the advanced docs when you need to control it directly.

## Next

Pick the path that matches your goal:

- **Going to production?** -> [Create a production-ready workspace](/en/tutorials/first-workspace/)
- **Need Infisical secrets?** -> [Environment variables guide](/en/tutorials/env-vars/)
- **Want Claude to run One CLI for you?** -> [Install skill to agent](/en/tutorials/skills-install/)
- **Need exact command details?** -> [CLI commands](/en/docs/cli-overview/)
- **Do not want it?** Delete the `my-app/` folder.
