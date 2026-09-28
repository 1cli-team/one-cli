---
title: one serve
description: Local Dashboard for workspaces, login, and shared credentials.
---

```bash
one serve
one serve --port 0 --open=false
```

The server only binds to loopback addresses. The sidebar contains Workspaces, Shared credentials, and Settings. All pages share the current Infisical account. Settings supports browser login, pending state, reopening, cancellation, logout, and custom instances.

Shared credentials can initialize a default location, create a Secret Manager project, or select an existing project, then browse its environments and folders. Lists contain metadata only. Reveal, copy, edit, and delete are explicit actions; remote writes are immediate and deletion identifies the complete target scope.

Workspace overview shows each build command and its configuration source. Workspace Infisical project bindings and project configuration use one reviewed Manifest draft. Remote values never enter the Manifest draft or repository. Browsing an environment does not change the default.

See [login and local settings](/en/docs/login/) for CLI usage and security boundaries.

## Development services and console

Run `one serve` in a terminal, then open a project's **Run** tab to start, stop, or restart its development service. Startup uses the effective `one dev` task, tool and dependency preparation, and Infisical variables for the selected environment. Each project has one instance; different projects can run independently. Save or discard pending Manifest drafts first.

The console streams output with known secret values redacted, with ANSI colors, search, copy, and paused scrolling. It retains up to 256 KiB / 2,048 chunks. Refreshing reconnects; closing the page leaves services running. Interactive or raw tasks still need an external terminal; this console does not accept command input.

Local HTTP(S) addresses are detected from logs. An optional URL can also be set in Overview (`projects[].dev.url`), without credentials, query parameters, or fragments. Access links become available when the port is reachable; this is a connectivity check, not an application health check. Services without a web page can run too.

The environment selector affects the next start or restart; existing instances retain their environment. Stopping cleans up the process tree. Ctrl-C stops the Dashboard and every service it started. State and logs are held in memory; restarting the Dashboard does not restore services.
