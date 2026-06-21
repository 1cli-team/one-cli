# Mode: Bootstrap a New One Workspace

Use when the user wants to create a fresh One CLI workspace from
scratch. Creates the workspace, optionally adds first projects,
sets up Infisical env-var backend if asked.

## Inputs to extract

| Field | Required | Notes |
|---|---|---|
| `project_name` | yes | Must match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. Convert fuzzy names ("用户中心" → `user-center`). Ask if unclear. |
| `projects` | no | List of `{template, name}` to add post-create. |
| `needs_secrets` | no | If true, create or switch the workspace to Infisical. Requires an `env/infisical` machine profile. |
| `parent_dir` | no | Defaults to cwd. |

If anything is unclear, ask one concise clarification:

> 我可以建一个 `<name>` 工作区。哪些首批项目？是否要用 Infisical 托管环境变量？

## Template mapping (run `one templates -o json` for source of truth)

| Intent | Template ID |
|---|---|
| NestJS / Node API / 后端 API | `nestjs-api` |
| Go / Gin / Go 后端 API | `go-api` |
| React / SPA / 前端 | `react-spa` |
| Next.js / SSR / 全栈 | `nextjs-app` |
| Astro / SSG / 静态站 | `astro-site` |
| Docs / Starlight / 文档站 | `starlight-docs` |
| RN / Expo / 移动 App | `expo-mobile` |
| Electron / desktop app | `electron-app` |
| TypeScript library / npm 包 | `ts-library` |
| Go library / Go module / 共享 Go 包 | `go-lib` |

If the user wants something not in this list (Vue + Pinia, Rust, etc.),
let them know the built-in templates are the supported set and ask
whether one of the closest matches is acceptable. Custom stacks are
out of scope for `one create` / `one add`.

## Workflow

### Step 1 — Sanity check current state

Walk upward from the parent dir for a `one.manifest.json` (or just check
`<parent_dir>/one.manifest.json`). If one already exists, confirm the
user really wants a new *sibling* workspace rather than extending the
existing one. If they want to extend, switch to `add-feature.md`.

Note: `one create` will refuse with `WORKSPACE_NESTED_FORBIDDEN` if it
detects an enclosing workspace, so this check is purely to avoid a
useless command round-trip.

### Step 2 — Create the workspace

```bash
one create <project_name> --yes -o json
```

Schema: `one-cli/create/v2`. Fields:
- `created_path` — absolute path of the new workspace
- `skills.status` — "completed" / "failed"
- `skills.installed_to` — list of agent skills directories (Claude Code,
  Cursor, Codex, etc. — multi-agent install since v0.3.0)
- `skills.skill_count` — how many skills landed

If `skills.status == "failed"`, the workspace still exists; surface
the error and continue. Don't delete the workspace.

### Step 3 — Add requested projects

For each `{template, name}`:

```bash
cd <created_path>
one add <template_id> --name <project_name> --yes -o json
```

Schema: `one-cli/add/v1`. `one add` automatically:
- Renders the template into `apps/` / `services/` / `packages/`
- Applies template-declared domain defaults such as `container=docker`,
  `deploy=kustomize`, or `deploy=aws-s3`
- Generates the per-project GitHub Actions workflow
- Refreshes `AGENTS.md`, `CLAUDE.md`, and `.one/agents/**`

### Step 4 — Optional Infisical (managed env vars)

```bash
one configure add env/infisical --profile work \
  --client-id "$INFISICAL_CLIENT_ID" \
  --client-secret "$INFISICAL_CLIENT_SECRET" \
  --use
one env switch infisical -o json
```

If the user asked for Infisical before creation and the machine profile
already exists, prefer `one create <project_name> --yes --env-provider infisical -o json`.
Create-time and switch-time binding attempt to create or bind the
Infisical project. If the profile, network, or permissions are not ready,
the workspace can still exist and the next `one env set/get/list/pull`
will retry lazy binding.

After init the user populates values:

```bash
one env set DATABASE_URL "postgres://..." --env dev -o json
one env pull --env dev -o json   # writes one .env per manifest project
```

`env pull` is driven by `one.manifest.json` — every secret at the
project's Infisical folder (plus inherited parent folders) lands
in `<project>/.env`, with no `.env.example` filter. Scope keys at
the Infisical folder level if you don't want them in a particular
project.

### Step 5 — Install missing dependencies

Use `dependencies.md` and install by toolchain. For a mixed workspace,
run the Node workspace install once, then handle each Go project from
its own directory.

Examples:

```bash
# JS / TS / Node workspace
pnpm install

# Go project
(cd services/api && go mod download)
```

Use `go mod tidy` instead of `go mod download` when the agent changed Go
imports or when `go.mod` / `go.sum` needs repair. One CLI itself does not
wrap dependency installation; the bundled skill should do this setup when
it is needed for the next build / test / run step.

### Step 6 — Verify

```bash
cat one.manifest.json
```

Confirm the expected projects appear in the `projects` array and
`domains.env.kind` reflects what the user asked for. (CI workflow is
always synced; each project's dev command lives at
`projects[].domains.dev.command` and is also always written by
`one add`.) If something is missing (e.g. a project's container
section is absent), re-run the relevant `one add` for that project;
manifest write is idempotent.

## Mode-specific error recovery

(See main `SKILL.md` for cross-mode codes.)

| Code | Recovery |
|---|---|
| `EXISTING_TARGET_NOT_EMPTY` | Target exists and is non-empty. Ask the user to pick a different directory or remove the existing one. `create` no longer offers `--overwrite` / `--ignore`. |
| `INFISICAL_AUTH_MISSING` | Tell user to create an Infisical Universal Auth identity, then run `one configure add env/infisical --profile <name> --client-id <id> --client-secret <secret> --use`. |
| `INFISICAL_AUTH_FAILED` | Bad client id/secret or rate limit. Rotate the secret in the Infisical web UI. |
| `INFISICAL_PROJECT_NOT_FOUND` | Project id wrong, or identity has no access. User checks both. |
| `SKILLS_INSTALL_FAILED` | Workspace still exists; check `~/.claude/skills/` perms (or other detected agent paths). |

## Success response

Reply in the user's language. Include:
- Created path
- Projects added (template + name pairs)
- Which template-driven deploy / container domains were enabled
- Whether skills installed and to which agent paths
- Whether Infisical was selected / switched successfully
- Next command if anything was skipped (e.g. `pnpm install`)
