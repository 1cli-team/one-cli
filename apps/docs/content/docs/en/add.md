---
title: one add
description: Add a templated project to an existing workspace.
---

`one add` selects a template from the registry, writes it into the workspace, registers it in the manifest, and syncs template defaults such as Dockerfile, Kustomize, workflows, and AI guides.

There are two entry points:

- Human first run: run `one add` and use the interactive picker to choose the category, template, and project name.
- Scripted or known-template flow: run `one templates` to see template IDs, then run `one add <template-id> --name <project-name>`.

`template-id` is the template ID, such as `nestjs-api`, `nextjs-app`, or `ts-library`. It is not the project name; the project name comes from `--name`.

## Usage

```bash
one add [template-id] --name <project-name> [--deploy-provider <backend>] [options]
```

## Arguments

| Argument | Description |
|---|---|
| `template-id` | Template ID, such as `nestjs-api`. Omit it for interactive selection |
| `-n, --name` | Project name; required in non-interactive mode |
| `-y, --yes` | Non-interactive mode |
| `--deploy-provider <backend>` | Explicit deploy backend; must be in the template's compat list |
| `-o, --output <fmt>` | `json` / `yaml` / `text` |

The workspace root uses pnpm. Each project's toolchain comes from the template: Node templates use the workspace package manager, Go templates use the Go toolchain, and so on.

## Interactive Mode

Running `one add` with no arguments opens a terminal picker, which is best for first-time use or when you do not know the template ID yet. It asks for template category, template, project name, and, when a template supports multiple deploy backends, the deploy backend for that project.

Non-interactive calls should pass both template ID and project name:

```bash
one add nestjs-api --name api --yes
```

## Output

```json
{
  "schema": "one-cli/add/v1",
  "subproject_name": "user-api",
  "target_path": "/abs/path/my-app/services/user-api",
  "template_id": "nestjs-api",
  "toolchain": "node",
  "package_manager": "pnpm",
  "ai_guides": {
    "status": "completed",
    "providers": ["codex", "claude-code"],
    "generated_files": [
      "/abs/path/my-app/AGENTS.md",
      "/abs/path/my-app/CLAUDE.md"
    ],
    "file_count": 2
  }
}
```

`warnings[]` means a compatibility or post-sync step produced a non-blocking warning; the project was still added. `ai_guides.status` tells you whether root `AGENTS.md` / `CLAUDE.md` refreshed successfully. `ai_guides.generated_files` contains absolute paths under the workspace root.

## Examples

### Interactive

```bash
cd my-app
one add
```

This flow asks for:

1. Template category, such as Frontend / Backend / Library
2. Template ID, such as `nestjs-api`
3. Project name, such as `api`

Use this path when you are not sure which template ID to type.

### List Templates, Then Add Explicitly

```bash
one templates
one add nestjs-api --name api
```

The `id` shown by `one templates` is the first argument after `one add`.

### Non-interactive / CI / Agent

```bash
one add nestjs-api --name user-api --yes
one add nextjs-app --name web --yes
one add ts-library --name shared --yes
```

### Agent JSON Call

```bash
one add nestjs-api --name user-api --yes -o json | jq
```

## What Gets Synced

- Registers the project in `one.manifest.json#projects[]`
- Writes `projects[].domains.{container,deploy}` from template defaults
- Adds a `Dockerfile` for `container/docker` projects
- Adds `kustomize/base` and `kustomize/overlays/{dev,staging,prod}` for `deploy/kustomize`
- S3-compatible deploy projects do not write local deploy artifacts; deploy uses the object-storage profile configured by `one configure add deploy/aws-s3 --profile <name>` or another split S3 backend (`deploy/aliyun-oss`, `deploy/r2`, etc.)
- Adds GitHub Actions workflow entries
- Refreshes `AGENTS.md` / `CLAUDE.md`

If a non-critical step fails, such as AI guide refresh, the project still exists and the related status is marked `failed` or `skipped`.

## Common Errors

| Code | Recovery |
|---|---|
| `TEMPLATE_NOT_FOUND` | Template ID is wrong; read `available_templates` from error context and choose one |
| `TEMPLATE_REQUIRED` | No template ID was provided in a non-interactive context; pass one explicitly |
| `INVALID_NAME` | `--name` must match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$` |
| `SUBPROJECT_NAME_REQUIRED` | Non-interactive mode requires `--name` |
| `TARGET_EXISTS` | Project directory already exists; choose a different `--name` |
| `NOT_ONE_PROJECT` | cwd is not a workspace; run `one create <dir>` or `cd` into an existing workspace |
| `REGISTRY_FETCH_FAILED` | Network or registry issue; inspect the registry URL in context |
| `AI_GUIDE_EXISTS` | Root `AGENTS.md` / `CLAUDE.md` is user-managed and cannot be overwritten |

Full table: [Error codes](/en/docs/error-codes/).

## Template Choice

Not sure which one to use? Read the [template decision tree](/en/docs/templates/).

## After Adding

- Check `one.manifest.json#projects[]` to confirm registration
- AI guides and container / deploy artifacts are synced by `one add`
- `one add` does not install dependencies: JS / TS workspaces install from the root with the package manager; Go projects run `go mod download` in the project directory, then `go mod tidy` only after changing imports or when module metadata needs repair
