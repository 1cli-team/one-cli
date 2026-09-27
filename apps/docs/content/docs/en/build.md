---
title: one build
description: Build all projects or one selected project.
---

`one build` prepares tools and application dependencies, then runs project build tasks to completion.

```bash
one build
one build web
one build apps/web
one build -p web --env prod
one build --dry-run -o json
```

Without a selector, builds all buildable manifest projects, even when invoked from a project directory. A project name or workspace-relative path selects only that project; its local dependencies are not automatically built.

## Build commands

- Node: runs `build` from the current `package.json` using the workspace package manager (`pnpm`, `npm`, `yarn`, or `bun`).
- Go: runs `task build` from the project's `Taskfile.yml`. The Go API template writes `bin/server`; the Go library template compiles packages with `go build ./...`. Older libraries can add that task to their Taskfile.

Go projects require a Taskfile. Full workspace builds skip projects without a build task and report `no-build-task`; explicitly selecting one fails with `RUNTIME_TASK_NOT_FOUND`. Invalid configuration and missing required files fail before preparation. A workspace with no build tasks also fails. Build scripts control artifact locations.

## Ordering and execution

Full builds run sequentially, with local Node dependencies before their consumers. Dependencies, devDependencies, and optionalDependencies are matched by package name, or by directory for `file:` / `link:` dependencies. Independent projects retain manifest traversal order. Duplicate package names and dependency cycles are reported before execution. Go resolves its package dependencies through the Go toolchain.

The first failure stops the build. Remaining tasks are reported as `not_run`, and the failing child's exit code is preserved. Ctrl+C stops the active process tree. Each project's logs carry its name.

## Tools, dependencies, and environments

Uses the same automatic mise/builtin selection and dependency preparation as `one dev`, including libraries without dev commands. Node dependencies are installed once at the workspace root. Each build runs through the `one run` environment-loading and PATH rules, in its project directory. `--env` selects an environment; otherwise the manifest default applies.

`--dry-run` reads configuration and reports ordered commands, directories, and skipped projects. It does not install tools or dependencies, load secrets, access the network, or write files.

## Output

`-o json` and `-o yaml` return `one-cli/build-plan/v1` for previews and `one-cli/build-result/v1` for execution results. Process logs go to stderr, leaving stdout parseable. Results include per-project status, command, duration, and exit code. Preparation failures include an error and leave build tasks `not_run`.

Use [`one container build`](/docs/container/) to build container images and [`one run`](/docs/run/) for custom commands.
