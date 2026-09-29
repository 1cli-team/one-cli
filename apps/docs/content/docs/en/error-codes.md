---
title: Error Codes
description: One CLI error-code reference for code, context, and remediation handling.
---

import { Callout } from "fumadocs-ui/components/callout";

<Callout type="info">
This page mirrors the generated Chinese reference from `internal/platform/errors/codes.go`. When adding codes, update the source registry and regenerate the docs.
</Callout>

## What This Is

Every failing `one` command emits a **structured error envelope**:

```json
{
  "schema": "one-cli/error/v1",
  "error": {
    "code": "TEMPLATE_NOT_FOUND",
    "message": "...",
    "context": { "available_templates": ["nestjs-api", "go-api", "..."] },
    "remediation": [
      {
        "action": "use-different-template",
        "hint": "Use a template from the registry",
        "command": "one add nestjs-api --name api"
      }
    ]
  }
}
```

Field meanings:

- **`error.code`**: stable routing identifier. Agents should branch on code, not message text.
- **`error.context`**: structured data from the failure site. It often already contains the data needed for recovery.
- **`error.remediation`**: recovery actions. Each item has `action`, `hint`, and sometimes `command`; agents should prefer these before guessing.

## Generic / Lifecycle

Command-level failures, user cancellation, and internal serialization failures.

### `ONE_CLI_ERROR`

Generic CLI failure with no more specific code. Inspect `context`.

### `OUTPUT_MARSHAL_FAILED`

Internal JSON/YAML marshal failure; should not happen in normal use.

### `PROMPT_CANCELLED`

User cancelled a terminal interaction with Ctrl-C / ESC. Treat as graceful cancellation.

### `UNKNOWN_COMMAND`

Positional argument did not match a command. Run `one --help`.


## Workspace / Project

Workspace detection, naming rules, and target-directory conflicts.

### `EXISTING_TARGET_NOT_EMPTY`

The target contains files or uncommitted Git deletions. Check the reported conflicts and choose an empty directory or an empty Git repository.

### `INVALID_NAME`

Project / subproject name does not match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. Use kebab-case or replace spaces.

### `INVALID_WORKSPACE_ROOTS`

`one.manifest.json#workspace.roots` is malformed. Inspect the manifest.

### `NODE_VERSION_UNSUPPORTED`

Local Node.js is too old. Upgrade to Node.js 18+.

### `NOT_ONE_PROJECT`

Current directory has no `one.manifest.json`. Run `one create <dir>` or `cd` to a workspace.

### `PROJECT_NAME_REQUIRED`

Non-interactive create was called without a workspace directory. Pass `one create <workspace-directory>`.

### `TARGET_EXISTS`

Subproject directory already exists. Pick another `--name`.

### `WORKSPACE_NESTED_FORBIDDEN`

Refused to create a workspace inside another workspace. Use `one add` or create elsewhere.


## Manifest

`one.manifest.json` shape, missing file, or empty project registry.

### `MANIFEST_INVALID`

Manifest is malformed. Fix JSON and schema fields.

### `MANIFEST_MISSING_OR_EMPTY`

Manifest is missing or has no projects. Add one with `one add <template-id> --name <project-name>`.


## Template / Registry

Template registry download, parsing, and lookup.

### `NO_TEMPLATES`

Registry is empty. This is usually a registry packaging issue.

### `REGISTRY_FETCH_FAILED`

Registry download failed. Check network and registry URL from `context`.

### `REGISTRY_INVALID`

Registry JSON is malformed.

### `REGISTRY_NOT_FOUND`

Registry path does not exist.

### `SUBPROJECT_NAME_REQUIRED`

Non-interactive add was called without `--name`.

### `TEMPLATE_NOT_FOUND`

Template ID is not in the registry. Read `available_templates` from `context` or run `one templates -o json`.

### `TEMPLATE_REQUIRED`

Non-interactive add was called without a template ID.


## Workspace Post-sync

Failures after manifest write, usually from per-domain backend sync during `create` or `add`.

### `STATUS_FIX_FAILED`

A backend sync failed or rolled back after manifest write. Re-run the command after fixing the surfaced cause.


## Backends and workspace configuration

Backend selection and generated workspace configuration.

### `LOCAL_ORCH_PORT_CONFLICT`

Two projects requested the same dev port and the runner could not auto-allocate another.

### `PREFERENCES_INVALID`

The requested local preference value is not supported.

### `PREFERENCES_FILE_INVALID`

The local preferences file could not be read or parsed.

### `RELEASE_FLOW_MISMATCH`

Release-flow backend expected a toolchain or repo state that the workspace does not have.


