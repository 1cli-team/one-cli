# Mode: Add a Project (Template Path)

Use when the user wants to add a new project to an existing
workspace using one of the bundled templates. Pick a template, render
it, infra + CI + agent docs update automatically. The `defaults` declared
on the template (e.g. `container=docker` + `deploy=kustomize` for
go-api / nestjs-api / nextjs-app) auto-enable at add time.

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
- Writes `Dockerfile` if missing
- Adds an entry to `docker-compose.yml` (if workspace had it)
- Adds Deployment + Service to `k8s/deployment.yaml` (if workspace had it)
- Writes `.github/workflows/<name>-ci.yml`
- Updates `one.manifest.json`
- Refreshes `AGENTS.md`, `CLAUDE.md`, and `.one/agents/**`

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

Expect the new project in `projects[]`. Inspect the rendered
files on disk to confirm the expected artefacts landed (Dockerfile,
`.github/workflows/<name>-ci.yml`, etc.). If something is missing,
re-run `one add` for that project — the writer is idempotent.

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
- Whether infra (Docker / K8s) integration happened
- The dev command to start it (look at the rendered `package.json`'s
  `scripts.dev` or the README in the rendered template)
