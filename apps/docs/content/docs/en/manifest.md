---
title: What is one.manifest.json
description: The workspace's project registry, environment configuration, and local development settings.
---

Every One CLI workspace has a `one.manifest.json` at its root. It records the workspace identity, projects, environments, and environment-variable source. Commands use it to find projects and select their toolchains.

## Example

```json
{
  "version": 1,
  "workspace": { "id": "demo-app-2bb61e", "name": "demo-app" },
  "environments": {
    "names": ["dev", "preview", "prod"],
    "default": "dev"
  },
  "domains": { "env": { "kind": "dotenv" } },
  "projects": [
    {
      "name": "web",
      "templateId": "react-spa",
      "relativeDir": "apps/web",
      "toolchain": "node",
      "buildVersion": "0.1.0",
      "packageManager": "pnpm",
      "domains": {
        "env": { "path": ".env", "inherits": true, "keys": ["API_URL"] },
        "dev": { "command": "pnpm dev" }
      }
    },
    {
      "name": "api",
      "templateId": "go-api",
      "relativeDir": "services/api",
      "toolchain": "go",
      "domains": { "dev": { "command": "go run ./cmd/server" } }
    }
  ]
}
```

The real file is strict JSON; comments and unknown fields are rejected.

## Fields

| Field | Meaning |
|---|---|
| `version` | Manifest schema version, currently `1` |
| `workspace` | Stable `id` and display `name` |
| `environments` | Environment names and default environment |
| `domains.env` | Workspace environment source: `dotenv` or `infisical`, with optional backend-specific `config` |
| `projects[]` | Project names, paths, templates, toolchains, optional `packageManager` and `buildVersion` |
| `projects[].domains.env` | Environment overrides: `path`, `inherits`, `disabled`, and declared key names; inherits the workspace backend |
| `projects[].domains.dev` | The `command` executed by `one dev` |

For Infisical, `domains.env.config` can contain `projectId`, `projectName`, `rootPath`, and `keys`. Key values and local Profile names never belong in the manifest. Keep credentials in machine-local Profiles and values in dotenv or Infisical.

## Who writes it

| Action | Change |
|---|---|
| `one create` | Writes workspace identity, default environments, environment source, and an empty project list |
| `one add` | Registers a project and its development command |
| `one env set` | Records declared key names; Infisical can initialize its project binding |
| `one env switch` | Changes the workspace environment source |
| `one serve` | Applies explicitly reviewed project or environment-source changes with revision checks |

`one build` selects each project's build command from its toolchain. Node projects use package scripts; Go projects use `Taskfile.yml`. Workspace tasks and the ordinary `ci` aggregate run through `one run`.

## Manual edits

Keep paths and names consistent when renaming or removing projects. Update `projects[].domains.dev.command` if the project's development script changes. The workspace layout remains `apps/`, `services/`, and `packages/`.

If the manifest and filesystem disagree, inspect the declared paths and restore the missing project files or fix the registry entry. Do not put business values, dependencies, caches, or build outputs in the manifest.

## Removed deployment configuration

The `deploy` and `container` domains have been removed. A manifest containing those fields returns `MANIFEST_INVALID` with a removal hint. Delete the corresponding fields manually; no migration or cleanup of existing Dockerfiles, platform configuration, or CI files is performed.

Invalid JSON or unknown fields also produce `MANIFEST_INVALID`. See [error codes](/en/docs/error-codes/).
