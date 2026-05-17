# Mode: In-Place Migration

Use only when the user **must** keep the current directory as the
workspace root (git remote already configured, deployment paths
already wired up, etc.). Otherwise prefer `side-by-side.md`.

`one create .` will NOT work here:

- if the dir is non-empty, the CLI returns `EXISTING_TARGET_NOT_EMPTY`
- if any ancestor already has `one.manifest.json`, it returns
  `WORKSPACE_NESTED_FORBIDDEN`

So this path hand-writes the workspace scaffold files. The complete
set is fixed by `packages/cli/internal/scaffold/scaffold.go` — when
in doubt, re-read that file for the authoritative list.

## Inputs to extract

| Field | Required | Notes |
|---|---|---|
| `workspace_name` | yes | The `workspace.name` value. Must match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. Defaults to the current dir's basename, but ask for confirmation. |
| `source_projects` | yes | Per-subproject mapping built via `detect.md`. For each: current path inside the repo, target `relativeDir`, `name`, `templateId`, `toolchain`. |
| `keep_existing_tooling` | no | Whether to leave the user's existing `turbo.json` / `nx.json` / root `package.json` in place. Default **yes** (we're respecting their root). |

## Workflow

### Step 1 — Safety net

```bash
git -C . rev-parse --is-inside-work-tree    # must succeed
git status --porcelain                       # must be empty
```

Bail out if either fails — ask the user to commit / stash first.

### Step 2 — Confirm the dir isn't already a workspace

```bash
test ! -e one.manifest.json     # must not exist anywhere up the tree
```

Walk upward yourself; if any ancestor has it, this skill doesn't apply.

### Step 3 — Relocate sub-projects to canonical roots

Allowed root directories: `apps/`, `services/`, `packages/`
(hard-wired in `packages/cli/internal/workspace/roots.go`).

For each entry in `source_projects` whose current path **doesn't**
already sit under one of those roots:

```bash
mkdir -p <target_root_parent>          # e.g. apps/
mv <current_path> <relativeDir>        # e.g. mv web apps/web
```

Common no-op cases (skip the move):

- Already in `apps/<name>` / `services/<name>` / `packages/<name>`
- Turbo / pnpm default layout already used `apps/` and `packages/`

If the existing repo uses a different layout (`src/`, `frontend/`,
`backend/`), pick the closest of the three canonical roots and `mv`.

### Step 4 — Write workspace-root scaffold files

Create these files at the repo root. Match the `one create` output
verbatim (sources in `packages/cli/internal/scaffold/content.go`).
Skip a file if it already exists and the user wants their version
preserved — surface the conflict instead of overwriting.

**`pnpm-workspace.yaml`** (required):

```yaml
packages:
  - "apps/*"
  - "services/*"
  - "packages/*"
```

**Root `package.json`** (write only if missing):

```json
{
  "name": "<workspace_name>",
  "private": true,
  "version": "0.1.0",
  "packageManager": "pnpm@10.14.0",
  "scripts": {
    "prepare": "husky",
    "changeset": "changeset"
  },
  "devDependencies": {
    "@changesets/cli": "latest",
    "@commitlint/cli": "latest",
    "@commitlint/config-conventional": "latest",
    "husky": "latest"
  }
}
```

If the repo already has a root `package.json`, **merge instead of
overwriting**: keep the user's `name`, add `"private": true` if
missing, ensure `packageManager` is `"pnpm@10.14.0"` (warn if it's a
different package manager — One CLI only supports pnpm at the
workspace level today).

**`.gitignore`** (append if exists):

```
# dependencies
node_modules

# build output
dist
coverage

# environment
.env
.env.local

# secrets — private keys must NEVER be committed
.secrets/keys/

# misc
.DS_Store
```

**Optional but recommended (skip silently if files already exist):**

- `commitlint.config.js` — `module.exports = { extends: ['@commitlint/config-conventional'] };`
- `.husky/pre-commit`, `.husky/commit-msg` — see
  `packages/cli/internal/scaffold/content.go` for verbatim content
- `.changeset/config.json` — see same file
- `CLAUDE.md` — workspace-level agent guide. If the user already has
  one, leave it alone.

### Step 5 — Write `one.manifest.json`

This is the marker file that makes the directory a One workspace.
Generate with `uuidgen` (or any random 6-hex appender) and write:

```json
{
  "version": 1,
  "workspace": {
    "id": "<workspace_name>-<6-hex>",
    "name": "<workspace_name>"
  },
  "projects": []
}
```

Then for each sub-project, append to `projects[]` (use `jq` or a
careful Edit). The full schema is in `references/manifest.md`.

After writing, **verify**:

```bash
cat one.manifest.json | jq '.version'        # → 5
cat one.manifest.json | jq '.projects | length'   # → expected count
```

If `jq` reports a parse error, you have malformed JSON — re-do the
edit. Don't ship a half-broken manifest.

### Step 6 — Install dependencies

```bash
pnpm install     # workspace root; picks up apps/services/packages

# For each Go project:
(cd <relativeDir> && go mod download)
```

If the user previously had per-project `node_modules` (npm / yarn /
old pnpm), remove those and let the workspace-root install regenerate
them — `pnpm install` at the root won't share through stale per-project
`node_modules`.

### Step 7 — Verify end-to-end

```bash
one --version      # CLI on PATH (should already be true)
cat one.manifest.json | jq      # parses

# Smoke-test at least a project:
pnpm --filter <project_name> build    # node
(cd <relativeDir> && go build ./...)  # go
```

If `one dev` is what the user wanted: it relies on a Procfile.dev that
templates generate. Without a template-generated project there's no
Procfile.dev, so `one dev` won't have anything to run — that's expected,
not a bug.

## Common conflicts and resolutions

| Conflict | Resolution |
|---|---|
| Root `package.json` already exists | Merge — never overwrite. Preserve user's `dependencies`/`devDependencies`/scripts; add `"private": true` if missing. |
| `pnpm-workspace.yaml` already exists with different globs | Tell the user. If their globs include `apps/*`, `services/*`, `packages/*` (or equivalents), leave their config — One CLI only requires that **sub-projects live in those dirs**, not that the workspace globs are exactly those three. |
| User uses Yarn or npm workspaces | Surface this — One CLI currently assumes pnpm at the workspace level. Migrating package manager is out of scope; tell the user the workspace will be marked `packageManager: "pnpm"` and they need to migrate manually (or run with pnpm). |
| Project's current dir uses `src/` (e.g. `src/web`) | Move to `apps/web`. The `src/` convention isn't compatible with the hard-wired roots. |
| User has a `Dockerfile` per project | Keep them in place; record container intent in `manifest.projects[].domains.container` after migration if they want `one container build` to pick them up (see `manifest.md`). |

## Mode-specific error recovery

| Code | Recovery |
|---|---|
| `EXISTING_TARGET_NOT_EMPTY` | Shouldn't happen in this path (we never call `one create`); if it does, you accidentally ran `one create .` — abort and just write the scaffold files. |
| `WORKSPACE_NESTED_FORBIDDEN` | An ancestor has a manifest. This skill doesn't apply — point the user up the tree to the existing workspace. |
| `INVALID_NAME` | `workspace_name` or a project name doesn't match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. Convert to kebab-case. |

## Success response

Reply in the user's language. Include:

- The current directory is now a One workspace (workspace name + id)
- Which sub-projects landed at which `relativeDir`
- Which scaffold files were created vs. preserved
- Whether `pnpm install` succeeded
- The next command they probably want (`one dev`, `pnpm --filter X dev`, …)
