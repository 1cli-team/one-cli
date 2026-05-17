---
title: one configure
description: Manage machine-level endpoint profiles for Infisical, object storage, Kubernetes, Vercel, Cloudflare, EdgeOne, and Docker registries.
---

`one configure` manages **machine-level profiles**, not application code. Profiles hold endpoints and credentials used later by `one env`, `one container`, `one deploy`, and `one run`.

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
```

Bare `one configure` and `one configure add` open the interactive wizard. Scripts, CI, and agents should pass `<pair>`, the profile name, and backend flags explicitly.

## Interactive Mode

For local human setup, use the wizard:

```bash
one configure
one configure add
```

The wizard first asks which `(domain, backend)` to configure, such as `env/infisical`, `deploy/aws-s3`, or `container/docker`. Then it asks for profile name, endpoint, token, access keys, kubeconfig, or registry fields as needed. Secret fields use password-style input.

Scripts, CI, and agents should not wait for the wizard; pass the pair, profile name, and backend flags explicitly.

## Supported pairs

| pair | purpose |
|---|---|
| `env/infisical` | Infisical site URL + Universal Auth client id / secret |
| `deploy/aliyun-oss` | Aliyun OSS object storage |
| `deploy/tencent-cos` | Tencent COS object storage |
| `deploy/aws-s3` | AWS S3 |
| `deploy/minio` | self-hosted MinIO |
| `deploy/rustfs` | self-hosted RustFS |
| `deploy/r2` | Cloudflare R2 |
| `deploy/kustomize` | Kubernetes kubeconfig + context |
| `deploy/vercel` | Vercel API token |
| `deploy/cloudflare` | Cloudflare API token |
| `deploy/edgeone` | Tencent EdgeOne Pages API token |
| `container/docker` | Generic Docker registry host, namespace, username, password |
| `container/dockerhub` | Docker Hub username, password/token, namespace |
| `container/ghcr` | GitHub Container Registry username, PAT, namespace |
| `container/acr` | Aliyun ACR region, username, password/token, namespace |

`env/dotenv` does not need a profile; it is for local `.env` workflows. The S3-compatible deploy backends share one profile shape, but each provider has its own backend ID.

## Examples

```bash
one configure add env/infisical --profile work \
  --client-id "$INFISICAL_CLIENT_ID" \
  --client-secret "$INFISICAL_CLIENT_SECRET" \
  --use

one configure add deploy/aws-s3 --profile web-prod \
  --region us-east-1 \
  --access-key-id "$AWS_ACCESS_KEY_ID" \
  --access-key-secret "$AWS_SECRET_ACCESS_KEY" \
  --use

one configure add deploy/kustomize --profile prod-k8s \
  --kubeconfig ~/.kube/config \
  --kubeconfig-context prod \
  --use

one configure add container/ghcr --profile ghcr \
  --namespace "$GITHUB_USER" \
  --username "$GITHUB_USER" \
  --password "$GHCR_PAT" \
  --use
```

## Resolution order

When a command needs a profile, it resolves in this order:

1. `--profile <name>`
2. project / workspace profile pins in `one.manifest.json`
3. `~/.config/one/config.json#domain/backend.default`

The same profile name can exist under different backends, for example `prod` under both `deploy/aws-s3` and `deploy/kustomize`.

## Storage

```text
~/.config/one/
├── config.json         # non-secret fields: endpoint, region, default pointer
├── credentials.json    # secrets: clientSecret, accessKeySecret, password
└── cache/              # short-lived token cache
```

Both JSON files are written as `0600`. `show` masks secrets by default; only `show --reveal` prints cleartext.

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
| `PROFILE_FILE_INVALID` | repair or delete `~/.config/one/config.json` / `credentials.json`, then recreate profiles |
| `PROFILE_VERSION_UNSUPPORTED` | recreate old configs under the current `(domain, backend)` layout |

## Next

- [one serve](/en/docs/serve/) — edit the same profiles in a local web UI
- [one env](/en/docs/env-vars/) — use `env/infisical`
- [one deploy](/en/docs/deploy/) — use deploy profiles
- [one container](/en/docs/container/) — use container profiles
