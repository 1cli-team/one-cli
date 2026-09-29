---
title: build task
description: Build selected projects and their local dependencies through mise.
---

`one build` is shorthand for `one run build`. `build` is an ordinary task name, just like `test` or `lint`, and uses the same task parsing, flags, and help. The effective task graph comes from mise, including your overrides in `mise.toml`.

```sh
one build
one build -p web -p api
one build -p web --env prod
one build -p web --dry-run -o json
one build --force
one build -p web --ui raw
```

Node commands come from `package.json` and use the workspace package manager (pnpm for new workspaces). Go commands come from Taskfile. Full builds include available build tasks; explicitly selecting a project without one fails. Local Node dependencies build before their consumers. Concurrency is allocated automatically from the task graph; `--concurrency 4` sets an explicit limit.

The Dashboard displays the project build command and its source. Edit the source file to change it. Workspace dependencies and sources / outputs freshness checks live in mise configuration. Artifact caching is disabled through One; `--force` bypasses freshness checks. See [one run](/en/docs/run/) for task management and terminal modes.
