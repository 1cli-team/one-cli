---
title: "What the one-cli Skill Should Tell Coding Agents"
description: "One CLI exposes one bundled skill that records command contracts, preset vocabulary, and dependency bootstrap rules."
date: "2026-05-12"
author: "One CLI Team"
tags: ["skill", "codex", "dependencies"]
---

## A skill is not marketing copy

The bundled `one-cli` skill is meant for local coding agents such as Codex, Claude Code, and Cursor. It should not repeat the homepage pitch or read like an abstract architecture overview. The valuable part is executable guidance: when to read the manifest, when to call `one templates -o json`, when dependencies can be installed, and when the agent must stop for user input.

For an agent, the skill reduces guessing. It turns implicit project conventions into concrete operating steps.

## Three categories of context

The first category is the command contract. One CLI commands support JSON envelopes, and errors include stable `error.code` values. The skill should tell agents to use `-o json` and read `error.context` rather than parsing free-text messages.

The second category is the workspace source of truth. A directory is a One workspace when it contains `one.manifest.json`. Agents should not infer the root from `apps/`, `packages/`, or package.json because those structures vary by template combination.

The third category is dependency bootstrap. JS, TS, and Node projects install from the workspace root with the declared package manager. Go projects run module commands inside each Go subproject. This difference must be explicit, otherwise agents will run install commands in the wrong directory.

## Why One CLI exposes one skill

Splitting every command into its own skill looks granular, but it pushes more decisions to the entry point. One CLI works better as one user-facing `one-cli` skill with internal workflows for bootstrap, add-feature, dependencies, and reference lookup.

That shape has two practical benefits:

- Users only need to say “use the one-cli skill”.
- The skill can share manifest, preset, template catalog, and error recovery rules across workflows.

The external surface stays small while the internal rules stay specific.

## A good agent sequence

When a user asks an agent to prepare a One workspace to run, a good sequence is:

1. Walk upward to find the nearest `one.manifest.json`.
2. Read the manifest to confirm package manager and subprojects.
3. Install JS/TS/Node dependencies at the workspace root.
4. Run `go mod download` inside each Go subproject, and use `go mod tidy` after changing imports or when module metadata needs repair.
5. Report the exact commands that were executed.

The point is not to automate everything. The point is to automate only what has a clear boundary.
