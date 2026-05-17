# Mode: Side-by-Side Migration (Default)

Use when the user is OK with placing their existing project(s) inside
a **new** sibling workspace directory. `one create` lays down a
known-good skeleton; this skill moves user code into it and registers
the entries in the manifest.

This is the **default** path. Pick `in-place.md` only when the user
needs the workspace root to be their current directory.

## Inputs to extract

| Field | Required | Notes |
|---|---|---|
| `workspace_name` | yes | Becomes the new workspace dir + `workspace.name`. Must match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. Convert fuzzy names ("我的中台" → `my-platform`). Ask if unclear. |
| `parent_dir` | no | Where the new workspace dir is created. Defaults to the parent of the user's existing project dir. |
| `source_projects` | yes | List of `{path, relativeDir, name, templateId, toolchain}`. For a single-project repo this is a list of one. Build via `detect.md`. |
| `move_strategy` | no | `mv` (default) or `cp -R`. Use `cp -R` only if the user wants to keep the original tree intact (e.g. their existing repo has a different git remote they want to preserve). |
| `enable_docker` | no | Pass `--docker` to `one create` if any source project uses containers / `Dockerfile`. |
| `enable_k8s` | no | Pass `--k8s` only when the user explicitly mentions Kubernetes. |

If anything required is unclear, ask one consolidated question.

## Workflow

### Step 1 — Safety net

```bash
# 1. Check each source project is a clean git repo
git -C <source_project> rev-parse --is-inside-work-tree
git -C <source_project> status --porcelain    # must be empty

# 2. Confirm the destination doesn't exist yet
test ! -e <parent_dir>/<workspace_name>
```

If `status --porcelain` is non-empty, stop and ask the user to
commit / stash. If the destination exists, ask for a different name —
**never** delete to make room.

### Step 2 — Detect (once per source project)

Run the fingerprint workflow in `detect.md` against each source
project. Build the `source_projects` list with `templateId`,
`toolchain`, and target `relativeDir` (`apps/<name>`, `services/<name>`,
or `packages/<name>`). Show the mapping to the user and get
confirmation **before** Step 3.

### Step 3 — Create the empty workspace

```bash
one create <workspace_name> --yes -o json [--docker] [--k8s]
```

Schema: `one-cli/create/v2`. Capture from the response:

- `created_path` — the absolute workspace root path
- `skills.status` — if `"failed"`, the workspace still exists; surface
  and continue, don't roll back

On `WORKSPACE_NESTED_FORBIDDEN`: `parent_dir` is already inside a
workspace. Move up one level or hand off to `in-place.md`.

On `EXISTING_TARGET_NOT_EMPTY`: the destination is somehow non-empty.
Re-check; do not delete.

### Step 4 — Move source code into the workspace

For each entry in `source_projects`:

```bash
# Default: move (preserves git history if the source was a submodule
# of the destination; otherwise it just relocates working tree).
mv <source_project>/<src> <created_path>/<relativeDir>

# Or, if move_strategy == cp:
cp -R <source_project>/<src>/. <created_path>/<relativeDir>/
```

**Don't** use `mv` with `--force`; **don't** add `cp -f`. If
`<created_path>/<relativeDir>` already exists (it shouldn't after a
fresh `one create`), stop and surface the conflict.

After the move, sanity-check the destination dir has the source's
top-level files (e.g. `package.json`, `next.config.js`, or `go.mod`).

### Step 5 — Register projects in the manifest

`one add` won't help here — it scaffolds **new** template stubs and
refuses if the target dir exists (`TARGET_EXISTS`). The manifest must
be updated by hand. Read `references/manifest.md` for the exact field
schema.

Use [jq](https://jqlang.github.io/jq/) (or a careful Edit) to append
each entry to `projects[]`:

```bash
cd <created_path>

# For each source project:
jq --arg name "<project_name>" \
   --arg dir "<relativeDir>" \
   --arg tid "<templateId>" \
   --arg tc "<toolchain>" \
   '.projects += [{
      name: $name,
      relativeDir: $dir,
      templateId: $tid,
      toolchain: $tc,
      buildVersion: "",
      packageManager: "pnpm"
    }]' one.manifest.json > one.manifest.json.tmp
mv one.manifest.json.tmp one.manifest.json
```

Notes:

- `templateId` empty string is **legal**; do not omit the field.
- `packageManager` defaults to `pnpm` for `toolchain: node` projects;
  omit (or set `""`) for `toolchain: go`.
- The CLI's read path sorts `projects[]` by `relativeDir` on save, so
  insertion order doesn't matter.

### Step 6 — Reconcile workspace-level tooling

Check whether the source projects bring conflicting workspace files:

| Conflict | Resolution |
|---|---|
| Source has its own root `package.json` | Discard it (the source's deps live in the per-project `package.json` inside `apps/<name>`). Manually merge any **dev** scripts the user wanted at workspace level into `<created_path>/package.json`. |
| Source has its own `pnpm-workspace.yaml` / `turbo.json` / `nx.json` | Delete from the source; One CLI's workspace uses pnpm with `packages: ["apps/*", "services/*", "packages/*"]` already. |
| Source has its own `.gitignore` at the project root | Keep at the project level — pnpm-workspace style allows nested `.gitignore`s. Don't merge into root. |
| Source has its own `Dockerfile` | Keep it inside `<relativeDir>/Dockerfile`. If `--docker` was set, you may also append a service entry to `<created_path>/docker-compose.yml` by hand (or re-run `one container` for that project after migration). |

### Step 7 — Install dependencies

```bash
cd <created_path>
pnpm install                            # picks up all node projects in apps/services/packages

# For each Go project:
(cd <relativeDir> && go mod download)
```

### Step 8 — Verify

```bash
# Manifest is structurally valid
cat one.manifest.json | jq '.version'   # → 5
cat one.manifest.json | jq '.projects | length'   # → expected count

# Smoke-test a project end-to-end
pnpm --filter <project_name> build      # node
# or
(cd <relativeDir> && go build ./...)    # go
```

If smoke fails because of the source project's own build issues
(unrelated to migration), surface that error to the user verbatim —
don't pretend the migration succeeded.

## Mode-specific error recovery

| Code | Recovery |
|---|---|
| `WORKSPACE_NESTED_FORBIDDEN` | `parent_dir` is inside a workspace. Move up a level or switch to `in-place.md`. |
| `EXISTING_TARGET_NOT_EMPTY` | Destination `<workspace_name>` already exists. Pick a different name; don't delete. |
| `INVALID_NAME` | `workspace_name` or a project name violates `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. Convert to kebab-case. |
| `SKILLS_INSTALL_FAILED` | The workspace still exists; agent-skill install path (`~/.claude/skills/` etc.) had a permission issue. Migration proceeds. |

## Success response

Reply in the user's language. Include:

- Created workspace path
- Each source project → its new `relativeDir` + `templateId`
- Whether `mv` or `cp` was used (so they know whether the source tree
  was preserved)
- Result of the smoke build (passed / failed + relevant error)
- Next steps: `cd <created_path>`, then any project-specific dev /
  build / deploy command
