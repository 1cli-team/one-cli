---
title: What is one.manifest.json
description: "The workspace ledger: who writes it, when to edit it, and what drift means."
---

Every One CLI workspace root has a `one.manifest.json`. This page explains what it does, who should edit it, and when you should leave it alone.

**For**: anyone seeing the manifest for the first time, and anyone trying to understand where workspace state comes from.

**You will learn**: the manifest is the workspace **source of truth**, and when you, versus One CLI, should modify it.

## One Sentence Definition

`one.manifest.json` is the workspace **ledger**. It records which projects exist, which templates created them, which backend each domain uses (`env`, `deploy`, `container`), and which environments exist.

Its presence means "this is a One CLI workspace". Other commands use it to decide whether the current directory belongs to a workspace.

## What It Stores

The block below uses `jsonc` so the fields can be explained inline. The real `one.manifest.json` file is still strict JSON; do not paste the `//` comments into the actual file.

```jsonc
{
  "version": 1, // Manifest schema version written by One CLI
  "workspace": { // Workspace identity
    "id": "demo-app-2bb61e", // Stable workspace ID generated at creation time
    "name": "demo-app" // Workspace name, usually from the directory or --name
  },
  "environments": { // Environments known to the workspace
    "names": ["dev", "staging", "prod"], // Available environment names
    "default": "dev" // Default environment when --env is omitted
  },
  "domains": { // Workspace-level domain defaults
    "env": { // Secrets / environment variable backend
      "kind": "infisical", // Env backend: dotenv or infisical
      "config": { // Backend-specific env config
        "keys": ["VITE_API_URL", "VITE_PUBLIC_SITE"], // Declared workspace-level key names; values never enter the manifest
        "projectId": "86c73b57-5d1b-4f99-90dc-5d0c8ee0e823", // Infisical project ID
        "projectName": "demo-app", // Project name inside Infisical
        "rootPath": "/" // Root path inside Infisical
      }
    },
    "deploy": { // Workspace-level deploy default
      "kind": "kustomize", // Default deploy backend
      "config": { // Kustomize backend config
        "namespace": "demo-app-2bb61e", // Optional Kubernetes namespace; defaults to workspace.id when omitted
        "kustomizationPath": "kustomize/overlays/prod" // Kustomize overlay directory used by one deploy render/apply
      }
    }
  },
  "projects": [ // Project registry for this workspace
    {
      "name": "web", // Project name used by one env / one deploy -p
      "templateId": "nextjs-app", // Template ID used to create this project
      "relativeDir": "apps/web", // Project path relative to the workspace root
      "toolchain": "node", // Project toolchain: node, go, etc.
      "buildVersion": "0.1.0", // Default build version read by container / deploy commands
      "packageManager": "pnpm", // Package manager for Node projects
      "domains": { // Project-level domain overrides
        "container": {}, // Empty object enables container builds and inherits profile defaults
        "deploy": { "kind": "kustomize" }, // This project deploys with kustomize
        "dev": { "command": "pnpm run dev" } // Command used by one dev
      }
    },
    {
      "name": "spa", // Second project: static frontend app
      "templateId": "react-spa", // React SPA template
      "relativeDir": "apps/spa", // Frontend project directory
      "toolchain": "node", // Node toolchain
      "buildVersion": "0.1.0", // Current default build version
      "packageManager": "pnpm", // Install dependencies with pnpm install
      "domains": { // Only override what differs from workspace defaults
        "deploy": {
          "kind": "rustfs", // This project uses object storage instead of workspace kustomize
          "config": { "bucket": "demo-app-2bb61e" } // Object storage bucket; often defaults to workspace.id
        },
        "dev": { "command": "pnpm run dev" } // Command used by one dev
      }
    },
    {
      "name": "api", // Backend service
      "templateId": "go-api", // Go API template
      "relativeDir": "services/api", // Go service directory
      "toolchain": "go", // Go toolchain
      "buildVersion": "0.1.0", // Current default build version
      "domains": {
        "container": {}, // Enable container builds
        "deploy": { "kind": "kustomize" }, // This service deploys to Kubernetes
        "dev": { "command": "go run ./cmd/server" } // Go dev command used by one dev
      }
    }
  ]
}
```

Important fields:

