---
title: "Why Monorepo Scaffolding Needs a Manifest"
description: "A monorepo scaffold should record why projects exist, not just where files were generated."
date: "2026-05-17"
author: "One CLI Team"
tags: ["manifest", "scaffold", "monorepo"]
---

## File layout is only the first layer

Most scaffolders can create a folder layout. That is useful, but a monorepo needs more than folders. A workspace has projects, roles, dependencies, deployment targets, and conventions that should remain visible after the initial generation step.

Without a manifest, later tooling has to infer intent from directory names and scripts. That might work when the repository is new, but it becomes harder as teams add services, packages, and deployment paths.

## The manifest explains intent

One CLI writes `one.manifest.json` so the workspace keeps a structured record of its own shape. The manifest gives future commands and agents a place to read project inventory and operational intent.

That matters for common tasks:

- Adding another frontend or backend with `one add`.
- Deciding where dependencies should be installed.
- Understanding which templates created the current projects.
- Keeping generated guidance aligned with the workspace.
- Letting agents inspect facts before editing files.

The manifest is not a replacement for code. It is the map that tells tools how the code is organized.

## Scaffolding should be reversible as knowledge

The first scaffold command contains useful decisions: which template was chosen, which deploy target was selected, and which environment strategy applies. If those decisions disappear into files, every future tool has to rediscover them.

A manifest keeps the decisions readable. That makes the workspace easier to automate because the next command does not have to start from zero.

```bash
one create product-suite --yes -o json
one add nestjs-api --name api --yes -o json
one add nextjs-app --name web --yes -o json
```

Each step should leave behind enough structure for the next step to be safer.

## The agent angle

Coding agents need boundaries. A manifest lets an agent answer basic questions before acting: Am I in a One workspace? Which projects exist? What is generated? Which package manager is expected?

That is why manifest-driven scaffolding fits AI-native development better than one-shot folder generation. The tool does not only create files. It preserves the workspace facts that agents need later.
