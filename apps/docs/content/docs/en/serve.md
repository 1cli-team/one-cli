---
title: one serve
description: Local Dashboard for Workspaces, Projects, and machine-level Profiles.
---

`one serve` starts a local Dashboard bound only to `127.0.0.1` and opens a browser. The Dashboard lists Workspaces observed on this machine, lets you switch between them, stages and reviews Workspace environment Backend and Project configuration changes, manages Infisical secrets, and manages the machine-level Profiles used by `one configure`.

Why not let AI edit the Profile files directly: they contain API keys, kubeconfig paths, and registry tokens. The risk of leaking them is higher than the value of saving a few manual inputs. `one serve` physically keeps those fields out of command-line and agent context.

## Usage

```bash
one serve [options]
```

The process blocks in the foreground. Press Ctrl-C to stop. Workspace environment Backend and Project configuration changes remain in a browser draft until the top-bar save action displays an exact diff and the user confirms. Project changes use an atomic, revision-checked Manifest patch; Backend changes use the revision-checked env switch workflow. Selecting Infisical initializes and persists the Workspace's Infisical project binding, but does not migrate existing secret values between providers. Source files remain read-only. Profile mutations share `~/.config/one/{config,credentials}.json` with `one configure`; Workspace/Project selections write only Profile names to `~/.config/one/profile-bindings.json`.

## Arguments

| Argument | Description |
|---|---|
| `--host <host>` | Bind host. Only loopback is accepted (`127.0.0.1`, `localhost`, `::1`). Non-loopback returns `SERVE_BIND_FORBIDDEN` |
| `--port <n>` | Listen port. Default `0` lets the kernel pick a free port |
| `--open` | Open browser after startup. Default `true`; pass `--open=false` for CI, headless, WSL, or remote SSH |
| `-o, --output <fmt>` | `json` / `yaml` / `text`; default is TTY-aware auto detection |

## Interactive Mode

`one serve` has no terminal wizard. Browser forms can stage the Workspace environment Backend plus allowlisted Project runtime, environment, container, and deployment settings. A single confirmation publishes the collected Manifest draft. Profile bindings remain separate machine-local saves. When the Workspace uses `env/infisical`, the Dashboard can list key names and create, reveal, update, or delete one remote value at a time.

For local human setup, run `one serve`. Scripts, CI, and agents can use `--open=false` to receive the plain loopback URL and call the API directly. Because the API can read and mutate sensitive configuration, do not run it on a shared machine with untrusted local processes.

## Workspace Discovery And Persistence

One CLI records a Workspace in the machine-local list in two cases:

- after `one create` completes successfully;
- when `one serve` runs from the Workspace root or any descendant directory.

The XDG-aware registry lives at `~/.config/one/workspaces.json`. It stores only a local entry ID, Manifest Workspace ID, name, canonical absolute root, and observation timestamps. It does not copy Projects, Backend settings, Profiles, or credentials. An unavailable directory remains visible as missing; Forget removes only the local registration and never deletes the directory, Manifest, Profiles, or credentials.

Running `one serve` outside a Workspace still opens the historical list. The Dashboard selects the launch Workspace first, or the most recently seen ready Workspace when there is no current one.

## Environment Selection And Local Storage

The Dashboard selector exposes exactly Development (`?env=dev`), Preview (`?env=preview`), and Production (`?env=prod`); unknown UI query values normalize to Development. Selecting one does not add it to the Manifest or upgrade the Manifest schema. The core/API store can also represent safe custom IDs such as `staging` when another CLI/API workflow supplies them.

Global Settings hides the environment selector because Profile definitions and CRUD are machine-global, not environment-scoped. Links preserve the query so returning to a Workspace or Project keeps its previous binding context.

```text
~/.config/one/
├── config.json             # Profile names, non-secret fields, defaults, legacy bindings
├── credentials.json        # Profile credentials
├── profile-bindings.json   # v1: canonical root + environment -> Profile names only
└── workspaces.json          # observed Workspace registry
```

`profile-bindings.json` is a machine-local v1 store written as `0600` with atomic replacement. Its canonical-root key keeps two copies of the same repository independent even if both copies contain the same Manifest Workspace ID. It contains no credential values and never writes inside either repository.

For a `(domain, backend)`, effective Profile resolution is:

1. one-shot `--profile` flag;
2. Project + environment binding;
3. Workspace + environment binding;
4. legacy Project binding in `config.json`;
5. legacy Workspace binding in `config.json`;
6. machine default.

## Output

After binding, stdout emits one startup envelope and then blocks:

