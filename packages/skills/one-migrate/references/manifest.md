# `one.manifest.json` — Schema v1 Reference for Hand-Writing

The CLI's read path is the source of truth:
`packages/cli/internal/workspace/manifest.go`. Anything that
disagrees with that file is wrong. This page is the
"copy-paste a valid manifest" cheat sheet you need when migrating —
`one create` writes a minimal manifest and `one add` extends it, but
during migration you're skipping both and have to write the same
shape by hand.

## Minimal valid manifest (after `one create`)

```json
{
  "version": 1,
  "workspace": {
    "id": "my-workspace-a1b2c3",
    "name": "my-workspace"
  },
  "projects": []
}
```

Three top-level fields are enough to make the directory a One
workspace; everything else is filled in lazily as the user adds
projects and selects backends.

## Full schema with one project entry

```jsonc
{
  "version": 1,
  "workspace": {
    "id": "my-workspace-a1b2c3",      // kebab name + 6-hex; unique
    "name": "my-workspace"             // matches ^[a-zA-Z0-9][a-zA-Z0-9_-]*$
  },
  "environments": {                    // optional; same default seed used by `one create`
    "names": ["dev", "staging", "prod"],
    "default": "dev"                   // MUST appear in names
  },
  "domains": {                         // optional; workspace-level backend selection
    "env":       { "kind": "dotenv",    "config": {} },
    "deploy":    { "kind": "kustomize", "config": {} },
    "container": { "kind": "docker",    "config": {} }
  },
  "projects": [
    {
      "name": "web",                          // ^[a-zA-Z0-9][a-zA-Z0-9_-]*$
      "relativeDir": "apps/web",              // MUST start with apps/, services/, or packages/
      "templateId": "nextjs-app",             // from `one templates -o json`, or "" if unknown
      "toolchain": "node",                    // "node" or "go"
      "buildVersion": "",                     // tag stamped by `one container build`; leave empty
      "packageManager": "pnpm",               // "pnpm" for node; omit or "" for go
      "domains": {                            // optional per-project overrides
        "env": {
          "path": "apps/web/.env",
          "inherits": true,
          "disabled": false,
          "keys": ["DATABASE_URL", "NEXT_PUBLIC_API_URL"]
        },
        "container": {
          "image": "registry.example.com/team/web:0.1.0",
          "namespace": "team"
        },
        "deploy": {
          "kind": "vercel",
          "config": { "projectId": "prj_xxx", "env": "prod" }
        }
      }
    }
  ]
}
```

## Field-by-field

### Top level

| Field | Required | Type | Notes |
|---|---|---|---|
| `version` | **yes** | `number` | Must be `1`. Anything else is rejected at read time (`manifest.go`). |
| `workspace` | yes | object | Identity block. |
| `environments` | no | object | Safe to omit during migration; add it if you need default env resolution immediately. `one env set --env <name>` can append entries later. |
| `domains` | no | object | Workspace-level backend selection (env / deploy / container). Safe to omit unless you already know the workspace default backends. |
| `projects` | yes | array | May be empty `[]`. CLI sorts by `relativeDir` on save. |

### `workspace`

| Field | Required | Type | Notes |
|---|---|---|---|
| `id` | yes | `string` | Stable identifier. Convention: `<kebab-name>-<6-hex>`. Generate once; never change. |
| `name` | yes | `string` | Must match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. |

### `environments` (optional)

| Field | Required | Type | Notes |
|---|---|---|---|
| `names` | when present | `string[]` | Default seed when written: `["dev","staging","prod"]`. |
| `default` | when present | `string` | Must be one of `names`. Used when `--env` is omitted. |

If you omit `environments` entirely, env commands still work when the
user passes `--env`; add the block manually when the migrated workspace
needs a stable default environment from day one.

### `domains` (optional)

Each sub-key (`env`, `deploy`, `container`) is a `BackendRef`:

| Field | Required | Type | Notes |
|---|---|---|---|
| `kind` | yes (within the ref) | `string` | E.g. `"dotenv"`, `"infisical"`, `"kustomize"`, `"docker"`, `"vercel"`, `"aws-s3"`. Run the relevant `one <domain>` command to see the supported kinds. |
| `config` | no | `object` | Kind-specific blob; decoded by the per-domain accessors. Safe to leave `{}` and let the CLI populate later. |

**During migration, omit `domains` unless you know the intended
workspace defaults.** Template-based `one add` and backend-selection
commands can stamp these later; writing them by hand is error-prone
unless you are deliberately pinning env / deploy / container defaults.

