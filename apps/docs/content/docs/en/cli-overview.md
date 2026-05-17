---
title: CLI overview
description: One CLI top-level commands, common subcommands, output modes, and automation contracts.
---

One CLI is a single binary. It creates workspaces, adds projects, manages environment variables and endpoint profiles, runs local dev / container / deployment workflows, and exposes stable JSON output for agents and CI.

**Who this page is for**: people who just installed One CLI and want to know which commands exist; people who cannot remember a flag.

**After reading**: you will know each public command's one-line purpose, minimal example, common subcommands, and where to jump next for details.

## Top-level commands

| Command | Purpose | Minimal example |
|---|---|---|
| `one create` | Scaffold a new workspace | `one create my-app` |
| `one add` | Add a project interactively or from templates | `one add` |
| `one templates` | List available templates | `one templates` |
| `one env` | Manage dotenv / Infisical environment variables | `one env list` |
| `one container` | Inspect, build, and push Dockerfile-driven images | `one container info` |
| `one dev` | Start every project's local dev process in parallel | `one dev` |
| `one deploy` | Dispatch per-project deploys to kustomize / S3-compatible / Vercel / Cloudflare / EdgeOne | `one deploy --dry-run` |
| `one run` | Run a command with project `.env` injected | `one run -- npm test` |
| `one configure` | Configure machine-level endpoint profiles | `one configure` |
| `one serve` | Launch the local web UI for human profile editing | `one serve` |
| `one skills` | Install or refresh the bundled `one-cli` skill | `one skills install` |

## Create Workspaces

```bash
one create [dir] [--name <name>] [--env-provider dotenv|infisical] [--yes]
```

`[dir]` is the target directory, not the project name. The default name is `basename(dir)`. Bare `one create` asks for the target directory and optional project name. `dotenv` is the default env provider; pass `--env-provider infisical` explicitly when needed.

Read [Create](/en/docs/create/).

## Add Projects

```bash
one add # open the interactive picker
one templates # see available templates
one add <template-id> --name <project-name> [--deploy-provider <id>] [--yes] # add a specific template and choose a deploy mode
```

For a first run, use bare `one add` and follow the interactive picker for category, template, and project name. For explicit commands, run `one templates` first and put the listed template ID in the `one add <template-id>` position.

API / SSR templates usually enable `container/docker + deploy/kustomize`;
static web templates usually enable S3-compatible deploy across multiple S3 platforms;
mobile, library, and Electron templates do not enable deploy / container by default.

Read [Add](/en/docs/add/).

## Templates

```bash
one templates
one templates -o json
```

`one templates` lists bundled templates. Agents and CI should use `-o json` to read template IDs, categories, toolchains, and compatible backends.

Read [Templates](/en/docs/templates-cmd/).

## Environment Variables

```bash
one env get <KEY> [--env <env>] [-p <name|path>]
one env set <KEY[=VALUE]> [VALUE] [--env <env>] [-p <name|path>]
one env list [--env <env>] [-p <name|path>]
one env pull [--env <env>] [-p <name|path>] [--force] [--dry-run]
```

`one env` dispatches to the workspace's selected env backend. `dotenv` reads and writes local `.env` overlays; `infisical` supports remote get / set / list / pull. `--env` selects an environment such as dev, staging, or prod. `-p / --project` selects a project by manifest name or workspace-relative path.

Read [Secrets](/en/docs/env-vars/).

## Machine Profiles

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

`configure` manages machine-level endpoint profiles. Bare `one configure` and `one configure add` open the interactive wizard. Non-interactive scripts should pass both `<pair>` and `--profile`. Configure a profile once on your own machine and reuse it later.

Supported `<pair>` values:

| Domain | Backends |
|---|---|
| `env` | `infisical` |
| `container` | `docker` |
| `container` | `dockerhub`, `ghcr`, `acr` |
| `deploy` | `aliyun-oss`, `tencent-cos`, `aws-s3`, `minio`, `rustfs`, `r2` |
| `deploy` | `kustomize`, `vercel`, `cloudflare`, `edgeone` |

