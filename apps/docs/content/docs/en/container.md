---
title: one container
description: Inspect, build, and push Dockerfile images for workspace projects.
---

`one container` operates on projects that declare `projects[].domains.container` in `one.manifest.json`. The current backend is Dockerfile-driven: templates provide Dockerfiles, and One CLI resolves projects, image names, registry tags, and Docker invocations.

## Usage

```bash
one container info
one container build [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
one container push  [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
```

`[subproject]` and `-p / --project` select one project by manifest `name` or `relativeDir`. Omit them to target every container-enabled project.

## Interactive Mode

`one container info` and `one container push` do not open a wizard. In TTY mode, `one container build` can ask you to choose or type an image version when `--build-version` is omitted and One CLI cannot infer a stable version from the manifest, Git, or project metadata.

Scripts, CI, and agents should pass `--build-version` and `--profile` explicitly, or use `--dry-run` first to inspect the Docker command.

## info

Read-only inspection:

```bash
one container info -o json
```

Schema: `one-cli/container-info/v2`.

## build

```bash
one container build api
one container build -p services/api --build-version v0.1.0
one container build --dry-run
```

By default, build creates a local tag: `<workload>:<version>`. When `--profile` is passed, or the manifest pins a container profile, the tag becomes `<registry>/[namespace/]<workload>:<version>` and Docker login runs when credentials are present.

Schema: `one-cli/container-build/v2`.

## push

```bash
one container push api --profile ghcr
one container push -p apps/web --build-version v0.1.0 --dry-run
```

`push` must resolve a container profile for the project's kind, such as `container/ghcr`, `container/dockerhub`, `container/acr`, or generic `container/docker`. If the registry-qualified tag is missing locally but the matching bare tag exists, CLI retags first and then pushes.

Schema: `one-cli/container-push/v1`.

## Profile resolution

1. `--profile <name>`
2. `~/.config/one/config.json#workspaces[workspaceId].projects[project].profiles[container/kind]`
3. `~/.config/one/config.json#workspaces[workspaceId].profiles[container/kind]`
4. `~/.config/one/config.json#container/<kind>.default`

Configure once:

```bash
one configure add container/ghcr --profile ghcr \
  --namespace "$GITHUB_USER" \
  --username "$GITHUB_USER" \
  --password "$GHCR_PAT" \
  --use
```

Supported kinds are `container/docker` for a generic registry, plus `container/dockerhub`, `container/ghcr`, and `container/acr`.

## Manifest conditions

`nestjs-api`, `go-api`, and `nextjs-app` enable `container/docker` by default. Libraries, mobile, and Electron templates do not.

## Common errors

| code | fix |
|---|---|
| `BACKEND_NOT_ENABLED` | choose a template with container support or add container config to the manifest |
| `REGISTRY_CREDENTIAL_MISSING` | run `one configure add container/<kind> --profile <name> --use` |
| `IMAGE_TAG_NOT_FOUND` | build first, or pass the same `--build-version` |
| `CONTAINER_BUILD_FAILED` | run the printed Docker command inside the project for full logs |

## Next

- [Build & push images](/en/tutorials/container-build-push/)
- [one configure](/en/docs/configure/)
- [one deploy](/en/docs/deploy/)
