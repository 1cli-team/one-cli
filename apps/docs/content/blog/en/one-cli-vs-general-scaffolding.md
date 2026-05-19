---
title: "One CLI vs General Scaffolding Tools"
description: "One CLI focuses on workspace contracts and agent-safe operations, not only initial project generation."
date: "2026-05-17"
author: "One CLI Team"
tags: ["scaffolding", "comparison", "workflow"]
---

## The difference is what happens after generation

General scaffolding tools are useful because they save the first setup steps. They create a starter app, write config files, and get the project to a familiar baseline. That is still valuable.

One CLI is aimed at a different problem: what happens after the files exist. A team needs to add more projects, explain the workspace to agents, install dependencies safely, configure deployment targets, and keep the structure understandable over time.

## General scaffolding optimizes the start

Most scaffolding tools optimize for a fast beginning:

- Pick a framework.
- Generate the files.
- Install dependencies.
- Print a next command.

That flow is enough for a single app. It becomes less complete when the repository is a monorepo or when coding agents need structured context.

If later automation has to inspect folders and guess which commands are valid, the original scaffold did not leave behind enough contract.

## One CLI optimizes the workspace lifecycle

One CLI keeps the initial generation flow, but adds a workspace layer around it. The manifest, template registry, JSON output, and bundled skill all exist so future operations have a reliable starting point.

The difference shows up in everyday tasks:

- `one add` can add another project without losing workspace context.
- `one templates -o json` gives agents a parseable template catalog.
- Stable error codes let automation recover without parsing text.
- The bundled skill tells agents how to install dependencies by toolchain.
- The manifest tells tools what the workspace contains.

This makes One CLI less like a one-time generator and more like a workspace contract manager.

## When the distinction matters

If you only need one tiny app, a general scaffolder may be enough. If you expect a workspace to involve multiple projects, agents, deployment paths, or repeated handoffs, the contract becomes more important than the first file write.

One CLI is designed for that second case. It still scaffolds, but the larger purpose is to make the workspace legible to humans, scripts, and coding agents after generation.
