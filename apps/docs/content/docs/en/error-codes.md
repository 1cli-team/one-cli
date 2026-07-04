---
title: Error Codes
description: One CLI error-code reference for code, context, and remediation handling.
---

import { Callout } from "fumadocs-ui/components/callout";

<Callout type="info">
This page mirrors the generated Chinese reference from `internal/errors/codes.go`. When adding codes, update the source registry and regenerate the docs.
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

Create target exists and is non-empty. Pick an empty directory or remove the target manually.

### `INVALID_NAME`

Project / subproject name does not match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. Use kebab-case or replace spaces.

### `INVALID_WORKSPACE_ROOTS`

`one.manifest.json#workspace.roots` is malformed. Inspect the manifest.

### `NODE_VERSION_UNSUPPORTED`

Local Node.js is too old. Upgrade to Node.js 18+.

### `NOT_ONE_PROJECT`

Current directory has no `one.manifest.json`. Run `one create <dir>` or `cd` to a workspace.

### `PROJECT_NAME_REQUIRED`

Non-interactive create was called without a project name. Pass `one create <project-name>`.

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

### `REGISTRY_CREDENTIAL_MISSING`

Container push needs registry credentials. Use local build or configure `container/docker`.

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


## Plugin / Profile / Deploy

Backend selection, profile resolution, deployment, and generated delivery artifacts.

### `CI_PROVIDER_UNKNOWN`

Manifest references an unknown CI provider.

### `CI_RENDER_FAILED`

CI backend failed while rendering workflow files.

### `IMAGE_REF_INCOMPLETE`

Deploy / CI needs a complete image reference but registry/name/tag is missing.

### `IMAGE_TAG_NOT_FOUND`

Push target tag does not exist locally. Build first with `one container build <subproject>`.

### `IMAGE_TAG_REQUIRED`

Container build could not infer a version tag. Pass `--build-version`, set `projects[].buildVersion`, or create a Git tag.

### `K8S_PACKAGE_UNSUPPORTED`

Selected Kubernetes packaging form is not bundled in this build.

### `K8S_PLATFORM_UNDETECTED`

Kubernetes node architecture could not be detected. Check kubeconfig/context and `kubectl get nodes -o wide`.

### `LOCAL_ORCH_PORT_CONFLICT`

Two projects requested the same dev port and the runner could not auto-allocate another.

### `PROFILE_ALREADY_EXISTS`

Profile name already exists. Re-run `one configure add ... <name>` to update or choose another name.

### `PROFILE_BACKEND_INVALID`

Profile backend is not recognized or does not belong to the declared domain.

### `PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED`

Profile uses a credential source this build cannot read. Use `file` source.

### `PROFILE_FILE_INVALID`

`~/.config/one/config.json` or `credentials.json` is invalid JSON. Repair or delete and recreate profiles.

### `PROFILE_NONE_CONFIGURED`

No profile resolved from `--profile`, workspace binding, or machine default. Run `one configure add <domain>/<backend> --profile work`.

### `PROFILE_NOT_FOUND`

Requested profile does not exist. Run `one configure list <pair>` or add the profile.

### `PROFILE_VERSION_UNSUPPORTED`

Profile file schema does not match this binary. Upgrade CLI or recreate profiles.

### `RELEASE_FLOW_MISMATCH`

Release-flow backend expected a toolchain or repo state that the workspace does not have.


## Agent Docs / Skills

`AGENTS.md`, `CLAUDE.md`, `.one/agents/**`, and One CLI skill installation.

### `AI_CONFIG_INVALID`

`one.manifest.json#ai` is malformed.

### `AI_CONFIG_MISSING`

Legacy provider gate; the current CLI renders all supported providers and should not normally surface this.

### `AI_GUIDES_FAILED`

Agent docs refresh failed. Read the surfaced error.

### `AI_GUIDE_EXISTS`

Existing `AGENTS.md` / `CLAUDE.md` is user-managed and will not be overwritten.

### `AI_NO_SUBPROJECTS`

Workspace has no recognizable projects yet.

### `AI_PROVIDER_INVALID`

Unknown AI provider; supported generated guides are Codex and Claude Code.

### `SKILLS_INSTALL_FAILED`

