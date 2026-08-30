# Mode: Add a Project (Template Path)

Use when the user wants to add a new project to an existing workspace using
one of the bundled technology stacks. It renders a locally developable
project; deployment is deliberately configured later, on the first
`one deploy <project>`.

## Inputs to extract

| Field | Required | Notes |
|---|---|---|
| `template_id` | yes | Run `one templates -o json` if unsure. |
| `project_name` | yes | Must match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. |

If either is missing and you're driving non-interactively, ask one
concise question. Don't guess template IDs from fuzzy descriptions
without confirming.

## Template ID quick reference

(Authoritative source: `one templates -o json`)

| Intent | Template ID |
|---|---|
| NestJS API | `nestjs-api` |
| Go API (Gin) | `go-api` |
| React SPA | `react-spa` |
| Next.js SSR | `nextjs-app` |
| Astro SSG | `astro-site` |
| Starlight docs | `starlight-docs` |
| RN / Expo | `expo-mobile` |
| Electron | `electron-app` |
| TS library | `ts-library` |
| Go library / Go module / 共享 Go 包 | `go-lib` |

## Workflow

### Step 1 — Verify workspace state

```bash
cat one.manifest.json
```

Expectations:
- File exists at the workspace root (walk up from cwd if you're inside
  a project) — this is what makes a directory a One workspace.
- Note the `package_manager` (currently always pnpm) and existing
  `projects[]` (don't pick a name that collides).

If no `one.manifest.json` exists in any ancestor → switch to
`bootstrap.md`. The `one add` command itself will refuse with
`NOT_ONE_PROJECT` outside a workspace.

### Step 2 — Add

```bash
cd <workspace_root>
one add <template_id> --name <project_name> --yes -o json
```

Schema: `one-cli/add/v1`. The CLI does all of:
- Renders template into `apps/<name>/` (frontend) / `services/<name>/`
  (backend) / `packages/<name>/` (library), based on the template's
  category
- Does not generate CI workflow files by default
- Updates `one.manifest.json`
- Refreshes `AGENTS.md`, `CLAUDE.md`, and `.one/agents/**`
- Leaves deployment and image configuration absent until first deploy

If the user explicitly requests CI, run it as a separate step after add:

```bash
one ci enable <project_name> -o json
```

### Step 3 — Install missing dependencies

Use `dependencies.md` and choose the command by project toolchain.

Examples:

```bash
# JS / TS / Node workspace
pnpm install

# Go project
(cd services/api && go mod download)
```

Use the detected package manager for Node workspaces (`pnpm`, `npm`, or
`yarn`). Use `go mod tidy` for Go after changing imports or when `go.mod` /
`go.sum` needs repair.

### Step 4 — Verify

```bash
cat one.manifest.json
```

Expect the new project and its dev command in `projects[]`. Deployment and
container fields should be absent after an ordinary add. Use bare `one` to see
the next recommended command.

## Mode-specific error recovery

| Code | Recovery |
|---|---|
| `TEMPLATE_NOT_FOUND` | Read `error.context.available_templates`. Pick one. |
| `TEMPLATE_REQUIRED` | Pass template id positionally: `one add <id> --name <name>`. |
| `SUBPROJECT_NAME_REQUIRED` | Pass `--name <name>`. |
| `TARGET_EXISTS` | Project directory already exists — pick a different name. |

## Success response

Tell the user:
- Project name + template + target directory
- That deployment will be chosen later
- The next command: `one dev <project_name>`
