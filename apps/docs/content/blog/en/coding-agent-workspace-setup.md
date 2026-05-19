---
title: "How Coding Agents Should Prepare a Workspace to Run"
description: "A safe workspace setup flow starts from the manifest, installs only missing dependencies, and reports exact commands."
date: "2026-05-17"
author: "One CLI Team"
tags: ["agent", "dependencies", "workspace"]
---

## Start from the workspace contract

When a coding agent is asked to prepare a project to run, the first step should not be a package install. The first step is finding the workspace contract. In a One CLI workspace, that contract is `one.manifest.json`.

The manifest tells the agent whether it is inside a One workspace, which package manager belongs at the root, and which subprojects exist. That is safer than guessing from `apps/`, `services/`, or package scripts.

## Install by toolchain, not habit

The common mistake is to run one install command everywhere. That works for small single-stack projects and fails in mixed workspaces.

One CLI's agent guidance separates dependency setup by toolchain:

- JS, TS, and Node projects install from the workspace root with the declared package manager.
- Go projects run module commands from the Go project directory.
- `go mod tidy` should be used after imports change or module metadata needs repair, not as a reflex before every read-only check.

This distinction keeps the agent from changing dependency files unnecessarily.

## A useful setup sequence

A reliable agent flow looks like this:

```bash
one templates -o json
```

Then inspect the manifest directly, choose the dependency path, and run only what is missing. For a Node workspace that uses pnpm, that usually means:

```bash
pnpm install
```

For a Go service, it usually means:

```bash
go mod download
```

The exact command should follow the workspace state, not an assumption baked into the agent prompt.

## Report the commands, not just the result

After setup, the agent should tell the user exactly what it ran. That makes the run reproducible and lets the user spot unnecessary actions.

Good setup reports include:

- The detected workspace root.
- The package manager or Go module path used.
- The exact install commands.
- Any files changed by dependency repair.
- Any command that was skipped because dependencies were already present.

This is a small discipline, but it prevents a large class of hidden local-state problems.
