---
title: one serve
description: Dashboard has moved to One Desktop.
---

`one serve` now displays a migration message. It no longer starts an HTTP server or opens a browser. The former `--port`, `--host`, and `--open` server options have been removed.

Open One Desktop to manage local workspaces, login, variables, and shared credentials. `one create` registers new workspaces automatically; use Open workspace in the desktop app to select an existing directory. Registration remains in `~/.config/one/workspaces.json`, respecting `XDG_CONFIG_HOME`.

Terminal users continue to use `one run`, `one env`, and `one login`. The desktop app bundles a separate `one-api` backend and `one`; installing the terminal CLI is optional during first-run setup.
