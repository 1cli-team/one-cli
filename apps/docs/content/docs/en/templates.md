---
title: How to Choose Templates
description: Decision tree for the 13 built-in templates. Pick the right one in 30 seconds.
---

If you are adding a new project and do not know which base template to pick, this page gives you a decision tree and a comparison table.

Use the [template catalog](/en/templates/) to filter by type and compare purpose and stack.

**For**: people who ran `one templates` and saw too many IDs, tech leads evaluating stack choices, and anyone writing template-selection rules for agents.

**You will learn**: how to pick the right base template in 30 seconds, then add projects with one add.

## 30-second Rule

```text
Need a backend API -----------------------> nestjs-api / go-api
Need a browser-facing web project --------> nextjs-app / react-spa / astro-site
Need a reusable package ------------------> ts-library / go-lib
Need a documentation site ----------------> starlight-docs
Need a mobile app ------------------------> expo-mobile
Need a desktop app -----------------------> electron-app
```

If unsure, ask one question: **how does the user consume this thing?** Browser -> Web. Command-line / HTTP calls -> API. `npm install` / `go get` -> Library. `.app` / `.dmg` / `.exe` -> Desktop. App Store -> Mobile. Reading content -> Docs.

## Full Comparison

| ID | Category | Keywords | One-line fit | Details |
|---|---|---|---|---|
| `nestjs-api` | API | TypeScript, NestJS, REST | Default API template for TypeScript teams | [View template](/en/templates/#nestjs-api) |
| `go-api` | API | Go, Gin, GORM | High-throughput / low-memory / mixed-language teams | - |
| `nextjs-app` | Web | Next.js, SSR, React | Default consumer web or full-stack app | [View template](/en/templates/#nextjs-app) |
| `react-spa` | Web | Vite, React, SPA | Console / internal app / no SEO | [View template](/en/templates/#react-spa) |
| `astro-site` | Web | Astro, static-first | Marketing or content site | [View template](/en/templates/#astro-site) |
| `starlight-docs` | Docs | Starlight, Astro | Documentation site or knowledge base | [View template](/en/templates/#starlight-docs) |
| `expo-mobile` | Mobile | Expo, React Native | Cross-platform iOS + Android | [View template](/en/templates/#expo-mobile) |
| `electron-app` | Desktop | Electron, React, Vite | Desktop app for macOS / Windows / Linux | [View template](/en/templates/#electron-app) |
| `ts-library` | Library | TS, strict semver | Reusable TypeScript package | - |
| `go-lib` | Library | Go, module, package layout | Reusable Go module | - |
| `empty-app` | App | No stack | Start with an empty application directory | - |
| `empty-service` | API | No stack | Start with an empty service directory | - |
| `empty-library` | Library | No stack | Start with an empty shared library directory | - |

## Add The Template You Chose

The `ID` column is the first argument after `one add`.

If you are still unsure, use the interactive flow:

```bash
one add
```

If you already picked a template:

```bash
one templates
one add nestjs-api --name api
```

`nestjs-api` comes from the template ID. `api` is the project name you choose.

## Recommended Combos

### Full-stack SaaS (default)

```bash
one create my-saas
cd my-saas
one add nestjs-api --name api
one add nextjs-app --name web
one add ts-library --name shared
```

Why: TypeScript across the stack lets `shared` be used by both API and web. Next.js can cover SEO and authenticated app surfaces.

### High-performance Backend + Static Marketing

```bash
one add go-api --name api
one add astro-site --name marketing
one add react-spa --name console
```

Why: Go handles traffic; Astro keeps the public site fast and SEO-friendly; React SPA works for an authenticated console.

### Mobile + API

```bash
one add nestjs-api --name api
one add expo-mobile --name app
one add ts-library --name shared
```

`shared` can hold DTOs and business types reused by React Native and the API.

## Still Unsure?

Run `one templates -o json` for full template metadata, or run `one add` interactively. The picker includes category and one-line descriptions.

You can also pick one of the recommended combos above, get it running, and change course once you know more.

## Template dependencies and Electron workspaces

Node templates do not copy pre-generated lockfiles. `one dev` creates or updates the
repository's root lockfile when needed; commit it to Git. `one build` validates an
existing lockfile without rewriting it.

`electron-app` requires a pnpm workspace. It remains one One project, while its main,
UI, and preload packages join the root `pnpm-workspace.yaml` and share the root lockfile.
Package names use the project name as their scope, for example `@desktop/electron`,
`@desktop/ui`, and `@desktop/preload`, so multiple desktop apps can coexist.

The template follows the root package-manager version, registry, and mirror settings.
Existing build-script policies are preserved; configurations without a policy receive
the bundled template defaults. Explicit denials of Electron's installation script
must be adjusted at the root. For concurrent desktop development, set a different
`ELECTRON_RENDERER_PORT` in each project's environment.

These rules apply to newly generated projects. Existing Electron projects are not
automatically rewritten or migrated.


## Develop directly in a template directory

Bundled templates contain ordinary source files that you can run and debug in
`packages/templates/<id>`. Go templates have a local `go.work` to isolate them
from the repository workspace. Electron has a `pnpm-workspace.yaml` for template
development. These development files are excluded from generated projects,
which use the destination One workspace's configuration.

For example, inside the One CLI source repository:

```sh
cd packages/templates/go-api
go test ./...
go run ./cmd/server
```

The Go API defaults to in-memory SQLite. Configure environment variables as
described by the template when using PostgreSQL or other runtime settings.

```sh
cd packages/templates/electron-app
pnpm install
pnpm run dev
```

Electron builds preload first, then starts the main process and Vite UI. The
host still needs Electron's graphical environment and system sandbox support.
Use `pnpm run build` for a build without launching the application.

For a single-package Node template such as React, isolate the parent workspace:

```sh
cd packages/templates/react-spa
pnpm --ignore-workspace install
pnpm --ignore-workspace run dev
```

Template development uses the pnpm version in its `package.json`. Generated
projects inherit the destination workspace's version. Local dependencies,
lockfiles and build artifacts are excluded from the CLI bundle.

## Configure template generation

Files are copied verbatim by default. Only starters needing parameterization
include a `template.json` file, with these supported settings:

| Setting | Purpose |
| --- | --- |
| `schemaVersion: 1` | Declare the descriptor version |
| `go.modulePrefix` | Set the generated module path and rewrite its Go imports |
| `node.scope`, `node.sourceFiles` | Rename internal Node packages, dependency keys, scripts and the scope in listed source files |
| `text` | Replace example text in explicitly listed files |
| `exclude` | Exclude files or directories used only for template development |

Each `text` rule contains `files`, `from` and `value`. The supported values are
`projectName` and `projectNameKebabCase`. Optional `minMatches` defaults to 1;
every listed file must meet that count. Paths are exact, template-relative paths.
No scripts or expressions are executed. Unknown fields, missing files,
insufficient matches and overlapping replacements fail before destination writes.

Go templates maintain one normal `go.mod` and a `go.sum` when needed. Bundling
temporarily renames `go.mod` to `_go.mod` to avoid Go's nested-module embedding
restriction; generation restores the filename. Do not edit the generated
resources in `packages/cli/internal/resources/bundled/` manually.

After editing, run these commands from the repository root:

```sh
mise run sync-bundled
mise run check
mise run check:templates
```

`check:templates` installs the Electron template dependencies, builds the Go and
Electron sources, checks generated formatting for every Node template, and
builds two differently named Electron projects plus two Go projects in a
temporary workspace. It requires network access and the corresponding
toolchains, and does not change the developer's global language preference.
