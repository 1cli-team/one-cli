---
title: "What Preset IDs Should and Should Not Do"
description: "A preset ID is a reproducible template-combination code. It should not grow into another business-scenario DSL."
date: "2026-05-12"
author: "One CLI Team"
tags: ["preset", "templates", "product-design"]
---

## A preset is a template combination, not a business script

One CLI preset IDs express template selections. A preset can represent a Go API, a Next.js app, and a TypeScript library in one compact input. The goal is to make scaffolding short, stable, and reproducible, not to encode every product intention into a string.

That boundary matters. If a preset starts representing business scenes like ecommerce admin, content platform, or social app, it quickly becomes another DSL. A DSL needs an interpreter, migration policy, compatibility rules, and a long tail of exceptions. That weight belongs outside the core CLI surface.

## The value of compact codes

Compact preset codes are useful because they are deterministic:

- The same preset expands to the same template combination on every machine.
- The preset code can be stored in the manifest for review and replay.
- Docs, the template builder, and agent prompts can share the same short code.
- Unsupported versions can fail before writing workspace files.

These capabilities are valuable for scaffolding and governance, but they stay at the project-structure layer. They do not model the user’s business domain.

## Custom names belong in fallback commands

Template selection can be encoded, but project names and directory names are often better represented by commands. The same `nextjs-app` template might be added as `web`, `admin`, or `console`. Those names are not the core meaning of the preset.

```bash
one create my-stack --preset 1.bgok.fnav.ei --yes
one add nextjs-app --name admin --yes
```

This split keeps presets responsible for which templates were selected, while CLI commands handle what those templates are called inside the current workspace.

## Keep the surface small

The One CLI product surface should stay small. Commands such as `create`, `add`, `templates`, `configure`, `serve`, and `skills` cover the main workflows. A preset is one input to `create`; it does not need to become a new center of gravity.

A good preset design helps users type less, helps agents guess less, and keeps generated output replayable. It does not need to explain every business scenario, and it should not replace a clear template catalog or manifest.
