---
title: one.manifest.toml
description: The complete Manifest v2 structure for workspace identity, projects, and Infisical bindings.
---

`one.manifest.toml` is the workspace's configuration file. Manifest v2 uses TOML and names each project with a table key. One CLI reads this file only; it does not load the former JSON format or provide a migration command.

## Complete structure

```toml
version = 2

[workspace]
id = "my-workspace"
name = "My Workspace"

[env.infisical]
siteUrl = "https://app.infisical.com"
projectId = "your-project-id"
environments = ["dev", "staging", "prod"]

[projects.web]
path = "apps/web"
toolchain = "node"
template = "react-spa"

[projects.api]
path = "services/api"
toolchain = "go"
template = "go-api"
```

A fresh workspace contains its version and identity. Adding a project adds its table; binding Infisical adds `[env.infisical]`. Generated files contain no comments. You can add your own TOML comments: subsequent edits preserve comments, table order, and unchanged text.

## Fields

| Field | Meaning |
|---|---|
| `version` | Required schema version: `2` |
| `workspace.id` | Stable workspace identifier |
| `workspace.name` | Display name; also the initial name when creating remote storage |
| `env.infisical.siteUrl` | Optional Infisical instance URL; defaults to `https://app.infisical.com` |
| `env.infisical.projectId` | Required remote project ID when a binding is present |
| `env.infisical.environments` | Required list of unique remote environment slugs when bound; must contain `dev` |
| `projects.<name>` | Project name as a table key; unique within the workspace |
| `projects.<name>.path` | Required, unique, normalized workspace-relative directory |
| `projects.<name>.toolchain` | Required: `node`, `go`, or `none` |
| `projects.<name>.template` | Optional template ID used to create the project |

Project paths cannot be absolute or escape the workspace. Unknown fields, unsupported versions, and invalid field values produce `MANIFEST_INVALID`. TOML syntax errors include the file path and source location.

## Environment conventions

Omit `[env.infisical]` to run with the existing process environment. The manifest never contains variable values or a variable-name registry.

The default environment is always `dev`, regardless of the order of `environments`. Use `--env staging` or another declared slug to select a different environment. Slugs match Infisical identifiers, such as `dev`, `staging`, and `prod`; the UI may display Development, Staging, and Production. Listing a slug here does not create that environment remotely.

The shared remote folder is `/`. A project with `path = "services/api"` receives variables from `/`, `/services`, and `/services/api`, with the closest folder taking precedence. Each parallel task receives its own project's values. These folder and inheritance rules are conventions, with no per-project overrides.

## Configuration ownership

- `mise.toml` owns tasks, dependencies, tools, and caching. `one run dev` or the `one dev` task shortcut works when mise defines that task.
- `package.json` owns package-manager metadata; project versions stay in their native files.
- Dashboard discovers service URLs from process output.
- Personal One CLI preferences select stream or TUI output.

`one env set` stores values in Infisical. It persists a binding on first use and can register an environment locally after a successful write, but never writes variable names to the manifest. Dashboard previews the actual TOML before publishing binding changes and checks the file revision to reject stale drafts. Project settings display the conventions without per-project environment controls.

See [environment variables](/en/docs/env-vars/) for commands and [adding projects](/en/docs/add/) for project registration.
