---
title: AI-Native Governance Rules
description: "How One CLI turns agent calls, recoverable errors, and repository contracts into executable governance rules."
---

In One CLI, "AI-native" does not mean an embedded chatbot or a prompt wrapper around commands. It means a set of governance rules: agents can call the CLI directly, read structured results, recover from stable error codes, and use repository-level engineering contracts.

**For**: teams evaluating One CLI, people defining agent boundaries, and anyone checking how One CLI differs from a traditional scaffolder.

**You will learn**: what One CLI governs, what it does not govern, and which rules agents should follow inside a One workspace.

## Governance Scope

One CLI does more than generate project files. It leaves a durable governance layer inside the monorepo.

That layer covers four boundaries:

1. **Automation interface**: agents and CI read structured output instead of scraping terminal text.
2. **Error recovery**: agents recover from stable codes, context, and remediation hints.
3. **Engineering contracts**: agents read durable rules from `AGENTS.md` / `CLAUDE.md`.
4. **Permission boundaries**: local credentials, environment setup, and deployment configuration have explicit owners instead of being guessed by agents.

## Rule 1: Command Output Must Be Parseable

For agents and CI, command results should use the structured interface:

```bash
one templates -o json
```

The result uses a stable schema:

```json
{
  "schema": "one-cli/templates/v1",
  "total": 10,
  "templates": [
    {
      "id": "nestjs-api",
      "category": "backend",
      "toolchain": "node"
    }
  ]
}
```

Running `one templates` directly in a terminal still shows a human-readable table. Pipe / non-TTY output defaults to JSON. Scripts and agents should still pass `-o json` explicitly so environment changes do not affect parsing.

The governance rule is simple: **humans read text, agents read schemas**. Copy, colors, and tables can change; the JSON schema is the automation contract.

## Rule 2: Recovery Uses code / context / remediation

One CLI errors use a single envelope:

```json
{
  "schema": "one-cli/error/v1",
  "error": {
    "code": "TEMPLATE_NOT_FOUND",
    "message": "template \"api-fastify\" does not exist; run `one templates` to list templates.",
    "context": {
      "available_templates": [
        "nestjs-api",
        "go-api",
        "astro-site",
        "starlight-docs",
        "nextjs-app",
        "react-spa",
        "expo-mobile",
        "ts-library",
        "go-lib",
        "electron-app"
      ],
      "requested_template": "api-fastify"
    },
    "remediation": [
      {
        "action": "list-templates",
        "hint": "List all available template IDs",
        "command": "one templates -o json"
      }
    ]
  }
}
```

Agents should handle errors in this order:

1. Read `error.code` to classify the failure.
2. Read `error.context` before making another probe call.
3. Prefer `error.remediation[]` for recovery actions.
4. Ask a human only when required context is missing.

Do not branch on `message`. Messages are for humans and may be translated or rewritten; `code` is the stable interface.

The full catalogue is in [Error codes](/en/docs/error-codes/).

## Rule 3: Engineering Contracts Belong In The Repository

When One CLI creates a workspace or adds a template, it maintains repository-level agent guides:

- `AGENTS.md`: read by Codex and similar agents
- `CLAUDE.md`: read by Claude Code
- Each template's own `CLAUDE.md`: copied into the generated subproject with stack-specific rules

These files are not temporary prompts. They are part of the repository. An agent entering the workspace should read them before deciding how to code, run commands, install dependencies, or change configuration.

Example rules:

```text
- Do not put business logic in Controllers.
- DTOs must use class-validator decorators.
- Use HttpException subclasses; do not throw bare Error.
- Use pino logging; request traceId is injected automatically.
```

Managed blocks are refreshed by the CLI; teams can write their own conventions outside those blocks. If a managed block is wrong, fix the template or rerun the relevant One CLI workflow instead of hand-editing generated content.

## Rule 4: Configuration And Credentials Have Boundaries

One CLI can manage env, container, and deploy configuration, but agents should not handle real credentials.

Recommended boundary:

- `one configure` / `one.manifest.json` record auditable selections and profile references.
- `one serve` opens a local `127.0.0.1` configuration UI where humans enter sensitive values.
- `.env*`, private keys, and cloud tokens stay out of Git and out of reusable agent-facing docs.
- Agents can read structured state, install missing dependencies, and scaffold projects, but publishing, deletion, and credential overwrites should go through team policy or human confirmation.

This is part of governance: One CLI lets agents act, but it does not hand every permission to agents by default.

## Checklist

Ask these five questions when evaluating whether another tool is suitable for direct agent use:

1. Does pipe / non-TTY output become parseable by default?
2. Is explicit `-o json` supported?
3. Do errors have stable `code` values?
4. Do errors include `context` and `remediation`?
5. Does the repository contain durable engineering contracts that agents can read?

If all five are true, the tool is suitable for long-running agent workflows. If only one or two are true, it is a traditional CLI with some automation-friendly behavior.

## How To Verify These Rules

Use the currently installed `one` binary as the source of truth:

```bash
one templates -o json
```

Inside any One workspace, trigger a missing template to see the `one-cli/error/v1` error envelope:

```bash
one add api-fastify --name api --yes -o json
```

After adding a real template, check the workspace-level and subproject-level agent guides:

```bash
one add nestjs-api --name api --yes -o json
ls AGENTS.md CLAUDE.md services/api/CLAUDE.md
```
