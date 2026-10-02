---
title: one skills
description: Manage agent skills through the upstream Skills CLI and understand default template skills.
---

`one skills` is a thin wrapper around `npx skills@1.7.0`. It passes every argument unchanged in the current directory, preserves interactive input, stdout/stderr and exit codes, and prepares Node through mise when npx is unavailable. It does not load One/Infisical credentials or add Node projects to the workspace.

```bash
one skills --help
one skills find react
one skills add owner/repo --skill skill-name
one skills list
one skills update
one skills remove skill-name
```

All subcommands and flags belong to [Skills CLI](https://github.com/vercel-labs/skills). One does not add a `sync` command or reinterpret `--agent`, `--global`, `--yes`, or future upstream flags. `one skills --help` shows upstream help; `one help skills` describes the wrapper. Native output is not wrapped in One JSON. The npm bootstrap prompt is accepted by npx; Skills CLI's own confirmations remain unchanged unless you pass `--yes`.

## Automatic development skills

`one create` installs `one-cli` and `find-skills` by default, providing One usage and skill discovery guidance. `one add` installs missing common and stack skills for the selected template after all project files and workspace configuration are generated. Electron installs the union once after all three member projects are ready. Automatic installations go into `.agents/skills` under the workspace root (the shared directory read by Codex, Cursor and other compatible agents), never the user's global skill directory, and show only One's concise progress.

Use `--skip-skills` on either command to skip installation. Dashboard's project dialog offers the same choice. Default installation selects named skills and the universal agent explicitly, copies the files, and runs non-interactively; it does not install every skill in a source repository.

```bash
one create demo --skip-skills
one add react-spa --name web --skip-skills
```

Every template gets the common skills `one-cli` and `find-skills`. Node templates also get `pnpm`:

| Template                                      | Additional skills                                                                                                             |
| --------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `nestjs-api`                                  | `one-nestjs`, `nestjs-best-practices`                                                                                         |
| `go-api`, `go-lib`                            | `one-go`, `golang-patterns`, `golang-testing`                                                                                 |
| `nextjs-app`, `nextjs-site`                   | `one-web`, React best practices, composition patterns, `shadcn`; use installed Next documentation                             |
| `fumadocs-docs`                               | The Next/React skills above plus `one-fumadocs`                                                                               |
| `react-spa`                                   | `one-web`, React best practices, composition patterns, `shadcn`, `vite`                                                       |
| `expo-mobile`                                 | `one-expo`, `expo-overview`, `expo-router`, `expo-data-fetching`, `expo-dev-client`, `expo-upgrade`, `expo-project-structure` |
| `ts-library`                                  | `tsdown`, `vitest`                                                                                                            |
| `electron-app`                                | `one-web`, React best practices, composition patterns, `shadcn`, `vite`, `vitest`, `tsdown`, `one-electron`                   |
| `empty-app`, `empty-service`, `empty-library` | Common skills only                                                                                                            |

Empty templates do not prescribe a language, framework, UI, database, or internal architecture; the user chooses the stack. Common skills guide the One workspace layout and workflow without requiring another template's stack. Once the stack is chosen, install matching development skills as needed with `one skills add`. The generated `AGENTS.md` follows the same rule: complex scripts use the project's chosen language without requiring Node or TypeScript.

One's `one-cli` skill supplies outer and inner layout guidance plus stack adaptations. Follow existing project dependencies and team instructions: generated web templates use Base UI, existing applications may use Radix; Expo uses native styles and its existing Axios/SWR stack; Electron uses Awilix function factories. Installing an upstream skill does not authorize changing these choices.

One maintains seven development skills:

| Skill          | Responsibility                                                                                 |
| -------------- | ---------------------------------------------------------------------------------------------- |
| `one-cli`      | One commands, workspace layout, and project discovery; no prescribed stack for empty templates |
| `one-nestjs`   | NestJS modules, Drizzle, injected configuration, validation, logging, and health               |
| `one-go`       | Go service/library layouts, explicit dependencies, Gin/Gorm/Viper/Zap, and resource ownership  |
| `one-expo`     | Native styles, Expo Router, MMKV, development clients, and SWR native listeners                |
| `one-web`      | Base UI, Axios/SWR/Zustand, and React SPA vs. Next.js SSR/static export                        |
| `one-electron` | Three-process projects, Awilix, preload/IPC, Ubuntu setup, and packaging                       |
| `one-fumadocs` | MDX, localized docs, navigation, and static search                                             |

These skills adapt One templates; upstream skills supply framework-wide best practices. `one-web` also covers the Web foundation of Electron renderers and Fumadocs; their specialized skills guide native processes and documentation content. Agents read only the skills and references needed for the current task. TypeScript libraries use upstream tool skills, and empty templates receive only common skills without stack adaptations. Existing projects can install the matching One skill manually from the workspace root; automatic installation adds missing names when a project is added.

The curated sources are [Vercel](https://github.com/vercel-labs/agent-skills), [Skills CLI](https://github.com/vercel-labs/skills), [shadcn](https://github.com/shadcn-ui/ui), [Antfu](https://github.com/antfu/skills), [tsdown](https://github.com/rolldown/tsdown), [Expo](https://github.com/expo/skills), [ECC Go skills](https://github.com/affaan-m/ecc), and [NestJS community skills](https://github.com/kadajett/agent-nestjs-skills). Default sources are pinned to reviewed commits in One's catalog; user commands remain unrestricted upstream commands.

## Files, conflicts, and recovery

Commit installed `.agents/skills` files and upstream `skills-lock.json`. New workspaces ignore the root `.one/` directory, and installation no longer creates a duplicate source tree under `.one/skill-sources` or `.agents/skill-sources`. One's editable skill sources remain in `packages/agent-skills/skills` in the One repository, separate from runnable templates and business skills.

Published CLI releases install One's skills from `1cli-team/one-cli/packages/agent-skills`, pinned to the release's full Git commit. The lock records that GitHub provenance, so restoration does not depend on local caches. Development and snapshot builds instead write only selected bundled skills into their final `.agents/skills` paths and have Skills CLI register `./.agents/skills` as the local source; this lets unpublished changes work without duplicate files or references to nonexistent remote commits. Commit these local copies too. If a development skill is deleted, restore it from Git or add the corresponding project through One again; a local source cannot download its own missing files.

Existing installations and locks pointing to `.one/skill-sources` are preserved when adding projects. Keep their referenced sources until you explicitly migrate using `one skills add` with a published One source and commit. Review team edits before reinstalling, then check that no lock entries refer to the old directory before removing it or ignoring `.one/` in an older workspace. One does not delete legacy directories or rewrite provenance automatically.

Skills are installed by template and read by task. Agents can use names and descriptions to select a skill, read its `SKILL.md` when activated, and open its references and scripts only when needed. Generated `AGENTS.md` describes this workflow rather than requiring every skill to be read at startup; exact loading behavior belongs to the agent.

Automatic installation only adds missing names. It preserves existing skill directories, team changes, and different sources; source conflicts and invalid/unsupported locks produce warnings instead of overwrites. Explicit `one skills` commands retain upstream's update/overwrite behavior. Upstream owns its lock format; One does not maintain a second skills lock.

Installation has a five-minute total timeout. Network/download/tool failures do not undo a generated workspace or project: creation succeeds with `warnings` containing diagnostics and a command to retry from the workspace root. Automatic installation shows only One's localized progress, hiding the upstream logo, generic tips, and installation UI. Failures retain their specific reason, and structured creation results stay parseable. Manual `one skills` commands still preserve all upstream output. Default commit pins remain in the lock; changing versions is an explicit source/ref decision rather than an automatic upgrade during project creation.

## Business skills are manual

Login/registration, billing, permissions, uploads, and other business skills are maintained in independent GitHub repositories, outside One CLI's source, templates, and release assets. Neither `one create` nor `one add` installs them.

After a business repository is published, a user can choose a named skill from the workspace root, installing it in the project without `--global`:

```bash
one skills add owner/business-skills --skill one-auth
```

This is a placeholder repository/name example, not a bundled or already published skill. Installation makes instructions available to an agent; it does not implement login or provision backend services. The user must request the business feature separately.

## Existing workspaces

For existing projects, inspect the actual dependencies and choose the matching skills explicitly with `one skills add`. Do not select a retired template based on its name alone. For example, a React/Vite project can install:

```bash
one skills add vercel-labs/agent-skills --skill vercel-react-best-practices vercel-composition-patterns
one skills add antfu/skills --skill vite
one skills add shadcn-ui/ui --skill shadcn
```

In the One source repository, its maintained skills can be installed from `./packages/agent-skills`. Editing a canonical skill does not automatically overwrite an installed copy; reinstall explicitly when updating the copy.