| Field | Meaning |
|---|---|
| `version` | Manifest schema version written and read by One CLI |
| `workspace` | Workspace identity (`id` + `name`). Infisical auto-binding uses `name` when creating an Infisical project |
| `environments` | Environment names and default env. Shared by secrets, `one deploy --env`, and `projects[].domains.deploy.config.env` |
| `domains.env` / `.deploy` / `.container` | Workspace-level backend choice: `{kind, config}`, present only when needed. `kind` is the bare backend id; `config` is backend-specific JSON |
| `projects[]` | The core project registry. Each project has `buildVersion` and optional domain overrides |
| `projects[].domains.env` | Project-level env override (`path`, `inherits`, `disabled`, `keys`); no `kind`, because it inherits the workspace backend |
| `projects[].domains.container` | Project-level container override (`kind`, `image`, `namespace`). An empty object enables container builds and uses local profile resolution |
| `projects[].domains.deploy` | Project-level deploy backend. This one has `kind` because deploy can vary by project: web -> Vercel, API -> Kustomize |
| `projects[].domains.dev` | Project-level development command. `one dev` reads `command`, such as `pnpm run dev` for Node projects or `go run ./cmd/server` for Go projects |

`domains.deploy.config.namespace` is an optional override. When it is omitted, the Kubernetes namespace defaults to `workspace.id`, such as `demo-app-2bb61e`. Set it explicitly only when you want a fixed shared namespace or an environment-shaped name such as `demo-app-prod`.

> Design note: both workspace and project settings use `domains`. Env only carries project-level overrides. Deploy carries `kind` at project scope because different projects can use different deploy backends. Container can be an empty object, or carry overrides such as image / profile / namespace / kind.

## Who Writes It

| Command | Manifest change |
|---|---|
| `one create` | Creates the initial manifest with workspace identity, empty `projects`, `domains.env.kind`, `environments`, and always-on CI / dev conventions |
| `one add` | Adds an item to `projects[]`, writes template defaults into `projects[].domains.<name>`, writes `projects[].domains.dev.command`, and syncs infra files |
| `one env set` | Records key names in env config; values never go into the manifest. Infisical can lazy-bind if not already bound |
| `one container build` | Writes back `projects[i].domains.container.image` and sometimes `domains.container.config.platform` |
| `one deploy --env <name>` | Does **not** write the manifest; it only passes `--env` to the current deploy call |
| **You** | Rarely; see below |

## When To Edit It Manually

Most of the time you should not touch it. The few manual cases:

<!-- verify-cli:ignore-start -->
1. **Rename a project**: there is no `one rename` yet. Change `projects[].name` and rename the directory so the manifest and filesystem match.
2. **Remove a project**: there is no `one remove` yet. Delete the item from `projects[]` and remove the directory.
<!-- verify-cli:ignore-end -->
3. **Switch deploy backend**: for example, move `web` from `aws-s3` to `vercel` by editing `projects[i].domains.deploy.kind`, then run `one configure use deploy/vercel --profile <name>`.
4. **Adjust a local dev command**: for example, change `projects[i].domains.dev.command` from `pnpm run dev` to the project's own script. `one dev` trusts the manifest and does not auto-track later `package.json` changes.

Fields One CLI owns:

- `workspace.roots`: fixed to `apps/services/packages`
- `ai.providers`: all supported providers are enabled by default; current output includes `AGENTS.md` and `CLAUDE.md`
- Top-level `ci` / `dev`: always enabled; these fields are ignored. Project-level `projects[].domains.dev.command` is still read by `one dev`

## What Drift Means

Drift means the manifest and filesystem disagree. Common cases:

- You manually deleted `services/user-api/` but forgot to edit the manifest.
- You pulled a teammate's branch and the filesystem / manifest are out of sync.
- Template rendering failed after the project was registered.

Run the relevant per-domain command again, such as `one add`, `one container build`, or `one deploy render`. Those commands read the manifest and report what is missing.

## What Does Not Belong In The Manifest

Do not put these in `one.manifest.json`:

- Business runtime config, such as database URLs or API URLs -> use `.env` and secrets
- Project dependencies and scripts -> use that project's `package.json`, `Taskfile.yml`, or framework config
- User preferences, such as editor settings
- Temporary state, build outputs, or caches

## Validation

`one.manifest.json` is schema-validated. Invalid JSON or shape surfaces `MANIFEST_INVALID`; a missing or empty manifest surfaces `MANIFEST_MISSING_OR_EMPTY`, with remediation pointing to `one create` / `one add`.

See the `MANIFEST_*` section in [Error codes](/en/docs/error-codes/).

## Example

Run `one create my-app && cat my-app/one.manifest.json` to see a new workspace manifest with workspace identity, the default environment set, the env backend selection, and an empty `projects` array. Then add one project:

```bash
cd my-app
one add nestjs-api --name api
cat one.manifest.json
```

`projects[]` gains one item, and `projects[0].domains.{container,deploy}` is filled from the `nestjs-api` template defaults.
