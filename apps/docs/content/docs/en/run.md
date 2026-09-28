---
title: one run
description: Discover and execute workspace tasks with mise.
---

One selects projects and freezes their environment variables. mise schedules the task graph. Node projects define commands in `package.json` scripts and run them with pnpm; Go projects define them in Taskfile and run them with Task.

```sh
one run                          # List tasks without executing them
one run --list -p web -o json
one run build                    # Build the workspace and local dependencies
one run build -p web -p api
one run test -p api -- -run TestHandler
one run check --env prod
one run ci
one run build --dry-run -o json
```

## Selection and arguments

Without `-p`, a task selects the workspace aggregate. Repeat `-p` to select project names or workspace-relative paths. Everything after `--` is forwarded to one selected task; forwarding arguments to multiple projects is rejected. `one build web` shares the execution path of `one run build -p web`.

`one exec web -- pnpm add axios` runs an arbitrary command. It does not invoke a task graph or install application dependencies.

## Configuration

One maintains root and project `.mise/conf.d/one.toml` fragments. They contain tool pins, task adapters, aggregates, and local Node build dependencies. Edit `mise.toml` for your overrides; generated fragments have ownership checks and cannot be edited in place. Running a task refreshes generated configuration. Root `build`, `check`, and `test` aggregate only available project tasks; `ci` combines them.

Project tasks use canonical names such as `//apps/web:build`. Local Node dependencies with a build task run before dependent build, check, test, typecheck, and dev tasks. Go resolves module dependencies through `go.work` and its compiler. New Node workspaces use pnpm exclusively.

List and dry-run use static configuration and never execute mise, install tools, read secrets, or write files. Dynamic task expressions and parameterized dependencies cannot be previewed. Actual runs also read mise's effective task catalogue. Native mise tasks with their own commands use mise's environment; generated project adapters use One's project environment.

## Define a task

Keep project commands in `package.json` scripts or a Taskfile. For example, adding `"docs:build": "typedoc"` makes `one run docs:build -p lib` available. Define workspace tasks in the root `mise.toml`:

```toml
[tasks.verify]
depends = ["//apps/web:check", "//services/api:test"]

[tasks.hello]
description = "Print a greeting"
run = "echo hello"
```

Run the aggregate with `one run verify`, or the custom command with `one run hello`. You can also put executable scripts in `.mise/tasks/`, or load a separate task file with `[task_config] includes = ["tasks.toml"]`.

Inspect native configuration with `one mise tasks ls --all --local` and `one mise tasks info //apps/web:build --json`. Custom native tasks can run directly with `mise run hello`; generated project tasks need `one run` to prepare their environment context.

## Cache

Generated configuration enables mise's experimental features. Known template **build** tasks receive `sources`, `outputs`, and an environment fingerprint. Other tasks stay uncached until they have an explicit cache contract. Changing a template build script or build configuration disables automatic caching; declare the actual inputs and outputs in the project's `mise.toml` to enable it again. Cache storage and artifact restoration belong to mise.

```sh
one run build --cache local-only
one run build --cache off --force
one run build --cache read-only
one run build --concurrency 4
```

`--cache off --force` ensures execution even if source freshness checks would skip a task. Cache modes also include `read-write` and `write-only`; remote access requires your own mise cache backend configuration. One does not provision a remote service.

To opt a generated task into caching, declare its complete inputs and outputs in the project's `mise.toml`, including One's fingerprint command:

```toml
[tasks.build]
sources = ["src/**/*", "package.json", "tsconfig.json", "../../pnpm-lock.yaml"]
outputs = ["dist"]
cache = { enabled = true, env = ["NODE_ENV"], command_inputs = ['one __task-input --project "web" --task "build"'] }
```

`outputs = []` is appropriate for a deterministic task that only verifies inputs. Declare every upstream source, external file, configuration value, and compiler input that affects a custom task. Outputs must stay inside the task directory and must not overlap another selected task's outputs. Use `--cache off --force` while checking a new cache declaration.

One loads each managed project's environment once per invocation, before cache lookup, into a temporary context. The fingerprint and command consume that same snapshot. Environment values remain out of generated TOML and structured results. Every injected project variable contributes to the fingerprint; authentication credentials used only to fetch variables do not. Directly invoking a generated adapter through `mise run` requires that context, so use `one run` for these tasks.

## Terminals and output

Finite tasks use streamed mise output. `--ui raw` preserves native terminal input and disables caching. `--ui tui` is available for development through `one run dev` / `one dev`; finite tasks report that it is unavailable. Use a dedicated `test:watch` task or raw mode for interactive tests. Development runs finite upstream builds before starting the existing supervisor.

JSON/YAML previews use `one-cli/task-plan/v1`; execution uses `one-cli/task-result/v1`. Child logs go to stderr in structured mode. The result reports the overall status and exit code. Per-task status remains `unknown` because the selected mise version does not provide reliable structured completion events. One does not infer cache hits from console text.

## GitHub Actions

Keep your workflow in the repository and invoke the same tasks as local development. Once One is installed, a minimal job can use:

```yaml
- uses: actions/checkout@v7
- uses: jdx/mise-action@v4
  with:
    version: 2026.9.7
    experimental: true
- uses: actions/cache@v4
  with:
    path: .cache/mise-task-artifacts
    key: tasks-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('**/pnpm-lock.yaml', '**/go.sum', '**/mise.toml') }}-${{ github.sha }}
    restore-keys: tasks-${{ runner.os }}-${{ runner.arch }}-
- run: one run ci --ui stream
  env:
    MISE_TASK_CACHE_DIR: ${{ github.workspace }}/.cache/mise-task-artifacts
```

The cache directory must be excluded from task sources and version control. Cache keys isolate runner platforms; mise validates individual artifact keys. For workflows reading secrets, apply the repository's normal permissions and branch trust rules.

See [mise task caching](https://mise.jdx.dev/tasks/caching.html) and [mise configuration](https://mise.jdx.dev/tasks/task-configuration.html) for native options.
