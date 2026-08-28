---
title: one deploy
description: Dispatch projects to kustomize, S3-compatible storage, Vercel, Cloudflare, or EdgeOne deploy backends.
---

`one deploy` is the per-project deploy entry point. The first deployment chooses a compatible target and local connection; later runs reuse that project setup.

## Usage

```bash
one deploy [project] [--provider <target>] [--profile <connection>] [--env <env>] [--dry-run]
```

## Options

| option | purpose |
|---|---|
| positional `project` | deploy one project by manifest `name` or `relativeDir` |
| `-p`, `--project <name|path>` | legacy selector for scripts and CI |
| `--provider <target>` | explicit first-deploy target for automation |
| `--profile <name>` | one-shot local-connection override |
| `--env <env>` | override both deploy target environment and env-var environment |
| `--env-provider dotenv|infisical` | override the workspace env provider |
| `--build-version <version>` | CI/non-interactive image version, mainly for kustomize auto-build |
| `--dry-run` | print Docker, kubectl, object-storage, or platform CLI plans without remote side effects |

## Interactive Mode

For an unconfigured project, TTY mode asks in this order:

1. Project (when omitted)
2. Deployment-target category
3. A compatible service already implemented by this build
4. Whether to configure a missing local connection now or later

"Configure later" exits 0, does not modify the workspace, and prints the exact recovery command. Scripts use `one deploy <project> --provider <target> --profile <connection>`.

## Backends

| backend | project type | behavior |
|---|---|---|
| `kustomize` | APIs, SSR apps, container workloads | auto-builds and pushes the image, syncs the overlay, then runs `kubectl apply -k` |
| `aws-s3` / `aliyun-oss` / `tencent-cos` / `minio` / `rustfs` / `r2` | static sites | builds output, ensures the bucket, uploads through S3-compatible APIs |
| `vercel` | hosted frontend | deploys through Vercel |
| `cloudflare` | Cloudflare Workers | runs `wrangler deploy` |
| `edgeone` | EdgeOne Pages | runs `edgeone pages deploy` |

## Environment mapping

| backend | `prod` or empty | other env names |
|---|---|---|
| `kustomize` | `kustomize/overlays/prod` | `kustomize/overlays/<env>` |
| `vercel` | production deploy | preview deploy |
| `cloudflare` | `wrangler deploy` | `wrangler deploy --env=<env>` |
| `edgeone` | production deploy | preview deploy |
| S3-compatible | deploy target unchanged | deploy target unchanged; only build-time env changes |

`--env` must exist in `one.manifest.json#environments.names`.

## Profile resolution

1. `--profile <name>`
2. `~/.config/one/config.json#workspaces[workspaceId].projects[project].profiles[deploy/backend]`
3. `~/.config/one/config.json#workspaces[workspaceId].profiles[deploy/backend]`
4. `~/.config/one/config.json#deploy/<backend>.default`

Manifest files never store local profile names. Bind one locally with `one configure use <pair> --profile <name> --workspace`, or add `--project <name|path>` for a single project.

## Examples

```bash
one deploy --dry-run
one deploy web --env staging --dry-run
one deploy api --provider kustomize --profile prod-k8s --build-version v0.1.0
```

## Output schemas

| backend | schema |
|---|---|
| `kustomize` | `one-cli/deploy-apply/v1` |
| S3-compatible | `one-cli/deploy-apply/v1` |
| `vercel` | `one-cli/deploy-apply-vercel/v1` |
| `cloudflare` | `one-cli/deploy-apply-cloudflare/v1` |
| `edgeone` | `one-cli/deploy-apply-edgeone/v1` |

## Common errors

| code | fix |
|---|---|
| `BACKEND_NOT_ENABLED` | select a project and explicit target in non-interactive calls |
| `PROFILE_NOT_FOUND` | run `one configure list deploy/<backend>` |
| `PROFILE_NONE_CONFIGURED` | run `one configure add deploy/<backend> <name> --use` |
| `ENV_UNKNOWN_ENVIRONMENT` | add the env to `manifest.environments.names` or use an existing name |
| `REGISTRY_CREDENTIAL_MISSING` | configure `container/docker` before kustomize auto-build |

## Next

- [First deploy](/en/tutorials/deploy/)
- [Multi-backend deploy](/en/tutorials/deploy-multi-backend/)
- [one configure](/en/docs/configure/)
- [one container](/en/docs/container/)
