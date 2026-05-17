# CLAUDE.md

Agent guidance for working **inside** the One CLI repo (i.e. helping
develop One CLI itself, not using it from a workspace). Read this
before editing skills or bundled assets.

> **First-contact protocol.** Before relying on anything in this file
> as ground truth, run:
>
> ```bash
> task --list      # current tasks (this file may be stale)
> git ls-files     # current tree
> cat VERSION      # current version
> ```
>
> This file describes **invariants** — rules, design constraints,
> Don'ts. State (commands, file tree, version, release flow) lives
> in tooling output. **If you see this file contradict tooling, the
> tooling wins**; fix CLAUDE.md or open an issue.

## Repo overview

Go-based scaffolding + governance tool for AI-Native monorepo
workspaces. The repo itself follows the `apps/* + packages/*` layout
that `one create` produces for users:

- `packages/cli/` — Go module (`cmd/`, `internal/`, `pkg/`, `tools/`,
  `testdata/`). Module path: `github.com/torchstellar-team/one-cli/packages/cli`.
- `packages/templates/` — canonical template sources + `registry.json`.
- `packages/skills/` — bundled agent skills.
- `apps/docs/` — Next.js + Fumadocs SSG, served at one.torchstellar.com.
- `apps/dashboard/` — React + Vite UI shipped with `one serve`.

`packages/cli/internal/bundled/` is `go:embed`-ed at build time and
entirely gitignored. Two tasks regenerate it:

- `task sync-bundled` (cheap cp): `packages/templates/registry.json`,
  `packages/skills/`, `packages/templates/` → `bundled/registry.json`,
  `bundled/skills/`, `bundled/_templates/`.
- `task sync-web` (Node + pnpm + vite): `apps/dashboard/` →
  `bundled/_web/`. Slow on first run, near-free thereafter via task
  fingerprinting.

Both run as deps of `task vet` / `test` / `build`, so the normal
Taskfile flow always keeps the embed sources fresh. A fresh clone
needs `task sync-bundled && task sync-web` once before raw `go build`
or IDE-driven `gopls` will compile the bundled package.

## Writing or editing skills (under `packages/skills/`)

