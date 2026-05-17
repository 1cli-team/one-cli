---
title: one dev
description: Start local development processes from Procfile.dev.
---

`one dev` reads the workspace root `Procfile.dev` and starts local development processes with an available supervisor. `one create` and `one add` sync `Procfile.dev`, so workspaces have this entry point by default.

## Usage

```bash
one dev [-p <name|path>] [--dry-run]
```

## Options

| option | purpose |
|---|---|
| `-p`, `--project <name|path>` | start one project by manifest `name` or `relativeDir` |
| `--dry-run` | print the supervisor command without starting processes |
| `-o`, `--output <fmt>` | `json` / `yaml` / `text` |

## Interactive Mode

`one dev` has no wizard. It starts processes directly from the manifest and `Procfile.dev`. Use `--dry-run` when you want to inspect the supervisor command first.

## Runner

One CLI looks for a Procfile supervisor on PATH. Supported runners include overmind, hivemind, foreman, and honcho.

```bash
one dev
one dev -p web
one dev -p apps/web --dry-run
```

## Procfile.dev

Example:

```text
api: pnpm --dir services/api dev
web: pnpm --dir apps/web dev
```

The project list still comes from `one.manifest.json`; `Procfile.dev` is a generated runtime artifact.

## Common errors

| code | fix |
|---|---|
| `DEV_NO_SUPERVISOR` | install overmind, hivemind, foreman, or honcho |
| `DEV_PROCFILE_MISSING` | rerun `one add` to trigger Procfile sync, or check workspace generation |
| `DEV_PROJECT_NOT_FOUND` | use a manifest project `name` or `relativeDir` for `-p` |

## Next

- [Local dev orchestration](/en/tutorials/dev-local/)
- [one run](/en/docs/run/)
- [Workspace manifest](/en/docs/manifest/)
