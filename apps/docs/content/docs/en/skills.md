---
title: one skills install
description: Install the minimal One CLI skill for your coding agent.
---

The bundled `one-cli` skill has two responsibilities: follow [One Workspace Convention](https://github.com/1cli-team/one-workspace-convention), and discover supported workspace operations through the installed CLI's help. Command lists, flags, templates, and manifest schemas stay with the CLI rather than being copied into the skill.

```bash
one skills install
one skills install --agent claude-code --agent codex
one skills install --yes -o json
```

## Target selection

- `--agent <id>` / `-a <id>` selects a target explicitly, even before that agent has created its configuration directory. Repeat the option for multiple agents.
- With no explicit targets, an interactive terminal offers detected agents, all preselected.
- `--yes` / `-y` or non-interactive execution installs into all detected agents without prompting.
- If no targets are detected, the command fails and asks for an explicit `--agent`.
- `one skills` and `one skills install --help` only display help. The latter lists every supported agent ID.

Detection uses user configuration directories. Agent IDs from the previous installer remain supported. `CODEX_HOME`, `CLAUDE_CONFIG_DIR`, `XDG_CONFIG_HOME`, and `VIBE_HOME` override their respective default roots when set to absolute directories.

## Installation

Installation copies the embedded `one-cli/SKILL.md` into the selected agents' user skills directories. It works offline and from any directory. Duplicate targets sharing a directory are written once. Success output lists the destination directories; the skill is in `one-cli/SKILL.md` below each destination.

Repeating installation replaces that `one-cli` directory, including old reference files. Other skills stay intact. A legacy `one-cli` symlink is replaced with a direct copy without changing its shared store or other agents' links. If an installation fails after some targets succeed, the error's `context.installed_to` lists completed destinations; rerun the command after fixing the reported problem.

After installation, load the installed skill. Restart the agent session if it does not discover new skills automatically.

## Generated workspaces

`one create`, including preset creation, writes a short root `AGENTS.md`:

```markdown
# Development

Use the `one-cli` skill when developing this workspace.
If it is not installed, run `one skills install` first.
```

Creation does not install skills into your home directory. `one add` preserves existing instructions. For an existing workspace, install the skill and add the same guidance to its `AGENTS.md`.

## Updates and output

Ordinary CLI upgrades do not require reinstalling this skill: it directs agents to `one --help`, `one help --all`, and the relevant command help. Reinstall only when the skill's guidance itself changes or when configuring another agent.

Structured output uses `one-cli/skills-install/v1`, with `status`, `targets`, `installed_to`, and `skill_count` (one). Use the usual `-o json`, `-o yaml`, or `-o text` option. Installation failures use `SKILLS_INSTALL_FAILED`.
