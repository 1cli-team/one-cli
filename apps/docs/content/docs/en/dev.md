---
title: one dev
description: Start every developable project, or one selected project.
---

`one dev` reads each project's development command from the manifest and runs it through One CLI's built-in supervisor.

## Usage

```bash
one dev [project] [--dry-run]
```

## Options

| option | purpose |
|---|---|
| positional `project` | start one project by manifest `name` or `relativeDir` |
| `-p`, `--project <name|path>` | legacy selector for scripts and CI |
| `--dry-run` | print the supervisor command without starting processes |
| `-o`, `--output <fmt>` | `json` / `yaml` / `text` |

## Interactive Mode

If a selected Node project has no installed dependencies, a terminal asks whether to run the detected package manager's install command. Confirming installs and continues; declining exits successfully. Non-interactive calls return `DEPENDENCIES_NOT_INSTALLED` with the exact install command.

## Runner

One CLI's built-in supervisor starts all developable projects by default, or one positional project.

```bash
one dev
one dev web
one dev apps/web --dry-run
```

## Common errors

| code | fix |
|---|---|
| `DEPENDENCIES_NOT_INSTALLED` | run the install command from remediation, then retry |
| `SUBPROJECT_NOT_FOUND` | use a project `name` or `relativeDir` |

## Next

- [Local dev orchestration](/en/tutorials/dev-local/)
- [one run](/en/docs/run/)
- [Workspace manifest](/en/docs/manifest/)