```json
{
  "schema": "one-cli/serve/v2",
  "status": "listening",
  "url": "http://127.0.0.1:54321/",
  "host": "127.0.0.1",
  "port": 54321
}
```

The startup URL contains no login information and the API does not use a session token. The service disappears when the process exits. If another `one serve` process later reuses the same port, the old URL points to that new local process.

## Security Model

`one serve` owns profile files, and profile files own credentials, so this local service is a sensitive interface. It trusts the machine boundary and performs no session-level authentication; any local process that can reach the loopback port can call the API. These defenses remain in place:

| Layer | Threat blocked | Behavior |
|---|---|---|
| Host header check | DNS rebinding, where an attacker domain resolves to `127.0.0.1` | `Host` must match the bound `127.0.0.1:<port>` or `localhost:<port>`, otherwise `421 Misdirected Request` |
| Origin check for mutations | Cross-origin POST / script requests | POST/PUT/DELETE `Origin` must equal the service origin, otherwise `403 Forbidden` |
| Typed repository publishers | Stale or over-broad repository writes | Project patches and env Backend switches use separate allowlisted endpoints; the exact base revision must match or `SERVE_MANIFEST_CONFLICT` is returned |
| Legacy route boundary | Stale clients attempt former settings PUT routes | Former mutation paths return `409 SERVE_REPOSITORY_READ_ONLY` |

Credentials are **masked by default**. `GET /api/configure*` returns values such as `clientSecret: "********"`, `accessKeySecret: "********"`, and `password: "********"`. The UI's reveal button calls `?reveal=1` to fetch cleartext. Infisical lists contain key names only; a single value is retrieved on demand with `Cache-Control: no-store` and kept out of SWR caches. Workspace/Project projections expose only a resolved Profile name and source, never Profile fields or credentials.

Out of scope:

- Multi-user access
- `0.0.0.0` / LAN exposure; `SERVE_BIND_FORBIDDEN` refuses it
- Live push when external processes edit profile files; refresh the browser after `one configure ... add`

## Examples

### Default: Random Port + Auto-open Browser

```bash
one serve
# profile UI started: http://127.0.0.1:54321/
# Browser opens automatically; Ctrl-C exits
```

### CI / Headless / WSL: Print URL Only

```bash
one serve --open=false
```

### Fixed Port For Testing Or Screenshots

```bash
one serve --port 17900
```

### Container / Remote SSH

`one serve` binds to `127.0.0.1`. For a remote machine, use SSH port forwarding:

```bash
# remote
one serve --open=false --port 17900

# local
ssh -L 17900:127.0.0.1:17900 remote-host
# Open the URL printed on the remote side, replacing the host with 127.0.0.1
```

Do not try `--host 0.0.0.0`; it is rejected with `SERVE_BIND_FORBIDDEN`.

## REST API

The web UI uses these same routes. All routes require a matching Host header; mutating routes also require a matching Origin header. No token is required.

