---
name: one-cli
description: Develop and manage One CLI workspaces, discover installed commands and tasks, and apply project layouts within apps, services, and packages.
---

# One CLI

Find the nearest `one.manifest.toml`; use its project names, paths, toolchains,
and groups rather than assuming a single application. Read applicable
`AGENTS.md` and project README files. Team instructions and the user's chosen
stack take precedence over examples in installed skills.

For `empty-app`, `empty-service`, and `empty-library`, the user defines the
language, frameworks, UI, database, and other tooling. These templates only
establish the One workspace location and workflow, not a default stack or
internal architecture. Do not infer a stack from sibling projects or impose
dependencies from another template or installed skill. Once a stack is chosen,
adapt project tasks and relevant development skills to that choice.

Discover commands with `one --help`, `one help --all`, and command-specific
help. Discover templates with `one templates -o json` before `one add`.
The installed CLI is the authority; do not invent flags or commands.

Use `one run` to discover tasks and `one run <task> -p <project>` for project
checks, builds, tests, dependency preparation, and scripts. If a task is
missing, define it in the project's package.json/Taskfile.yml or root
mise.toml, synchronize with `one init mise`, and run it through One.
Keep shared multi-step scripts in root `scripts/`; project scripts belong
in the project. Follow the repository's script language convention.

Use `-o json` when parsing One's own structured results; branch on error
codes and context. `one skills`, `one mise`, and `one hk` forward native
tool output and flags, so their output does not use One's JSON envelope.

Manage runtime variables through One or Dashboard; application code reads
the injected process environment. Do not introduce another .env source.
Publishing credentials belong in Shared credentials, configured by the
user, and are injected by a task using `one exec --global`. Never request
or print their values.

Read [project layouts](references/layouts.md) when adding or restructuring
a project. Read [stack adaptations](references/stacks.md) when an upstream
skill's examples differ from this project's dependencies or conventions.

For deeper template-specific work, select the installed one-nestjs, one-go,
one-expo, or one-web skill for that stack. Electron process boundaries use
one-electron; Fumadocs content and search use one-fumadocs. Read only the skill
and references needed for the current task. Their absence does not prescribe
a stack for an empty/custom project; follow its actual dependencies and README.

Business skills are optional instructions maintained in separate GitHub
repositories. Install them only when the user requests installation, using
`one skills add <owner/repo> --skill <name>` from the workspace root. Install
skills in the project's `.agents/skills`, without `--global`. Installing a
skill does not authorize implementing a feature or provisioning an external service.
