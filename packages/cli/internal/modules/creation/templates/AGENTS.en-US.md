# Workspace development guidelines

## Understand the workspace

- Find the nearest `one.manifest.toml` in the current directory or its ancestors to identify project names, paths, and toolchains.
- `apps/` contains applications, `services/` contains backend services and workers, and `packages/` contains shared libraries.
- Read the relevant `README.md` and applicable `AGENTS.md` before changing a project.
- Consult `one --help` and command-specific `--help`; do not guess commands or flags. Before adding a project, discover templates with `one templates -o json`, then use `one add`.

## Use one task entry point

- Tests, checks, builds, dependency preparation, and project scripts must run through `one run <task>`. Select project tasks with `one run <task> -p <project>`.
- Inspect existing tasks with `one run` and reuse them first. Do not bypass this entry point by directly running commands such as `pnpm test`, `go test`, `node`, `docker`, or `mise run`; task definitions may call these native tools internally.
- If a required task is missing, define it in the project's `package.json`, `Taskfile.yml`, or the root `mise.toml`. After changing project tasks, synchronize with `one init mise` when needed, then execute through `one run`.
- One CLI help, task discovery, project creation, and variable management commands may be used directly without wrapping them in tasks.
- After making changes, use `one run` for the affected checks, tests, and necessary builds. Use task names from the actual configuration.

## Complex scripts

- For operations with multiple steps, complex branches, or repeated use, write TypeScript in `scripts/<name>.mts` instead of assembling long shell commands.
- Put scripts shared across projects in the workspace root's `scripts/`; put project-specific scripts in that project's `scripts/`.
- Configure the required Node version in mise and register the script as a task that calls `node scripts/<name>.mts`. The agent still executes it through `one run`.
- Read variables from the process environment. Do not write, expand, or print secret values in source code, command arguments, or logs.

## Environment variables

- Environment variables must be managed through One CLI or the One Dashboard. Do not create separate `.env` files as a variable source or write variable values into source code, `one.manifest.toml`, or `mise.toml`.
- Configure application runtime variables at workspace or project scope. One injects the corresponding project variables when running tasks. The default environment is `dev`; select another explicitly with `--env`.
- Scripts and application code read the process environment without connecting to Infisical themselves or reading local credential files.

## Global operation credentials

- Docker registry usernames and passwords, along with other publishing or operations credentials, belong in One's global variables. Keep them separate from application runtime variables.
- When credentials are missing, explain the required variable names, purposes, environment, and folder. Ask the user to run `one serve` and configure them on the Dashboard's “Shared credentials” page. Do not ask for credential values in chat.
- After the user configures them, inspect names with `one env list --global --env <environment> --path <folder>`. Have the task inject the required variables using `one exec --global` internally, with explicit `--env`, `--path`, and `--keys`.
- The agent still starts execution with `one run`. For example, when `scripts/push-image.mts` exists, the root `mise.toml` can define:

```toml
[tasks.push-image]
run = "one exec --global --env dev --path /docker --keys REGISTRY_USER,REGISTRY_PASSWORD -- node scripts/push-image.mts"
```

Then use `one run push-image`. The folder and variable names must match the user's Dashboard configuration.

## Automation and delivery

- Use `-o json` when parsing One CLI results. Handle errors using `error.code` and `error.context`, not translated messages; treat child-process output as logs.
- Report what changed, which checks actually ran and their results, and which checks remain unrun.

This file is maintained by the team. Later `one add` operations and language changes do not rewrite existing content.