One CLI skills could not be copied. Check permissions and rerun `one skills install`.

### `SKILLS_NOT_BUNDLED`

One CLI skill directory is missing from the package.


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

Existing `.env` differs from pulled values. Inspect first or use `--force` intentionally.

### `ENV_SET_KEY_REQUIRED`

`one env set` was called without a key.

### `ENV_SET_OVERWRITE_REQUIRED`

Variable already exists with a different value. Add `--yes` to confirm.

### `ENV_SET_VALUE_REQUIRED`

Non-interactive `env set` was called without a value.

### `ENV_UNKNOWN_ENVIRONMENT`

Env name is not registered. `set` can create it; read commands require it to exist first.

### `ENV_BACKEND_INVALID`

Env backend is not `dotenv` or `infisical`.

### `ENV_BACKEND_UNCHANGED`

Workspace is already on the requested env backend. No action needed.

### `ENV_MIGRATE_CONFLICT`

Target backend has a same-name key with a different value. Use overwrite or no-sync intentionally.

### `ENV_MIGRATE_PARTIAL`

Some keys synced and others failed. Fix the cause and retry.


## Infisical Backend

Authentication, authorization, network, and project/folder errors from Infisical.

### `INFISICAL_API_ERROR`

Infisical returned an API error. Check status and context.

### `INFISICAL_AUTH_FAILED`

Universal Auth login failed. Rotate or verify credentials.

### `INFISICAL_AUTH_MISSING`

No default Infisical credentials. Configure `env/infisical`.

### `INFISICAL_FOLDER_NOT_FOUND`

Target Infisical folder path does not exist.

### `INFISICAL_NETWORK_ERROR`

Could not reach Infisical. Check network and site URL.

### `INFISICAL_NOT_CONFIGURED`

Workspace is not configured for Infisical or has no project binding.

### `INFISICAL_PROJECT_CREATE_FORBIDDEN`

Machine identity cannot create projects. Grant permissions or create manually.

### `INFISICAL_PROJECT_NAME_TAKEN`

Desired project name already exists. Change the configured name or bind manually.

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

### `CLOUDFLARE_CLI_MISSING`

`wrangler` is missing. Install it locally or globally.

### `CLOUDFLARE_DEPLOY_FAILED`

`wrangler` failed. Check upstream logs, token, and account id.

### `CLOUDFLARE_PROFILE_INVALID`

Cloudflare profile is missing API token or required account data.

### `DOMAIN_INVALID`

Domain name is not recognized.

### `DOMAIN_NOT_PER_SUBPROJECT`

This domain is workspace-scoped; drop `-p / --project`.

### `DOMAIN_NOT_REGISTERED`

Domain is recognized but no backend implementation is registered.

### `DOMAIN_REQUIRED`

Required domain section is missing from the manifest.

### `EDGEONE_CLI_MISSING`

EdgeOne CLI is missing. Install it with npm or pnpm.

### `EDGEONE_DEPLOY_FAILED`

EdgeOne CLI failed. Check token and project configuration.

### `EDGEONE_PROFILE_INVALID`

EdgeOne profile is missing API token or required fields.

### `PATCH_CONFLICT`

Two backend patches conflict on the same target.

### `RUN_COMMAND_NOT_FOUND`

`one run` child command was not found.

### `RUN_DOTENV_MISSING`

Local dotenv file required by `one run` is missing.

### `SERVE_BIND_FORBIDDEN`

`one serve` only allows loopback bind. Use `127.0.0.1` or SSH forwarding.

### `SERVE_PAYLOAD_INVALID`

`one serve` request body is invalid JSON or missing fields.

### `SERVE_PORT_BUSY`

Requested serve port is busy. Choose another or use `--port 0`.

### `SERVE_TOKEN_INVALID`

Missing or expired local serve token. Restart and use the printed URL.

### `SUBPROJECT_NOT_FOUND`

`-p / --project` references a project not in `manifest.projects`.

### `VERCEL_CLI_MISSING`

Vercel CLI is missing. Install `vercel`.

### `VERCEL_DEPLOY_FAILED`

Vercel CLI failed. Check upstream logs, token, and project link.

### `VERCEL_PROFILE_INVALID`

Vercel profile is missing API token or team/project data.
