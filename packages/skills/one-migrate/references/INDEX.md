# Migration Mode Index — Decision Tree

Use this when the user's intent isn't obviously "side-by-side" or
"in-place" from their initial request. Two questions are enough to
pick the path.

## Step 1 — Is the target directory already a One workspace?

Walk upward from the directory the user has named (or cwd) looking
for `one.manifest.json`.

| Found `one.manifest.json`? | Then |
|---|---|
| Yes, in the user's project root or an ancestor | The workspace already exists. This skill doesn't apply — you want **`one-cli` skill → `add-feature.md`** (adopt nothing, just add a new template-based project). |
| No, anywhere upstream | Continue to step 2. |

`one create` refuses with `WORKSPACE_NESTED_FORBIDDEN` if it sees an
enclosing workspace, so this check spares you a useless round-trip.

## Step 2 — Move existing code, or stay in place?

Ask the user one concise question if you can't infer it:

> 把现有项目搬进一个新的 workspace 目录（`one create` 生成骨架，
> 把代码 `mv` 进去），还是直接在当前目录补齐 workspace 元文件？

| Answer | Path |
|---|---|
| 新建 workspace 目录 / 让我开干就行 | `side-by-side.md` (**default**, safer) |
| 必须保留当前目录（git remote、CI、文档路径都依赖它） | `in-place.md` |

Tie-breaker rules:

- The current directory has a `.git/` you cannot move (origin already
  pushed, sub-repos, etc.) → **in-place**.
- The current directory is a single-project repo with no monorepo
  scaffolding → **side-by-side** (safer + cleaner).
- The current directory is already a pnpm / turbo / nx monorepo with
  `apps/` or `packages/` already populated → either works; bias to
  **in-place** so you don't rewrite tooling configs.

## Step 3 — Single project or monorepo?

Affects both paths the same way:

- **Single project** → produces a manifest with one entry in
  `projects[]`. Most common case.
- **Monorepo (small)** → iterate `references/detect.md` per
  sub-project, register each in `projects[]`. Cap at ~10 projects
  for v1 — for larger monorepos, walk the user through the first
  3–4 and let them mirror the pattern.

Both paths route to the same final state: a workspace-root manifest
with `version: 5` and `projects[]` populated by `references/manifest.md`.