## Env Input Validation

Provider-agnostic `one env` input validation and migration conflicts.

### `ENV_INVALID_ENV_NAME`

Environment name does not match the allowed pattern. Use names like `dev`, `staging`, `prod`.

### `ENV_INVALID_KEY`

Env var key is invalid. Use names like `DATABASE_URL`.

### `ENV_KEY_NOT_FOUND`

Requested key does not exist at the selected path/environment.

### `ENV_PROFILE_NOT_FOUND`

Requested environment profile is missing or empty.

### `ENV_PULL_CONFLICT`

Reserved error code from the removed local environment-file workflow.

### `ENV_SET_KEY_REQUIRED`

`one env set` was called without a key.

### `ENV_SET_OVERWRITE_REQUIRED`

Variable already exists with a different value. Add `--yes` to confirm.

### `ENV_SET_VALUE_REQUIRED`

Non-interactive `env set` was called without a value.

### `ENV_UNKNOWN_ENVIRONMENT`

Env name is not registered. `set` can create it; read commands require it to exist first.

### `ENV_BACKEND_INVALID`

The Infisical binding is invalid.

### `ENV_BACKEND_UNCHANGED`

Reserved error code from the removed local environment-file workflow.

### `ENV_MIGRATE_CONFLICT`

Reserved error code from the removed local environment-file workflow.

### `ENV_MIGRATE_PARTIAL`

Reserved error code from the removed local environment-file workflow.

## Infisical Backend

Authentication, authorization, network, and project/folder errors from Infisical.

### `INFISICAL_API_ERROR`

Infisical returned an API error. Check status and context.

### `INFISICAL_AUTH_FAILED`

The browser session was rejected or expired. Run `one logout`, then `one login`.

### `INFISICAL_AUTH_MISSING`

There is no active browser session. Run `one login`.

### `INFISICAL_FOLDER_NOT_FOUND`

Target Infisical folder path does not exist.

### `INFISICAL_NETWORK_ERROR`

Could not reach Infisical. Check network and site URL.

### `INFISICAL_NOT_CONFIGURED`

This workspace has no Infisical project binding. Save the first variable with `one env set <KEY>` or Dashboard Save to create one. Reading, refreshing, and deleting do not initialize storage.

### `INFISICAL_PROJECT_CREATE_FORBIDDEN`

The signed-in account cannot create projects. Ask an administrator for permission or connect an accessible project in Dashboard settings.

### `INFISICAL_PROJECT_NAME_TAKEN`

Creation still failed after retrying name conflicts with short suffixes. Set a different `env.projectName` in `one.manifest.json` and retry. One never connects to an existing project by matching its name.

### `INFISICAL_PROJECT_NOT_FOUND`

Project ID is missing, inaccessible, or deleted.


## Misc

Codes that do not fall under the primary groups.

### `BACKEND_ID_UNKNOWN`

Manifest references a backend id this build does not recognize.

### `BACKEND_INTERFACE_MISMATCH`

Internal backend capability assertion failed. This is a build-side bug.

### `BACKEND_INVOKE_FAILED`

Backend invocation returned an error. Read context.

### `BACKEND_NOT_ENABLED`

Domain command was invoked where that domain is not configured. Add a template or configure the domain.

### `BACKEND_VERB_NOT_SUPPORTED`

Active backend does not implement this verb. Switch to a compatible backend.

### `DOMAIN_INVALID`

Domain name is not recognized.

### `DOMAIN_NOT_PER_SUBPROJECT`

This domain is workspace-scoped; drop `-p / --project`.

### `DOMAIN_NOT_REGISTERED`

Domain is recognized but no backend implementation is registered.

### `DOMAIN_REQUIRED`

Required domain section is missing from the manifest.

### `PATCH_CONFLICT`

Two backend patches conflict on the same target.

### `RUN_COMMAND_NOT_FOUND`

`one run` child command was not found.

### `RUN_DOTENV_MISSING`

Reserved error code from the removed local environment-file workflow.

### `SERVE_BIND_FORBIDDEN`

`one serve` only allows loopback bind. Use `127.0.0.1` or SSH forwarding.

### `SERVE_PAYLOAD_INVALID`

`one serve` request body is invalid JSON or missing fields.

### `SERVE_PORT_BUSY`

Requested serve port is busy. Choose another or use `--port 0`.

### `SERVE_REPOSITORY_READ_ONLY`

The requested repository mutation is not allowlisted. Use the reviewed Manifest draft for supported fields; edit source files through your normal development workflow.

### `SUBPROJECT_NOT_FOUND`

`-p / --project` references a project not in `manifest.projects`.