Skills follow the [agentskills.io](https://agentskills.io/specification)
specification — **read it before adding or restructuring a skill**.
Below is the rule sheet; the spec wins on disputes.

### Required structure

```
packages/skills/<skill-name>/
├── SKILL.md          # Required: frontmatter + instructions
├── references/       # Optional: docs the agent reads on demand
├── scripts/          # Optional: executable code (bash / python / js)
├── assets/           # Optional: templates, static resources
```

**Do NOT use other top-level subdirectory names** (e.g. `modes/`,
`docs/`, `playbooks/`). Use `references/`. The CLI does not enforce
this — it just `cp -R`s the directory — but agent ecosystem tooling
(`npx skills validate`, `skills-ref validate`, etc.) will reject
non-standard names.

### SKILL.md frontmatter

```yaml
---
name: skill-name           # MUST match parent directory name. lowercase + hyphens, ≤ 64 chars, no leading/trailing/consecutive hyphens
description: ...           # ≤ 1024 chars. What it does AND when to use. Include trigger keywords.
license: MIT               # Optional. License name or reference to a bundled file.
metadata:                  # Optional. Free-form key/value (the spec ALLOWS arbitrary keys).
  author: torchstellar-team
  version: '0.1.0'
---
```

**Do NOT add a separate `metadata.json` file.** All metadata goes in
the frontmatter `metadata:` block. The spec does not define a
`metadata.json` file at all; we mistakenly added one in v0.3.0 and
removed it in v0.4.0.

### File naming inside `references/`

- `REFERENCE.md` (uppercase) — the canonical technical reference
- `INDEX.md` (uppercase) — entry point / decision tree if you have one
- Domain-specific files use lowercase: `bootstrap.md`, `fix.md`,
  `auth.md`, etc.
- **No leading underscore** (`_index.md`, `_shared.md`) — pick a
  meaningful name instead.

### Body length

Keep `SKILL.md` under **500 lines** (spec recommendation). Push
detailed content into `references/<name>.md`. Agents load the body
on activation and reference files only when needed (progressive
disclosure).

### Cross-references

Use relative paths from the skill root. Keep references **one level
deep**:

```markdown
See [the bootstrap workflow](references/bootstrap.md).
```

Don't chain (`references/foo/bar.md` → `references/baz.md`); flatten.

### After editing any file under `packages/skills/`

`task sync-bundled` regenerates the gitignored mirror at
`packages/cli/internal/bundled/skills/`. It runs as a dep of `vet` /
`test` / `build`, so most workflows pick it up automatically; you
only need to run it explicitly if you're invoking `go` commands
directly (or want gopls to see the new files immediately).

## Building and testing

Use Taskfile, not bare `go` invocations:

```bash
task --list          # see all available tasks (authoritative)
task pre-push        # MUST be green before pushing — this is the contract
```

Key invariants the test suite enforces:

- snapshot tests under `packages/cli/internal/cli/` lock the JSON envelope shape
- `install_sh_test.go` keeps the curl installer's safety sentinels in place

The whole `packages/cli/internal/bundled/` tree is gitignored: don't
try to commit anything inside it, and don't hand-edit. It's
regenerated from canonical sources (`packages/templates/`,
`packages/skills/`, `apps/dashboard/`) on every `task vet` / `test` /
`build` via the `sync-bundled` + `sync-web` deps.

## Refreshing the bundled skill while developing

After editing anything under `packages/skills/one-cli/`, push the new
content onto your local agent paths to test it live:

```bash
task sync-bundled
task install-local   # symlinks packages/cli/bin/one → ~/.local/bin/one
one skills install   # writes bundled skill into ~/.<agent>/skills/
```

(If task names change, follow `task --list`.)

## Public API stability (`packages/cli/pkg/`)

Anything under `packages/cli/pkg/` is intended to be importable by
external Go modules and follows semantic versioning. Don't change
exported types, function signatures, or struct fields under
`packages/cli/pkg/` without considering downstream breakage. Anything
under `packages/cli/internal/` is fair game.

Run `git ls-files packages/cli/pkg/` for the authoritative current
list of public packages.

## Documentation drift — single source of truth

Three sources of truth feed every doc surface. Treat them as gospel;
treat their copies as untrusted caches that CI will refuse if they
drift:

- **Command names**: cobra tree (`packages/cli/internal/cli/`,
  `packages/cli/internal/cmd/*/`). Verified against every prose doc by
  `task verify-cli-references`.
- **Help text**: every cobra command's `--help` output is snapshotted
  under `packages/cli/testdata/reference/help/`, and structural
  invariants are guarded by `task verify-help`: the `rootHelp` constant
  in `packages/cli/internal/cli/root.go` must list exactly the
  registered top-level commands, and every `--flag` named in an
  `Example`/`Long` block must exist on the resolved command. Refresh
  the snapshots after intentional edits with
  `UPDATE_SNAPSHOTS=1 go test ./internal/cli/ -run TestHelpSnapshots`.
- **Error codes**: `packages/cli/internal/errors/codes.go`. Generated
  to `apps/docs/content/docs/reference/error-codes.md` by
  `task gen-error-codes`.
- **Version number**: `VERSION` file. Verified to match SKILL.md
  frontmatter and installation doc samples by `task verify-versions`.

For the "current public packages" / "current command list" / "current
template list" question, run `git ls-files` or `task --list`. **Don't
enumerate these in prose** — the lists rot; the tooling never does.

When you add a new fact list, ask first: is it derivable from code?
If yes, add a `gen-` or `verify-` task for it before merging the doc
that consumes it. All checks aggregate into `task verify-docs`, which
runs in CI and `pre-push`.

To intentionally write a deprecated command name (migration guide,
changelog, "was X, now Y" tables), wrap the section in:

```markdown
<!-- verify-cli:ignore-start -->
... section that mentions removed commands ...
<!-- verify-cli:ignore-end -->
```

Place these markers OUTSIDE fenced code blocks — putting `ignore-end`
inside an open fence will mask the closing ` ``` ` from the scanner.

## Don't

<!-- verify-cli:ignore-start -->
- Don't reintroduce `one skill` user-facing commands. External skill
  management is `npx skills`'s job (vercel-labs/skills CLI). One
  CLI's responsibility is only its own bundled skill.
<!-- verify-cli:ignore-end -->
- Don't add a `metadata.json` file in any skill (use frontmatter).
- Don't auto-install the `one` binary from a skill — point the user
  at the curl-based installer (see README) or the GitHub Releases
  page. Global binary installs are outside an agent's normal
  authority.
- Don't paste current state into this file. Repo overview, command
  lists, file trees and version numbers belong in tooling output
  (`task --list`, `git ls-files`, `cat VERSION`). This file is for
  invariants and Don'ts.
- Don't write `apps/docs/content/docs/reference/error-codes.md` by
  hand — it's generated by `task gen-error-codes` from
  `packages/cli/internal/errors/codes.go`.
