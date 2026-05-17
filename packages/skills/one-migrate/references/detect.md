# Stack Detection — Mapping Existing Code to a `templateId`

The authoritative template list is `one templates -o json`; run it
once and treat the IDs you see there as the only legal values for
`manifest.projects[].templateId`. This file is the fingerprint table
the agent uses to **propose** a match. Always show the user what was
picked and why — never silently lock in a guess.

## Detection workflow

For each project directory you're about to migrate:

1. `ls <project-dir>` to see top-level files.
2. If `package.json` exists, read it: capture `dependencies`,
   `devDependencies`, `scripts`, `main`, `exports`.
3. Walk the table below top-to-bottom; **first match wins** (more
   specific patterns are higher up).
4. If nothing matches, record `templateId: ""` and `toolchain` based
   on the build tool (`go.mod` → `go`, `package.json` → `node`).
   Tell the user the migration will still produce a valid manifest,
   just without the template-driven niceties (no auto Dockerfile,
   no per-project CI workflow).

## Fingerprint table

| Signal in project dir | → `templateId` | `toolchain` | Default `relativeDir` root |
|---|---|---|---|
| `astro.config.{mjs,ts,js}` **AND** `@astrojs/starlight` in deps | `starlight-docs` | `node` | `apps/` |
| `astro.config.{mjs,ts,js}` (no Starlight) | `astro-site` | `node` | `apps/` |
| `next.config.{js,ts,mjs}` **OR** `"next"` in dependencies | `nextjs-app` | `node` | `apps/` |
| `nest-cli.json` **OR** `@nestjs/core` in dependencies | `nestjs-api` | `node` | `services/` |
| `"expo"` in dependencies **OR** `app.json` with `expo` key | `expo-mobile` | `node` | `apps/` |
| `"electron"` or `"electron-forge"` in deps | `electron-app` | `node` | `apps/` |
| `vite.config.{ts,js,mjs}` **AND** React in deps (no Next.js) | `react-spa` | `node` | `apps/` |
| `go.mod` present | `go-api` | `go` | `services/` |
| `package.json` with `main` / `exports` **AND** no framework above | `ts-library` | `node` | `packages/` |
| None of the above | `""` (empty) | infer from build tool | ask user |

## Why these priorities

- **Starlight before plain Astro**: Starlight is a strict superset;
  always detect the more specific case first.
- **Next.js before generic React**: Next ships its own React, so a
  React dep alone is not enough to pick `react-spa` if `next` is
  also present.
- **Expo before plain React Native**: Expo is the supported path;
  bare RN projects fall to `templateId: ""` (we don't have a template).
- **Electron is its own category**: never collapse it into `react-spa`
  even if the renderer uses React + Vite.
- **`go-api` for any `go.mod`**: One CLI's Go template is the only Go
  toolchain it supports; if the project's actually a CLI / library
  rather than an API, that's fine — `templateId` describes the
  scaffold archetype, not the runtime profile.

## Sub-project iteration in a monorepo

For each candidate path under the existing repo (e.g. `apps/web`,
`packages/ui`, `services/billing`), run the fingerprint table once.
Record the result in a small per-project plan that maps to the final
`projects[]` entry:

```text
existing path          → relativeDir              templateId      toolchain
apps/web               → apps/web                 nextjs-app      node
apps/admin             → apps/admin               react-spa       node
packages/ui            → packages/ui              ts-library      node
services/api           → services/api             nestjs-api      node
infra/scripts          → (skip — not a project)   —               —
```

Skip directories that aren't actually deployable units (build
scripts, terraform configs, fixtures). The manifest is for code that
`one dev` / `one container build` / `one deploy` would touch.

## Confirming with the user

When you've finished the table, repeat the mapping back to the user
**before** moving any files or writing the manifest:

> 我打算按这个映射迁移：
> - `apps/web` (Next.js) → `apps/web`, templateId=`nextjs-app`
> - `services/api` (NestJS) → `services/api`, templateId=`nestjs-api`
> - `packages/ui` (TS lib) → `packages/ui`, templateId=`ts-library`
>
> 看起来对吗？有要改的地方就说，没有的话我开始执行。

Getting confirmation is cheap; un-doing a misclassified mass-move
isn't.
