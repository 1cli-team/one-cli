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

Discover tasks with `one run --list -o json`. For non-interactive agent execution,
use `one run <task> -p <project> --ui stream -o json` for project checks, builds,
tests, dependency preparation, and scripts. Explicit stream mode keeps logs
readable when the agent allocates a terminal; reserve TUI for a user-requested
interactive view. Tasks that require terminal input use their declared
interactive/raw mode.

Reuse existing commands and tasks before writing helpers. For one-off commands,
use `one exec <project> -- <command> [args...]`; keep short one-off logic inline
or send it to an interpreter's stdin. If a script is warranted, put it and its
intermediate files in an OS temporary directory, pass absolute paths because
`one exec` runs in the project directory, and clean up when finished. Reuse
One's environment injection and existing CLI operations inside the workflow.
Do not add repository scripts, package scripts, or task definitions solely for
one-off work. `one exec` forwards stdout/stderr and the exit code, and has no
`--ui` flag.

Create a persistent script when the user requests it as a deliverable or the
operation is intended for reuse and maintenance. Length or multiple steps alone
do not make an operation reusable. Put shared reusable scripts in root
`scripts/` and project-specific scripts in the project, following the
repository's script language convention. Define reusable tasks in the project's
package.json, Python pyproject.toml (`[tool.one.tasks.<name>]` with a `run`
argument array), Taskfile.yml, or root mise.toml. Synchronize with `one init mise`
and run them through One.

Use `-o json` when parsing One's own structured results; branch on error
codes and context. Capture `one run` stdout separately from its live stderr
logs, and check the final result and exit code. Preserve subprocess output,
including multiline text and ANSI colors. If the execution tool returns a
running session, retain its handle to read later logs and the final exit status.
`one exec`, `one skills`, `one mise`, and `one hk` forward native tool output;
do not parse that output as One's JSON envelope.

Manage runtime variables through One or One Desktop; application code reads
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

Official One skills are initially copied from the CLI bundle, then registered
in the upstream `skills-lock.json` with their GitHub `master` source. Update
project skills with `one skills update --project`; `npx skills update --project`
uses the same records. Updates follow the repository independently of CLI
releases. Check the installed CLI's help for supported commands and flags.

Use `one init skills --dry-run` to preview initialization or source registration,
then `one init skills` to apply it. This installs missing `one-cli` guidance and
registers unchanged older local copies without replacing their files. Select
specific official skills with `one init skills --skill one-go`. Modified or
unknown copies are skipped. Existing remote sources and pinned refs are retained.
CLI upgrades alone do not change a workspace's skills.

Upstream updates may replace local edits. Keep durable team rules in `AGENTS.md`
or a separate skill. Global installation and updates require an explicit
`--global`. Installing from GitHub does not require a GitHub Release.

Business skills are optional instructions maintained in separate GitHub
repositories. Install them only when the user requests installation, using
`one skills add <owner/repo> --skill <name>` from the workspace root. Install
skills in the project's `.agents/skills`, without `--global`. Installing a
skill does not authorize implementing a feature or provisioning an external service.
