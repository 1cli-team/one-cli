---
title: one configure
description: Manage local connections and preferences for environment variables.
---

`one configure` manages **local connections and preferences**, not application code. Credentials stay on this machine and are never written to the workspace or Git.

## Usage

```bash
one configure
one configure add
one configure add <pair> --profile <name> [backend flags...] [--use]
one configure list [pair]
one configure current [pair]
one configure show <pair> --profile <name> [--reveal]
one configure use <pair> --profile <name>
one configure remove <pair> --profile <name>
one configure locale [auto|zh-CN|en-US]
one configure open
```

With no connections, bare `one configure` opens the setup wizard. With existing connections it shows a concise overview. `show`, `use`, and `remove` let terminal users select an existing connection; scripts keep explicit `<pair>` and `--profile` inputs.

## Interactive Mode

For local human setup, use the wizard:

```bash
one configure
one configure add
```

The wizard first asks which service to connect, then asks for a connection name and the required service fields. Stable service IDs remain visible in automation commands. Secret fields use password-style input.

Scripts and CI should not wait for the wizard; pass the service ID, connection name (`--profile`), and service flags explicitly.

## Supported pairs

| pair | purpose |
|---|---|
| `env/infisical` | Infisical site URL + Universal Auth client id / secret |

`env/dotenv` does not need a profile; it is for local `.env` workflows.

## Examples

```bash
one configure add env/infisical --profile work \
  --client-id "$INFISICAL_CLIENT_ID" \
  --client-secret "$INFISICAL_CLIENT_SECRET" \
  --use

```

## Resolution order

When a command needs a profile, it resolves in this order:

1. `--profile <name>`
2. Project + environment binding in `profile-bindings.json`
3. Workspace + environment binding in `profile-bindings.json`
4. legacy Project binding in `config.json#workspaces`
5. legacy Workspace binding in `config.json#workspaces`
6. `~/.config/one/config.json#domain/backend.default`

The environment-aware bindings are keyed by canonical Workspace root, environment, and `(domain, backend)`. They store only a Profile name. The Dashboard UI offers `dev`, `preview`, and `prod` through `?env=`; the core/API also accepts safe custom IDs supplied by other workflows. Global Settings Profile CRUD is not environment-scoped. An empty environment keeps the legacy chain.

`one.manifest.json` never stores a local Profile name. `one configure use ... --workspace` and `--project` remain compatible legacy bindings; use `one serve` when you need a distinct selection for each environment.

## Storage

```text
~/.config/one/
├── config.json             # non-secret Profile fields, defaults, legacy bindings
├── credentials.json        # secrets: clientId, clientSecret
├── profile-bindings.json   # v1: canonical root + environment -> Profile names
└── cache/                  # short-lived token cache
```

All three JSON files are machine-local and written as `0600`; `profile-bindings.json` contains names only. None of them modifies or upgrades `one.manifest.json`. `show` masks secrets by default; only `show --reveal` prints cleartext.

## Output schemas

| command | schema |
|---|---|
| `add` | `one-cli/configure-add/v1` |
| `list <pair>` | `one-cli/configure-list/v1` |
| `list` | `one-cli/configure-list-all/v1` |
| `current <pair>` | `one-cli/configure-current/v1` |
| `current` | `one-cli/configure-current-all/v1` |
| `show` | `one-cli/configure-show/v1` |
| `use` | `one-cli/configure-use/v1` |
| `remove` | `one-cli/configure-remove/v1` |

## Common errors

| code | fix |
|---|---|
| `PROFILE_NONE_CONFIGURED` | run `one configure add <pair> --profile <name> --use` |
| `PROFILE_NOT_FOUND` | run `one configure list <pair>` and use an existing name |
| `PROFILE_BACKEND_INVALID` | use a profile whose backend matches the target project |
| `PROFILE_FILE_INVALID` | repair the file named in the error context (`config.json`, `credentials.json`, or `profile-bindings.json`) |
| `PROFILE_VERSION_UNSUPPORTED` | upgrade One CLI or recreate only the incompatible machine-local file |

## Next

- [one serve](/en/docs/serve/) — edit Profiles and choose environment-aware local bindings
- [one env](/en/docs/env-vars/) — use `env/infisical`
