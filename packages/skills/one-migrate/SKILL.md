---
name: one-migrate
description: Use this skill to migrate an existing project — a single repo or a small monorepo — INTO a One CLI workspace structure (one.manifest.json v1 at the root, code under apps/services/packages). Triggers include "migrate to one cli", "adopt my existing project", "convert this repo to a one workspace", "迁移到 one cli", "把项目改造成 one workspace", "现有项目接入 one cli", "把 monorepo 改成 one cli". Pairs with the `one-cli` skill, which handles fresh `one create` scaffolds and adding new template-based projects. Do NOT use this skill for migrating manifest schema versions or for cross-stack code rewrites (e.g. CRA → Next.js).
license: MIT
metadata:
  author: torchstellar-team
  version: '0.1.0'
---

# One CLI — Migration Skill

This skill orchestrates the path that has no dedicated CLI command:
taking a project the user **already has** — a standalone Next.js app,
a Go service, a small pnpm / turbo / nx monorepo — and turning it
into a valid One CLI workspace. The output is always the same shape:

- `one.manifest.json` (schema v1) at the workspace root
- code under `apps/` / `services/` / `packages/` (the three hard-wired
  roots; see `packages/cli/internal/workspace/roots.go`)
- workspace-level tooling files (`pnpm-workspace.yaml`, root
  `package.json`, `.gitignore`, `CLAUDE.md`, etc.)

After migration the workspace is interchangeable with one produced by
`one create`; `one dev`, `one container build`, `one deploy`, `one env`
all work the same way.

## Quick Routing

Pick the row that matches the user's situation, then read **only that
one file**.

| Situation | Read |
|---|---|
| Single existing repo, **new** sibling workspace is OK | `references/side-by-side.md` |
| Existing repo / monorepo, must stay in current directory | `references/in-place.md` |
| Need to decide which `templateId` matches the existing code | `references/detect.md` |
| Need the exact `one.manifest.json` v1 schema to hand-write | `references/manifest.md` |
| Not sure where to start | `references/INDEX.md` |

`side-by-side` is the **default** path when both work — it lets
`one create` lay down a known-good skeleton, after which the migration
reduces to "move files in + register them in the manifest". Pick
`in-place` only when the user explicitly wants to keep the current
directory as the workspace root (e.g. their git remote is already
set up at that path).

## Binary Prerequisite (READ BEFORE EVERY MODE)

Every workflow assumes the `one` binary is on PATH. Verify:

```bash
one --version
```

If this command fails, **stop and tell the user**:

> One CLI 还没装。请用 curl 一键安装（macOS / Linux）：
>
> ```bash
> curl -fsSL https://1cli.dev/install.sh | bash
> ```
>
> 或从 GitHub Releases 下载对应平台的归档：
> https://github.com/1cli-team/one-cli/releases/latest
>
> 装完再回来重试这个请求。

Do **not** install the binary yourself, even with prior permission —
global binary installs are outside an agent's normal authority.

## Hard Rules (apply to every migration)

### 1. Make a safety net before touching files

Before any file move, `cp`, or in-place write:

1. Confirm the user's project is a git repository (`git -C <dir> rev-parse --is-inside-work-tree`).
2. Confirm the working tree is **clean** (`git status --porcelain` → empty).
3. If either fails, stop and ask the user to either `git init && git add -A && git commit -m "snapshot before one-migrate"` or stash existing changes.

This is the only rollback story available — the skill never tries to
"undo" a partial migration.

### 2. Never use `--force` or overwrite-style copies

Use `mv` (default) or `cp -R` without `-f`. If a destination already
exists, **stop and ask**. Silently overwriting user code is the worst
possible failure mode.

### 3. `templateId` must come from `one templates -o json`

Run `one templates -o json` to get the authoritative list. Map detected
stacks to one of those IDs via `references/detect.md`. If nothing
matches cleanly, leave `templateId` as `""` (empty string) and tell
the user — **never invent a new ID**.

### 4. Projects live in one of three roots

`apps/`, `services/`, `packages/` are hard-coded in
`packages/cli/internal/workspace/roots.go` (no user override). Convention:

- frontend / user-facing → `apps/`
- backend / API / worker → `services/`
- library / shared code → `packages/`

`relativeDir` in the manifest must begin with one of those prefixes.

### 5. Project name must match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`

Same rule as `one create` / `one add` (see
`workspace.IsValidProjectName`). Kebab-case directories like
`@scope/my-app` → strip the scope to `my-app`. Ask the user when in
doubt — don't guess.

### 6. Manifest schema is v1, period

`one.manifest.json.version` must be `1`. The CLI rejects anything
else (`packages/cli/internal/workspace/manifest.go:241`). After writing
the manifest, verify with:

```bash
cat one.manifest.json | jq '.version'   # → 1
```

### 7. Read `error.code`, never parse `error.message`

Errors are `{schema: "one-cli/error/v1", error: {...}}` envelopes.
Branch on `error.code`; the message is for humans and may change.

### 8. End-to-end check before declaring success

After every migration, run **one** end-to-end command appropriate to
the toolchain — never stop at "manifest looks right":

| Toolchain | Smoke command |
|---|---|
| node | `pnpm install` at workspace root, then `pnpm --filter <name> build` |
| go | `(cd <relativeDir> && go mod download && go build ./...)` |
| any | `one dev` (relies on a Procfile.dev — only meaningful if the project's template would have generated one) |

If the smoke command fails because the user's project has its own
build issues unrelated to migration, surface that to the user instead
of swallowing it.

## Common Error Recovery

(Cross-mode error codes — see `references/REFERENCE.md` in the `one-cli`
skill for the full table.)

| Code | Recovery |
|---|---|
| `WORKSPACE_NESTED_FORBIDDEN` | `one create` refused because there's already a `one.manifest.json` somewhere in the ancestor chain. Either switch to `in-place.md`, or pick a parent dir outside the existing workspace. |
| `EXISTING_TARGET_NOT_EMPTY` | `one create <dir>` requires the target to be empty / non-existent. Pick a different workspace directory; do **not** delete the user's existing project to make room. |
| `INVALID_NAME` | The project / workspace name fails `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. Convert to kebab-case and re-run. |
| `TEMPLATE_NOT_FOUND` | Detection picked a `templateId` that isn't in `one templates`. Drop the field (set to `""`) or pick the closest match. |
| `NOT_ONE_PROJECT` | After migration, manifest is missing or unreadable from cwd. Walk upward; if absent, the manifest write step was skipped. |
| `TARGET_EXISTS` | A `mv`/`cp` would clobber an existing dir under the workspace root. Pick a different `relativeDir` or a different `name`. |

## References

- This skill's reference set: `references/INDEX.md`, `references/detect.md`,
  `references/side-by-side.md`, `references/in-place.md`,
  `references/manifest.md`
- Companion skill: `one-cli` (for `one create` / `one add` / per-domain commands)
- Manifest schema source: `packages/cli/internal/workspace/manifest.go`
- Workspace scaffold inventory: `packages/cli/internal/scaffold/scaffold.go`
- Template registry: `packages/templates/registry.json`
- Error code reference: <https://github.com/1cli-team/one-cli/blob/master/packages/cli/internal/errors/codes.go>
- Agent Skills format spec: <https://agentskills.io/specification>
