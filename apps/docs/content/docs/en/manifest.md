---
title: What is one.manifest.json
description: The workspace's project registry, environment configuration, and local development settings.
---

Every One CLI workspace has a `one.manifest.json` at its root. It records the workspace identity, projects, environments, and environment-variable source. Commands use it to find projects and select their toolchains.

## Example

```json
{
  "version": 1,
  "workspace": {
    "id": "demo-app-2bb61e",
    "name": "demo-app"
  },
  "environments": {
    "names": [
      "dev",
      "preview",
      "prod"
    ],
    "default": "dev"
  },
  "projects": [
    {
      "name": "web",
      "templateId": "react-spa",
      "relativeDir": "apps/web",
      "toolchain": "node",
      "buildVersion": "0.1.0",
      "packageManager": "pnpm",
      "env": {
        "path": "/apps/web",
        "inherits": true,
        "keys": [
          "API_URL"
        ]
      },
      "dev": {
        "command": "pnpm dev"
      }
    },
    {
      "name": "api",
      "templateId": "go-api",
      "relativeDir": "services/api",
      "toolchain": "go",
      "dev": {
        "command": "go run ./cmd/server"
      }
    }
  ],
  "env": {
    "siteUrl": "https://app.infisical.com",
    "projectId": "your-project-id",
    "rootPath": "/"
  }
}
```

The real file is strict JSON; comments and unknown fields are rejected.

## Fields

| Field | Meaning |
|---|---|
| `version` | Manifest schema version, currently `1` |
| `workspace` | Stable `id` and display `name` |
| `environments` | Environment names and default environment |
| `env` | Optional Infisical binding: `siteUrl`, `projectId`, `projectName`, `rootPath`, and `keys` |
| `projects[]` | Project names, paths, templates, toolchains, optional `packageManager` and `buildVersion` |
| `projects[].env` | Environment overrides: `path`, `inherits`, `disabled`, and declared key names; inherits the workspace backend |
| `projects[].dev` | The `command` executed by `one dev` |

For Infisical, `env` can contain `projectId`, `projectName`, `rootPath`, and `keys`. Key values and local Profile names never belong in the manifest. Keep credentials in the system keyring and values in Infisical.

## Who writes it

| Action | Change |
|---|---|
| `one create` | Writes workspace identity, default environments, and an empty project list; leaves `env` unset |
| `one add` | Registers a project and its development command |
| `one env set` | Records declared key names; Infisical can initialize its project binding |
| `one serve` | Applies explicitly reviewed project or environment-source changes with revision checks |

`one build` selects each project's build command from its toolchain. Node projects use package scripts; Go projects use `Taskfile.yml`. Workspace tasks and the ordinary `ci` aggregate run through `one run`.

## Manual edits

Keep paths and names consistent when renaming or removing projects. Update `projects[].dev.command` if the project's development script changes. The workspace layout remains `apps/`, `services/`, and `packages/`.

If the manifest and filesystem disagree, inspect the declared paths and restore the missing project files or fix the registry entry. Do not put business values, dependencies, caches, or build outputs in the manifest.

## Removed deployment configuration

The `deploy` and `container` domains have been removed. A manifest containing those fields returns `MANIFEST_INVALID` with a removal hint. Delete the corresponding fields manually; no migration or cleanup of existing Dockerfiles, platform configuration, or CI files is performed.

Invalid JSON or unknown fields also produce `MANIFEST_INVALID`. See [error codes](/en/docs/error-codes/).

## Migration from domains

The `domains` wrapper is no longer accepted. Move an Infisical workspace binding from `domains.env.config` to top-level `env`, and move project `domains.env` / `domains.dev` to `env` / `dev`. Remove `kind` and `config` wrappers. For a former dotenv workspace, remove the old binding and any local-file `path` overrides; bind Infisical when needed.

Old manifests return `MANIFEST_INVALID` with a migration hint. One CLI does not rewrite manifests or import/delete existing `.env` files. Move required values to Infisical explicitly. The env `pull` and `switch` subcommands and the `--env-provider` flag have been removed. Legacy preset code `d` remains reserved and is rejected; use `i` or omit the environment segment.
