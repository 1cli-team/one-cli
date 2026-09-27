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
