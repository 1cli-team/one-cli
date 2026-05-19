---
title: "Why CLI JSON Output Matters for Coding Agents"
description: "Stable JSON envelopes let agents branch on error codes and context instead of parsing human help text."
date: "2026-05-17"
author: "One CLI Team"
tags: ["json", "cli", "agent"]
---

## Human text is not a contract

Human-friendly CLI output is useful in a terminal. It is not a good automation contract. Messages change with wording, localization, and formatting. If an agent has to parse sentences to decide what failed, the integration is fragile from the start.

One CLI treats JSON output as part of the command contract. The goal is simple: agents should be able to read structured results, branch on stable fields, and report useful context back to the user.

## Error codes beat message parsing

The important field in a machine-readable error is the code, not the sentence. A message like "template not found" might become more helpful over time, or appear in another language. The code should stay stable.

That is why agent workflows should prefer:

```bash
one templates -o json
one create my-app --yes -o json
one add nextjs-app --name web --yes -o json
```

If a command fails, the agent can inspect `error.code` and `error.context`. It should not scrape `error.message` for meaning.

## Context reduces redundant probing

A good CLI error does not only say that something failed. It returns context that helps the caller recover.

For example, if a template name is wrong, the error context can include available templates. The agent can show the user valid options without running another discovery command. If a target directory exists, the context can explain the conflicting path.

This makes the agent loop shorter:

1. Run command with JSON output.
2. Read stable code and context.
3. Decide whether to recover automatically or ask the user.
4. Report the exact reason.

## JSON output is product design

JSON output is often treated as an implementation detail. For agent-facing CLIs, it is product design. It defines what the agent can trust and what the user can audit.

One CLI's command surface stays small, but the output contract gives it room to be used safely by scripts, CI jobs, and coding agents. That is the difference between a CLI that merely works in a terminal and a CLI that can participate in an AI-native workflow.
