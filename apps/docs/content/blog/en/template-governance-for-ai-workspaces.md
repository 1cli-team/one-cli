---
title: "Template Governance for AI-Ready Workspaces"
description: "Templates are safer for agents when they carry conventions, dependency rules, and generated guidance together."
date: "2026-05-17"
author: "One CLI Team"
tags: ["templates", "governance", "agent"]
---

## Templates are policy, not only files

A template is often treated as a bundle of starter files. In an AI-ready workspace, a template also carries policy: how dependencies are installed, how environment variables are documented, where generated files end, and where business code begins.

If that policy is only implied by file layout, agents have to infer it. If the policy is part of the template and manifest contract, agents can check it.

## Governance starts at creation time

One CLI templates are meant to make the first project consistent with later operations. That means `one create` and `one add` should not only write files. They should also register enough structure for future commands to understand what was created.

Good template governance answers:

- What category does this project belong to?
- Which runtime and package manager does it expect?
- What default commands are safe to run?
- What environment variables need user-owned values?
- Which files are generated guidance and which are application code?

These details help humans, but they are especially important for coding agents.

## The agent should not invent conventions

When an agent opens a generated workspace, it should not need to invent the workflow. It should read the manifest, follow the bundled skill, and use the documented commands.

That means template governance has to be boring and explicit. A generated Next.js app, Go API, or documentation site should carry enough metadata for the CLI and the agent to reason about it later.

```bash
one templates -o json
one add go-api --name api --yes -o json
```

The template name, project name, and command output become part of an auditable setup path.

## Why this matters over time

The value of governance grows after the repository changes hands. A new teammate or agent session can inspect the workspace and recover the original intent. That reduces onboarding friction and lowers the risk of local setup mistakes.

Template governance is not about making scaffolding heavier. It is about making generated workspaces durable enough for repeated human and agent operation.