| Method | Path | Meaning | Response schema |
|---|---|---|---|
| `GET` | `/api/configure` | All profile sections | `one-cli/serve-configure-config/v1` |
| `GET` | `/api/configure/{domain}/{backend}` | One section; `?reveal=1` returns cleartext | `one-cli/serve-configure-section/v1` |
| `POST` | `/api/configure/{domain}/{backend}` | Upsert body `{name, profile, use?}` | `one-cli/serve-configure-upsert/v1` |
| `DELETE` | `/api/configure/{domain}/{backend}/{name}` | Remove profile | `one-cli/serve-configure-remove/v1` |
| `PUT` | `/api/configure/{domain}/{backend}/default` | Set default profile with body `{name}` | `one-cli/serve-configure-use/v1` |
| `GET` | `/api/workspaces` | Machine-local Workspace list and live status | `one-cli/workspaces/v1` |
| `DELETE` | `/api/workspaces/{entryId}` | Forget a registration without deleting the Workspace | No body |
| `GET` | `/api/workspaces/{entryId}/overview` | Selected Workspace and Project overview | `one-cli/workspace-overview/v1` |
| `GET` | `/api/workspaces/{entryId}/profile-bindings/env?env={environment}` | Effective Workspace env Profile name/source | `one-cli/workspace-profile/v1` |
| `PUT` | `/api/workspaces/{entryId}/profile-bindings/env?env={environment}` | Select/unselect Workspace env Profile; body `{profile}` | `one-cli/workspace-profile/v1` |
| `PUT` | `/api/workspaces/{entryId}/environment/backend?env={environment}` | Revision-checked env Backend switch; body `{revision, backend}` | `one-cli/workspace-profile/v1` |
| `POST` | `/api/workspaces/{entryId}/environment/backend/initialize?env={environment}&project={name?}` | Repair a missing Infisical project binding | `one-cli/workspace-profile/v1` |
| `GET` | `/api/workspaces/{entryId}/projects/{name}?env={environment}` | Project/config projection, Manifest revision, and effective Profile names | `one-cli/workspace-project/v1` |
| `PUT` | `/api/workspaces/{entryId}/projects/{name}/profile-bindings/{domain}?env={environment}` | Select/unselect Project Profile; body `{profile}` | `one-cli/workspace-project/v1` |
| `PUT` | `/api/workspaces/{entryId}/manifest` | Apply reviewed typed Project patches; body `{revision, changes}` | `one-cli/workspace-manifest-apply/v1` |
| `GET/POST` | `/api/workspaces/{entryId}/secrets?env={environment}&project={name?}` | List direct key names / create one Infisical value | `one-cli/env-list/v1` / `one-cli/env-set/v1` |
| `GET/PUT/DELETE` | `/api/workspaces/{entryId}/secrets/{key}?env={environment}&project={name?}` | Reveal, update, or delete one Infisical value | `one-cli/env-get/v1`, `one-cli/env-set/v1`, or `one-cli/env-delete/v1` |
| `GET/PUT` | `/api/workspace/profile-bindings/env?env={environment}` | Launch-Workspace alias of the Workspace binding routes | Same as plural route |
| `PUT` | `/api/workspace/environment/backend?env={environment}` | Launch-Workspace alias of the Backend switch route | `one-cli/workspace-profile/v1` |
| `POST` | `/api/workspace/environment/backend/initialize?env={environment}&project={name?}` | Launch-Workspace alias of the binding repair route | `one-cli/workspace-profile/v1` |
| `GET` | `/api/workspace/projects/{name}?env={environment}` | Launch-Workspace Project projection alias | `one-cli/workspace-project/v1` |
| `PUT` | `/api/workspace/projects/{name}/profile-bindings/{domain}?env={environment}` | Launch-Workspace Project binding alias; body `{profile}` | `one-cli/workspace-project/v1` |

Plural Workspace routes accept only the opaque `entryId`. The server resolves its root from the registry and revalidates the Manifest before every read or mutation; a client-supplied `root` never selects a filesystem path. Manifest publication is a typed patch, not a replacement document. Secret folder paths are derived from the selected Workspace/Project; the browser cannot submit an arbitrary path. Sending an empty Profile string removes that direct binding and restores fallback resolution.

Former Project/Environment/Deploy/Container settings PUT paths under both `/api/workspace/...` and `/api/workspaces/{entryId}/...` remain registered for stale clients, but always return `409 SERVE_REPOSITORY_READ_ONLY`; repository writes use the revision-checked `/manifest` and `/environment/backend` routes. If copied Workspaces leave two live roots with one Manifest ID, both remain listed as conflicts: inspection is allowed and mutations return `409 Conflict` until the registry conflict is resolved.

Legal `(domain, backend)` values include `env/infisical`, `env/dotenv`, `deploy/aws-s3`, `deploy/aliyun-oss`, `deploy/tencent-cos`, `deploy/minio`, `deploy/rustfs`, `deploy/r2`, `deploy/kustomize`, `deploy/vercel`, `deploy/cloudflare`, `deploy/edgeone`, and `container/docker`. Other combinations return 404.

Probe example:

```bash
curl -s "http://127.0.0.1:<port>/api/configure" | jq '.config | keys'
```

## Common Errors

| Code | Recovery |
|---|---|
| `SERVE_PORT_BUSY` | Choose another port, or use `--port 0` |
| `SERVE_BIND_FORBIDDEN` | Bind only to loopback; use SSH tunneling for remote access |
| `SERVE_PAYLOAD_INVALID` | POST/PUT body is invalid JSON or missing a required field such as `name` or `profile` |
| `SERVE_MANIFEST_CONFLICT` | Reload the Workspace and review the current Manifest before recreating the draft |
| `SERVE_REPOSITORY_READ_ONLY` | Use the typed `/manifest` draft flow; the requested legacy route is not writable |
| `PROFILE_FILE_INVALID` | Repair the named local Profile file (`config.json`, `credentials.json`, or `profile-bindings.json`) |
| `PROFILE_IN_USE` | Choose **Automatic** for every Workspace/Project environment binding that references the Profile, then delete it |
| `PROFILE_BACKEND_INVALID` | URL `(domain, backend)` is not a legal pair |

Full table: [Error codes](/en/docs/error-codes/).
