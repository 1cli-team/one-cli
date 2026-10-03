---
title: one serve
description: Local Dashboard for workspaces, login, and shared credentials.
---

```bash
one serve
one serve --port 0 --open=false
```

Run `one serve` from any directory. Inside a workspace or its subdirectories, One finds the nearest `one.manifest.toml`, registers that workspace, and opens its page. From an ordinary directory, it opens the local workspace list. Repeated launches update the existing entry.

`one create` automatically registers new workspaces. Registration is stored locally in `~/.config/one/workspaces.json`, respecting `XDG_CONFIG_HOME`. You can then start Dashboard from any directory to access those workspaces.

The server only binds to loopback addresses. The sidebar contains Workspaces, Shared credentials, and Settings. All pages share the current Infisical account. Settings supports browser login, pending state, reopening, cancellation, logout, and custom instances.

Shared credentials can initialize a default location, create a Secret Manager project, or select an existing project, then browse its environments and folders. Lists contain metadata only. Reveal, copy, edit, and delete are explicit actions; remote writes are immediate and deletion identifies the complete target scope.

Project pages show project information followed by environment configuration. Workspace Infisical project bindings and project configuration use one reviewed Manifest draft. Remote values never enter the Manifest draft or repository. Browsing an environment does not change the default.

See [login and local settings](/en/docs/login/) for CLI usage and security boundaries.

## Development services

Start development services in a terminal with `one dev -p <project> --env <environment>`. The terminal provides command input, output, and process control.
