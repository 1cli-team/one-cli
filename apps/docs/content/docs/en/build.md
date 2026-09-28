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
one build --cache off --force
one build -p web --ui raw
```

Node commands come from `package.json` and use pnpm. Go commands come from Taskfile. Full builds include available build tasks; explicitly selecting a project without one fails. Local Node dependencies build before their consumers. Default concurrency is one; `--concurrency 4` allows independent tasks to run in parallel.

The Dashboard displays the project build command and its source. Edit the source file to change it. Workspace dependencies and cache settings live in mise configuration. See [one run](/en/docs/run/) for task management, custom caching, terminal modes, and Actions examples.