### `projects[]`

| Field | Required | Type | Notes |
|---|---|---|---|
| `name` | yes | `string` | Same regex as workspace name. Must be unique within the workspace. |
| `relativeDir` | yes | `string` | POSIX path, starts with `apps/`, `services/`, or `packages/`. CLI normalises to forward slashes on save. |
| `templateId` | yes | `string` | Must be in `one templates -o json` or `""` if no match (legal, just disables template-driven behaviors). |
| `toolchain` | yes | `string` | `"node"` or `"go"`. |
| `buildVersion` | yes | `string` | Empty string `""` is correct for new entries. `one container build` stamps this. |
| `packageManager` | no | `string` | `"pnpm"` for node projects; omit or `""` for go. (One CLI only supports pnpm at workspace level today.) |
| `domains` | no | object | Per-project overrides — see below. |

### `projects[].domains` (optional)

Three optional sub-blocks; their shapes differ from the workspace
`domains` because the scopes carry different state:

**`env`** — secrets override block. No `kind` field; inherited from
workspace.

| Field | Type | Notes |
|---|---|---|
| `path` | `string` | Where `one env pull` materialises the `.env`. Defaults to `<relativeDir>/.env`. |
| `inherits` | `bool` | Whether secrets from parent folders cascade. Default true. |
| `disabled` | `bool` | Skip this project entirely on `env pull`. Default false. |
| `keys` | `string[]` | Sorted union of variable names ever set. Maintained by `one env set`; leave `[]` during migration. |

**`container`** — Dockerfile override. No `kind`; Docker is the only
supported container backend today.

| Field | Type | Notes |
|---|---|---|
| `image` | `string` | `<registry>/[<namespace>/]<workload>:<version>`. Stamped by `one container build`. |
| `profile` | `string` | Machine-level container profile name. |
| `namespace` | `string` | Registry namespace; falls through to profile default. |

**`deploy`** — full `BackendRef` (kind + profile + config). Per-project
because deploy is the one domain where projects in the same workspace
realistically choose different backends (web → s3, api → kustomize).

Field shape matches the workspace-level `BackendRef`.

## Sort order and idempotency

The CLI's read path sorts `projects[]` by `relativeDir` on save
(`manifest_roundtrip_test.go:204`). Don't rely on insertion order —
read after every write to see canonical layout.

The CLI's writer is **idempotent**: re-running `one add`, switching env
backends, or saving backend selections overwrites the relevant slice;
hand-written extras at the same keys are preserved across writes as long as they survive
the marshaller's known fields. Don't store arbitrary unrelated data
in the manifest — round-tripping may drop unknown fields.

## Generating `workspace.id`

`one create` calls `workspace.GenerateProjectID(name)`
(`packages/cli/internal/workspace/strings.go:61`): kebab-case the
name, append 6 hex chars (`crypto/rand`). When you write a manifest
by hand:

```bash
HEX=$(head -c 3 /dev/urandom | xxd -p)   # 6 lowercase hex chars
KEBAB=<workspace_name>                    # already kebab-case
ID="${KEBAB}-${HEX}"
```

Don't reuse an ID from another workspace — the field is the stable
workspace identifier used by some tooling for tagging.

## Appending a project entry with `jq`

```bash
jq --arg name "web" \
   --arg dir "apps/web" \
   --arg tid "nextjs-app" \
   --arg tc "node" \
   '.projects += [{
      name: $name,
      relativeDir: $dir,
      templateId: $tid,
      toolchain: $tc,
      buildVersion: "",
      packageManager: "pnpm"
    }]' one.manifest.json > one.manifest.json.tmp
mv one.manifest.json.tmp one.manifest.json
```

Always write to a tmp file + rename — never overwrite in place; a
botched edit halfway through leaves no manifest at all.

For a Go project: drop `packageManager`, set `"toolchain": "go"`.

## Validating after a write

```bash
cat one.manifest.json | jq '.version'              # → 5
cat one.manifest.json | jq '.workspace.name'       # → expected string
cat one.manifest.json | jq '.projects | length'    # → expected count

# CLI's own read path — runs the schema v1 check:
one templates -o json >/dev/null    # any non-mutating invocation is enough;
                                    # the CLI loads the manifest on start.
```

If `one <anycmd>` reports `NOT_ONE_PROJECT` from inside the workspace
dir, the manifest is missing or in the wrong place — the file must be
at the directory the user runs `one` from (or an ancestor).