`env/dotenv` is the workspace-local `.env` backend and does not need a machine-level profile.
Profiles are stored in `~/.config/one/config.json` and `~/.config/one/credentials.json`. Sensitive fields are masked unless you explicitly run `show --reveal`.
When adding tokens, prefer using `one serve` to configure them so you do not hand tokens to an AI agent.

## Interactive Mode At A Glance

| Command | Interactive behavior |
|---|---|
| `one create` | Yes; no-arg mode asks for target directory and optional project name |
| `one add` | Yes; no-arg mode picks category, template, project name, and sometimes deploy backend |
| `one configure` | Yes; bare `one configure` or `one configure add` opens the profile wizard |
| `one skills install` | Yes; no-arg mode multi-selects target agents |
| `one env set` | Partial; confirms unknown environments or overwrites, scripts use `--yes` |
| `one container build` | Partial; TTY mode can choose a build version, CI uses `--build-version` |
| `one deploy` | Partial; kustomize build version or missing Cloudflare profile can ask questions, CI should pass explicit flags |
| `one templates` / `one dev` / `one run` | No wizard; behavior is controlled by arguments |
| `one serve` | Not a terminal wizard; it opens a local web UI for manual profile editing |

## Local Web UI

```bash
one serve [--host 127.0.0.1] [--port 0] [--open=false]
```

Starts a loopback-only HTTP server for humans to edit `env / deploy / container` profiles in a browser. This path handles API keys, kubeconfig paths, and registry tokens, so it is intentionally not an AI-agent credential-editing interface.

Read [Serve](/en/docs/serve/).

## Containers

```bash
one container info
one container build [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
one container push  [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
```

`one container` reads each project's Dockerfile and manifest container config. Bare `build` creates a local `<workload>:<version>` image. Passing `--profile`, or pinning a registry profile in the manifest, produces a registry-qualified tag and performs login. `push` requires a registry profile and can retag the local image before pushing.

## Local Development

```bash
one dev [-p <name|path>] [--dry-run]
```

Reads `projects[].domains.dev.command` from `one.manifest.json` and starts every project in parallel through the built-in supervisor (no third-party runner needed). `-p / --project` starts a single project. `--dry-run` prints the resolved commands without launching them.

## Deployment

```bash
one deploy [-p <name|path>] [--profile <name>] [--env <env>] [--env-provider dotenv|infisical] [--build-version <version>] [--dry-run]
```

`deploy` dispatches each project to the backend declared in the manifest. Backends / SSR projects usually use `kustomize`; static frontends can use S3-compatible backends; hosted frontends can use Vercel, Cloudflare, or EdgeOne.

`--env <name>` overrides the deploy target for this run. `--dry-run` prints the docker / kubectl / S3 / platform CLI plan without touching remote systems.

## Run With Env

```bash
one run [-p <name|path>] [--env-provider dotenv|infisical] [--env <env>] -- <command> [args...]
```

Runs the child process in the resolved project directory after injecting secrets. By default it uses the workspace manifest's env provider; pass `--env-provider` to force dotenv or Infisical.

## Agent Skills

```bash
one skills install # choose target AI agents interactively
one skills install --yes
one skills install --agent claude-code # install skills for a specific AI agent
```

Installs or refreshes the bundled `one-cli` skill into detected coding agents.

| Skill | Purpose |
|---|---|
| `one-cli` | Create workspaces, add template projects, install missing dependencies, and look up commands / JSON / error codes |

Read [Install skill to agent](/en/tutorials/skills-install/).

## Output Modes

Every command supports the same output flags:

| Trigger | Mode |
|---|---|
| `-o json` or `--output json` | Force pretty-printed JSON |
| `-o yaml` or `--output yaml` | Force YAML with the same schema as JSON |
| `-o text` or `--output text` | Force human output |
| Default + pipe / non-TTY | JSON |
| Default + TTY | Colored human output |

Running `one templates` directly shows terminal-friendly output.
Agents and CI get JSON by default when reading through a pipe.
Scripts should still pass `-o json` explicitly so parsing does not depend on the execution environment.

## Meta Commands

```bash
one --version
one --help
one <command> --help
```

`one --help` shows only top-level commands. Use `one <command> --help` for exact flags, and read [Error codes](/en/docs/error-codes/) for structured failures.
