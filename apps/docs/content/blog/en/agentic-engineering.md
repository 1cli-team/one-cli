---
title: "Agentic Engineering: Structured Workspaces for AI Coding Agents"
description: "Agentic engineering makes AI coding agents useful by giving them clear workspace state, repeatable commands, and fewer places to guess."
date: "2026-05-31"
author: "One CLI Team"
tags: ["agentic-engineering", "workspace", "cli"]
---

## What is agentic engineering?

Agentic engineering is the practice of designing software workflows so AI coding agents can do useful engineering work with less ambiguity. For a software team, that means treating repository shape, command contracts, project metadata, dependency boundaries, and review loops as part of the agent operating environment, not as background knowledge hidden in prompts.

For software teams, the important shift is operational. An agent that edits code needs the same facts a teammate would need: what kind of workspace this is, which project owns the change, how dependencies are installed, which commands verify the result, and what output means success or failure. If those facts live only in habit, README prose, or tribal memory, every agent run starts with reverse engineering.

## Coding agents need a workspace contract

AI coding agents can inspect files, but inspection is not the same as understanding intent. A repository might have `apps/web`, `services/api`, and `packages/ui`, but those folders do not prove which package manager owns the workspace, which templates created the projects, or which commands are safe to run from each directory.

A structured workspace gives the agent a contract before it changes anything:

- The workspace root is clear.
- Project names and paths are explicit.
- Project types and toolchains are recorded.
- Dependency setup follows the project type.
- Build, test, dev, and env commands have stable entry points.
- Machine-readable output exists for automation.

That contract reduces the number of guesses in the agent's plan. It also gives reviewers a better audit trail because the agent can report the same facts the CLI and manifest recorded.

## Manifest-driven workflows reduce ambiguity

Manifest-driven CLI workflows turn project setup into durable state. Instead of asking an agent to infer everything from folder names, the workspace records decisions in a file that both humans and tools can read.

In a One CLI workspace, that file is [`one.manifest.json`](/en/docs/manifest/). It records the workspace identity, projects, template origins, toolchains, environments, and project-level domains. The point is not to add ceremony. The point is to make intent explicit enough that later commands and agent sessions can start from facts.

This matters most when the work is repetitive:

- Adding a frontend, backend, or package should update the same project registry.
- Running a command should resolve the right project directory.
- Installing dependencies should follow the recorded toolchain.
- Agent guidance should point to the same source of truth as the CLI.

When the manifest is the contract, a human can ask for a change and the agent can check the workspace before acting.

## Where One CLI fits

[One CLI](/en/) is a CLI for structured AI coding agent workspaces. It is not an AI agent framework, and it does not build agents for you. Its role is narrower: create and maintain workspaces that are easier for humans, scripts, and coding agents to operate safely.

The practical pieces are:

- [`one create`](/en/docs/create/) creates the workspace skeleton and manifest.
- [`one add`](/en/docs/add/) adds templated projects and registers them in the manifest.
- [`one run`](/en/docs/run/) resolves a project and runs a command from the right directory.
- JSON output and stable error codes make command results easier for agents to parse.
- [`one skills install`](/en/docs/skills/) installs One CLI's operating guidance into supported coding agents.

That combination supports agentic engineering by making the workspace less implicit. The agent still has to reason about the task, read the code, and propose or apply changes. One CLI gives it a cleaner starting point.

## A practical One CLI workflow

A small team can use One CLI to make a new agent-ready workspace repeatable from the first command:

```bash
one create agentic-product --yes
cd agentic-product
one add nextjs-app --name web --yes -o json
one add nestjs-api --name api --yes -o json
cat one.manifest.json | jq
```

At that point, the workspace has a recorded structure. A coding agent can inspect `one.manifest.json`, see that `web` and `api` are separate projects, and avoid guessing whether a command belongs at the root or inside a subdirectory.

The next prompt to an agent can be concrete:

```text
Use the One CLI workspace manifest first.
Add a shared TypeScript package for API client types.
Run the relevant build or lint command and report exact commands.
```

The agent can then follow the same command contract a developer would use:

```bash
one add ts-library --name api-client --yes -o json
one run -p web -- pnpm lint
one run -p api -- pnpm test
```

If a command fails, JSON output and documented error codes give the agent a better recovery path than parsing localized help text. The workflow is still reviewed by humans, but it starts from a more stable set of facts.

## Start with the contract

The simplest way to improve agentic engineering is to remove avoidable ambiguity before the agent starts editing code. Give the workspace a manifest, use repeatable CLI commands, prefer machine-readable output, and make setup steps visible in the repository.

To try an agentic engineering workflow with One CLI, start with the [quick start](/en/docs/quick-start/) or walk through the [manual workspace tutorial](/en/tutorials/first-workspace/). For the agent-facing command pattern, read [Output & error codes](/en/tutorials/json-output-error-codes/).
