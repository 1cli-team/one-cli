---
title: How to Choose Templates
description: Decision tree for the 13 built-in templates. Pick the right one in 30 seconds.
---

If you are adding a new project and do not know which base template to pick, this page gives you a decision tree and a comparison table.

Run [`one templates`](/en/docs/templates-cmd/) to list the templates in your installed CLI. The table below compares their purpose and stack.

**For**: people who ran `one templates` and saw too many IDs, tech leads evaluating stack choices, and anyone writing template-selection rules for agents.

**You will learn**: how to pick the right base template in 30 seconds, then add projects with one add.

## 30-second Rule

```text
Need a backend API -----------------------> nestjs-api / go-api
Need a browser-facing web project --------> nextjs-app / react-spa / nextjs-site
Need a reusable package ------------------> ts-library / go-lib
Need a documentation site ----------------> fumadocs-docs
Need a mobile app ------------------------> expo-mobile
Need a desktop app -----------------------> electron-app
```

If unsure, ask one question: **how does the user consume this thing?** Browser -> Web. Command-line / HTTP calls -> API. `npm install` / `go get` -> Library. `.app` / `.dmg` / `.exe` -> Desktop. App Store -> Mobile. Reading content -> Docs.

## Full Comparison

| ID              | Category | Keywords                      | One-line fit                                        | Details                                                                                     |
| --------------- | -------- | ----------------------------- | --------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `nestjs-api`    | API      | TypeScript, NestJS, REST      | Default API template for TypeScript teams           | [Source](https://github.com/1cli-team/one-cli/tree/master/packages/templates/nestjs-api)    |
| `go-api`        | API      | Go, Gin, GORM                 | High-throughput / low-memory / mixed-language teams | -                                                                                           |
| `nextjs-app`    | Web      | Next.js, SSR, React           | Default consumer web or full-stack app              | [Source](https://github.com/1cli-team/one-cli/tree/master/packages/templates/nextjs-app)    |
| `react-spa`     | Web      | Vite, React, SPA              | Console / internal app / no SEO                     | [Source](https://github.com/1cli-team/one-cli/tree/master/packages/templates/react-spa)     |
| `nextjs-site`   | Web      | Next.js, React, static export | Marketing or content site                           | [Source](https://github.com/1cli-team/one-cli/tree/master/packages/templates/nextjs-site)   |
| `fumadocs-docs` | Docs     | Fumadocs, Next.js, Base UI    | Documentation site or knowledge base                | [Source](https://github.com/1cli-team/one-cli/tree/master/packages/templates/fumadocs-docs) |
| `expo-mobile`   | Mobile   | Expo, React Native            | Cross-platform iOS + Android                        | [Source](https://github.com/1cli-team/one-cli/tree/master/packages/templates/expo-mobile)   |
| `electron-app`  | Desktop  | Electron, React, Vite         | Desktop app for macOS / Windows / Linux             | [Source](https://github.com/1cli-team/one-cli/tree/master/packages/templates/electron-app)  |
| `ts-library`    | Library  | TS, strict semver             | Reusable TypeScript package                         | -                                                                                           |
| `go-lib`        | Library  | Go, module, package layout    | Reusable Go module                                  | -                                                                                           |
| `empty-app`     | App      | No stack                      | Start with an empty application directory           | -                                                                                           |
| `empty-service` | API      | No stack                      | Start with an empty service directory               | -                                                                                           |
| `empty-library` | Library  | No stack                      | Start with an empty shared library directory        | -                                                                                           |

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
one add nextjs-site --name marketing
one add react-spa --name console
```

Why: Go handles traffic; Next.js static export serves the public site with SEO metadata; React SPA works for an authenticated console.

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

## Static website and documentation templates

`nextjs-site` uses Next.js, React, Tailwind CSS, and shadcn Base UI and exports `out/`.
`fumadocs-docs` uses the Base UI implementation of Next.js + Fumadocs with English/Chinese MDX,
sidebar navigation, a table of contents, syntax highlighting, and browser-side static search.
Deploy its `out/` directory without a Node server.

The retired IDs `astro-site` and `starlight-docs` now provide replacement guidance.
Use `nextjs-site` and `fumadocs-docs` for new projects. Existing project directories,
dependencies, and tasks stay as they are.

## Template dependencies and Electron workspaces

Node templates do not copy pre-generated lockfiles. Tasks such as `one dev` and
`one build` share dependency preparation: matching installations are reused, and
installation may create or update the root lockfile. Review and commit those changes.
For strict lockfile validation in CI, explicitly run `one mise exec -- pnpm install --frozen-lockfile`.

`electron-app` creates three top-level One projects. With `--name desktop`, they are
`apps/desktop-renderer`, `services/desktop-main`, and `packages/desktop-preload`.
Their npm package names are `desktop-renderer`, `desktop-main`, and `desktop-preload`.
Directories preserve the supplied project name; npm names use kebab-case. All three
join the root pnpm workspace and share its lockfile, without a nested workspace.

The manifest records `[groups.desktop]` with these three members. Use
`one run desktop:dev`, `one run desktop:build`, and
`one run desktop:pack` to operate on the group. These aliases survive `one init mise`.
Each member remains independently addressable, such as `one build -p desktop-main`.
The main process uses Awilix function factories; the renderer uses React, Vite,
and shadcn/ui backed by Base UI. Packaging copies renderer and preload outputs
into the main project's build directory.
The template follows the root package-manager version, registry, and mirror settings.
Existing build-script policies are preserved; configurations without a policy receive
the bundled template defaults. Explicit denials of Electron's installation script
must be adjusted at the root. For concurrent desktop development, set a different
`ELECTRON_RENDERER_PORT` for both the renderer and main projects of each desktop app.

Electron development checks sandbox permissions only when the OS identifies as Ubuntu
(`ID=ubuntu`). On the first AppArmor user-namespace denial, it generates a profile for
the installed Electron binary and prints commands for an administrator to review,
install, and load it. Rerun development after setup; the script never elevates itself.
A changed Electron binary path triggers another check. Linux preserves explicit displays,
including SSH X11 forwarding and Xvfb; otherwise it discovers the current user's live
Wayland / X11 desktop. Ambiguous displays require an explicit selection; headless systems
receive guidance to use a graphical terminal, X11 forwarding, or Xvfb. macOS and Windows
skip these checks. See the generated README for details.

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
task check
go run ./cmd/server
```

NestJS API and Go API start without database connections, JWT, login, or user CRUD. Drizzle ORM/Kit and Gorm are retained. Choose a database, install its driver, and explicitly add connections and migrations.

Application templates retain Axios for HTTP, SWR for remote data and caching, and Zustand for shared UI state. Expo keeps MMKV and native development clients with React Native styles. Electron keeps Awilix function factories and an app-information-only preload API. Add business features through manually installed business skills or application code.

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

| Setting                          | Purpose                                                                                                            |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `schemaVersion: 1`               | Declare the descriptor version                                                                                     |
| `go.modulePrefix`                | Set the generated module path and rewrite its Go imports                                                           |
| `node.scope`, `node.sourceFiles` | Rename internal Node packages, dependency keys, scripts and the scope in listed source files                       |
| `text`                           | Replace example text in explicitly listed files                                                                    |
| `exclude`                        | Exclude files or directories used only for template development                                                    |
| `projects`                       | Composite components, each declaring `source`, name `suffix`, and `category` (`frontend`, `backend`, or `library`) |
| `sharedFiles`                    | Template-relative files copied into every composite component                                                      |

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
```
