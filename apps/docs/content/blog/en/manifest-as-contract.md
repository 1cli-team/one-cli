---
title: "Why One CLI Treats the Manifest as the Contract"
description: "one.manifest.json is not just scaffold output. It is the shared contract read by humans, the CLI, and coding agents."
date: "2026-05-12"
author: "One CLI Team"
tags: ["manifest", "agent", "monorepo"]
---

## The manifest is the workspace source of truth

In a One CLI workspace, `one.manifest.json` is one of the most important files. It is not a temporary cache for the CLI. It describes the apps, packages, preset, and operational boundaries that make up the workspace.

Traditional scaffolding tools usually stop after writing files. After that, the project structure becomes a pile of conventions, and later tooling has to infer intent from folders, package scripts, or README text. One CLI writes those decisions into the manifest so future commands and agents can read the same structured context.

## Why folder structure is not enough

Folders can answer where files live, but they cannot reliably explain why a folder exists. `apps/web` might be a Next.js app or a Vite React app. `packages/shared` might be a publishable library or an internal utility package.

Humans can often infer the difference. Agents should not have to guess.

The manifest makes those semantics explicit:

- Project type and template origin can be read deterministically.
- `one add` can decide where a new module belongs.
- Agents can confirm workspace boundaries before installing dependencies or running commands.
- CI, deployment, and local orchestration can refer to the same project inventory.

## Agents need verifiable context

Telling an agent that a repository is a full-stack project is not enough. The agent needs to know which templates created it, which package manager it uses, where Go modules live, which files are generated, and which files are user-maintained business code.

That is why One CLI pairs the manifest with the bundled `one-cli` skill. The manifest describes the current workspace. The skill describes how an agent should operate on it. Together, they move the workflow from guessing to checking facts first.

## A steadier workflow

A typical flow looks like this:

```bash
one create my-stack --preset 1.bgok.fnav.ei --yes
cd my-stack
one add nextjs-app --name admin --yes
```

Each step records the structure change back into the manifest. Human developers can inspect the real boundary of the workspace. Agents can start from the manifest instead of reverse-engineering a pile of scripts and directory names.

The value of the manifest is not that there is one more config file. The value is that scaffolding, docs, CLI commands, and agent behavior converge on the same engineering contract.
