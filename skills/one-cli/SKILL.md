---
name: one-cli
description: Develop and manage One CLI workspaces using One Workspace Convention and the installed CLI's help. Use when creating, extending, or working in a One CLI workspace.
---

# One CLI

## Workspace structure

Follow [One Workspace Convention](https://github.com/1cli-team/one-workspace-convention)
when creating or changing the workspace structure:

- `apps/`: runnable applications.
- `services/`: backend services and workers.
- `packages/`: shared libraries and reusable modules.

## Use the installed CLI

Use One CLI for workspace operations it supports. Discover the current commands
with `one --help`; use `one help --all` when you need the complete catalogue.
Before running a command, read `one <command> --help`, then the relevant
subcommand's help if needed.

Choose commands and flags from that help. The installed CLI is the authority for
its current behavior; do not assume commands or options from an older version.
