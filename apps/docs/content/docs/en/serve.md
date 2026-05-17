---
title: one serve
description: Local web UI for editing machine-level profiles, with manual credential entry and no AI exposure.
---

`one serve` starts a small HTTP service bound only to `127.0.0.1` and opens a browser so you can edit `~/.config/one/config.json` and `~/.config/one/credentials.json`. These are the same `(domain, backend)` profile sections managed by `one configure`: Infisical credentials, S3 AK/SK, kubeconfig context, registry token, and dotenv names.

Why not let AI edit the file directly: it contains API keys, kubeconfig paths, and registry tokens. The risk of leaking them is higher than the value of saving a few manual inputs. `one serve` physically keeps those fields out of command-line and agent context.

## Usage

```bash
one serve [options]
```

The process blocks in the foreground. Press Ctrl-C to stop. Every mutation writes directly to `~/.config/one/{config,credentials}.json` using `0600` files and atomic rename, sharing the same storage layer as `one configure <verb> <pair>`.

## Arguments

| Argument | Description |
|---|---|
| `--host <host>` | Bind host. Only loopback is accepted (`127.0.0.1`, `localhost`, `::1`). Non-loopback returns `SERVE_BIND_FORBIDDEN` |
| `--port <n>` | Listen port. Default `0` lets the kernel pick a free port |
| `--open` | Open browser after startup. Default `true`; pass `--open=false` for CI, headless, WSL, or remote SSH |
| `-o, --output <fmt>` | `json` / `yaml` / `text`; default is TTY-aware auto detection |

## Interactive Mode

`one serve` has no terminal wizard. It starts a local web UI and handles sensitive profile fields through browser forms.

For local human setup, run `one serve`. Scripts, CI, and agents usually use `--open=false` only to receive the URL; they should not bypass the browser UI to read cleartext credentials.

## Output

After binding, stdout emits one startup envelope and then blocks:

```json
{
  "schema": "one-cli/serve/v1",
  "status": "listening",
  "url": "http://127.0.0.1:54321/?token=8bRxr7N-GN1Q...",
  "host": "127.0.0.1",
  "port": 54321,
  "token": "8bRxr7N-GN1Q..."
}
```

The `?token=` in the URL is a one-time 32-byte session token generated for this process. The same token is written as an `HttpOnly; SameSite=Strict` cookie on the first `GET /`. When the process exits, the token is invalid; restart creates a new URL.

## Security Model

`one serve` owns profile files, and profile files own credentials. `/api/*` has three independent defenses:

| Layer | Threat blocked | Behavior |
|---|---|---|
| Host header check | DNS rebinding, where an attacker domain resolves to `127.0.0.1` | `Host` must match the bound `127.0.0.1:<port>` or `localhost:<port>`, otherwise `421 Misdirected Request` |
| Origin check for mutations | Cross-origin POST / script requests | POST/PUT/DELETE `Origin` must equal the service origin, otherwise `403 Forbidden` |
| Session token | Stale tab reuse and CSRF | `/api/*` must include the correct token through cookie or `?token=`, otherwise `401 Unauthorized` |

Credentials are **masked by default**. `GET /api/configure*` returns values such as `clientSecret: "********"`, `accessKeySecret: "********"`, and `password: "********"`. The UI's reveal button calls `?reveal=1` to fetch cleartext.

Out of scope:

- Multi-user access
- `0.0.0.0` / LAN exposure; `SERVE_BIND_FORBIDDEN` refuses it
- Live push when external processes edit profile files; refresh the browser after `one configure ... add`

## Examples

### Default: Random Port + Auto-open Browser

```bash
one serve
# profile UI started: http://127.0.0.1:54321/?token=...
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

The web UI uses these same routes. All routes require token plus Host match; mutating routes also require Origin match.

| Method | Path | Meaning | Response schema |
|---|---|---|---|
| `GET` | `/api/configure` | All profile sections | `one-cli/serve-configure-config/v1` |
| `GET` | `/api/configure/{domain}/{backend}` | One section; `?reveal=1` returns cleartext | `one-cli/serve-configure-section/v1` |
| `POST` | `/api/configure/{domain}/{backend}` | Upsert body `{name, profile, use?}` | `one-cli/serve-configure-upsert/v1` |
| `DELETE` | `/api/configure/{domain}/{backend}/{name}` | Remove profile | `one-cli/serve-configure-remove/v1` |
| `PUT` | `/api/configure/{domain}/{backend}/default` | Set default profile with body `{name}` | `one-cli/serve-configure-use/v1` |

Legal `(domain, backend)` values include `env/infisical`, `env/dotenv`, `deploy/aws-s3`, `deploy/aliyun-oss`, `deploy/tencent-cos`, `deploy/minio`, `deploy/rustfs`, `deploy/r2`, `deploy/kustomize`, `deploy/vercel`, `deploy/cloudflare`, `deploy/edgeone`, and `container/docker`. Other combinations return 404.

Probe example:

```bash
curl -s "http://127.0.0.1:<port>/api/configure?token=<token>" | jq '.config | keys'
```

## Common Errors

| Code | Recovery |
|---|---|
| `SERVE_PORT_BUSY` | Choose another port, or use `--port 0` |
| `SERVE_BIND_FORBIDDEN` | Bind only to loopback; use SSH tunneling for remote access |
| `SERVE_TOKEN_INVALID` | Restart `one serve` and use the new URL |
| `SERVE_PAYLOAD_INVALID` | POST/PUT body is invalid JSON or missing required fields such as `name` |
| `PROFILE_FILE_INVALID` | Repair or delete `~/.config/one/config.json` / `credentials.json` and recreate profiles |
| `PROFILE_BACKEND_INVALID` | URL `(domain, backend)` is not a legal pair |

Full table: [Error codes](/en/docs/error-codes/).
