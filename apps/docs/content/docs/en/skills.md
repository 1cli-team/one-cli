---
title: one skills
description: "One CLI skills (one-cli, one-migrate): what they do, when they trigger, and how one skills install puts them in your coding agent."
---

One CLI ships two agent skills with every release. Once installed onto your machine's coding agent, the agent auto-loads the matching skill when it sees an intent that fits — so you don't have to re-explain how One CLI works on every prompt.

## What is a Skill

A skill is the [agentskills.io](https://agentskills.io/specification) format: a directory containing one `SKILL.md` (frontmatter description + routing table) plus optional `references/`, `scripts/`, `assets/`. The agent reads the frontmatter at startup to decide whether to activate; only after activation does it pull in `references/` files on demand — progressive disclosure to keep the context window tight.

One CLI is the **author** of these skills, not a delivery channel for third-party ones, so installing them isn't "adding a plugin" — it's giving your agent the official handbook for using One CLI.

## The Two Bundled Skills

### `one-cli`

Covers every routine task **inside or on the way into** a One CLI workspace: create a workspace, add a project, fix a manifest, look up a command / JSON schema / error code.

Trigger keywords (from [SKILL.md](https://github.com/torchstellar-team/one-cli/blob/master/packages/skills/one-cli/SKILL.md) frontmatter):

| Bucket | English | 中文 |
|---|---|---|
| Create a workspace | create a new project / scaffold a workspace | 建一个新项目 / 新建 workspace / 搭脚手架 |
| Add a project | add a backend / add a frontend | 新增一个服务 / 加一个前端应用 |
| Fix / diagnose | fix the workspace / manifest is out of sync | 检查项目 / 修一下 / manifest 不同步 |
| Look up command / schema / error code | look up exact command / JSON schema / error code | — |

### `one-migrate`

Covers the path that has **no dedicated CLI command**: taking a project the user already has — a standalone Next.js app, a Go service, a small pnpm / turbo / nx monorepo — and turning it into a valid One CLI workspace. The output is shape-identical to anything `one create` would produce.

Trigger keywords:

| English | 中文 |
|---|---|
| migrate to one cli | 迁移到 one cli |
| adopt my existing project | 把项目改造成 one workspace |
| convert this repo to a one workspace | 现有项目接入 one cli |
| — | 把 monorepo 改成 one cli |

> Out of scope: cross-stack rewrites (e.g. CRA -> Next.js) and manifest schema migrations.

## Installation: `one skills install`

```bash
one skills install                          # interactive multi-select; only Claude Code pre-checked
one skills install -a cursor -a claude-code # install to specific agents
one skills install --yes                    # install to every detected agent (CI)
```

Behavior:

1. **Auto-detects** installed coding agents on the machine — Claude Code, Cursor, Codex, Gemini CLI, GitHub Copilot, OpenCode, Cline, and 50+ others. Full list: `one skills install --help`.
2. **Interactive mode**: opens a multi-select list; space toggles and enter confirms. Only Claude Code is pre-checked.
3. **Non-TTY or `--yes`**: installs to every detected agent. If none are detected, falls back to Claude Code's default path.
4. **Materialises** every One CLI skill into `~/.one/skills-store/one-bundled/<skill-name>/`. Each target agent gets a **symlink** in its global skills directory pointing at the store.
5. **Windows fallback**: when the OS rejects symlinks (no dev-mode privilege), the CLI silently switches to a directory copy.
6. **Idempotent**: safe to re-run. After upgrading the `one` binary, run it once to refresh the skill content across all targets.

After install, `~/.one/skills-store/one-bundled/one-cli/SKILL.md` and `~/.one/skills-store/one-bundled/one-migrate/SKILL.md` exist in the store; the agent picks them up through its own global skills directory.

Output schema: `one-cli/skills-install/v1`, with `targets`, `installed_to`, `skill_count`.

## Interactive Mode

Running `one skills install` opens the agent multi-select list, which is best for local human setup. To install to specific agents without the list, pass `-a` explicitly:

```bash
one skills install -a cursor -a claude-code
```

For CI or installing to every detected agent, use:

```bash
one skills install --yes
```

## Reading the skill source

The full skill bodies are intentionally **not inlined here** (to avoid drift against the source). Read them directly in the repo:

- [`packages/skills/one-cli/`](https://github.com/torchstellar-team/one-cli/tree/master/packages/skills/one-cli) — SKILL.md + references/INDEX.md + each playbook
- [`packages/skills/one-migrate/`](https://github.com/torchstellar-team/one-cli/tree/master/packages/skills/one-migrate) — same shape

Or open the local store after install: `~/.one/skills-store/one-bundled/`.

## Installing external / community skills

One CLI **does not** distribute external skills — that's [vercel-labs/skills](https://github.com/vercel-labs/skills) territory. Use:

```bash
npx skills add <github:org/repo | https://… | …>
```

The on-disk format is compatible with One CLI skills; agents see a uniform directory shape.

## Further reading

- [`one` overview](/en/docs/cli-overview/) — `one skills` sits among the top-level commands
- [agentskills.io specification](https://agentskills.io/specification) — the skill standard itself
- [`one create`](/en/docs/create/) — first command the `one-cli` skill activates on
