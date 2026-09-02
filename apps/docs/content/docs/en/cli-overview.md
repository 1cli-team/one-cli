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
| `one ci` | Inspect or manage optional continuous integration | `one ci` |
| `one run` | Run a command with project `.env` injected | `one run -- npm test` |
| `one configure` | Configure machine-level endpoint profiles | `one configure` |
| `one serve` | Launch the local Workspace, Project, and Profile Dashboard | `one serve` |
| `one skills` | Install or refresh the bundled `one-cli` skill | `one skills install` |

## Create Workspaces

```bash
one create [dir] [--name <name>] [--env-provider dotenv|infisical] [--yes]
```

`[dir]` is the target directory. The workspace name defaults to `basename(dir)`. Create produces an empty workspace with local dotenv and `one dev`; it does not configure CI, ask for projects or deployment, or install Coding Agent Skills.

Read [Create](/en/docs/create/).

## Add Projects

```bash
one add # open the interactive picker
one templates # see available templates
one add <template-id> --name <project-name> [--yes] # add a specific stack
```

Bare `one add` asks which directory group to add to (application, service, or shared library), then the technology stack, then the project name. Documentation sites are applications. It does not configure CI or ask about deployment. Ordinary add leaves deployment unset until `one deploy <project>`; `--deploy-provider` remains an advanced automation option.

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

## Local Connections

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

`configure` manages local connections and preferences. With no connections, bare `one configure` opens the setup wizard; otherwise it shows a concise overview. `show`, `use`, and `remove` allow terminal selection. Scripts keep explicit service IDs and `--profile` names for compatibility. Credentials stay in local files, never the workspace or Git.

Supported `<pair>` values:

| Domain | Backends |
|---|---|
| `env` | `infisical` |
| `container` | `docker` |
| `container` | `dockerhub`, `ghcr`, `acr` |
| `deploy` | `aliyun-oss`, `tencent-cos`, `aws-s3`, `minio`, `rustfs`, `r2` |
| `deploy` | `kustomize`, `vercel`, `cloudflare`, `edgeone` |

Local `.env` files do not need a machine-level connection.
Local connections are stored in `~/.config/one/config.json` and `~/.config/one/credentials.json`. Environment-aware Workspace/Project selections contain names only and live in `~/.config/one/profile-bindings.json`. Sensitive fields are masked unless you explicitly run `show --reveal`; none of these files changes `one.manifest.json`.
When adding tokens, prefer `one configure open` so you do not hand tokens to an AI agent.

## Interactive Mode At A Glance

| Command | Interactive behavior |
|---|---|
| `one create` | Yes; no-arg mode asks for target directory and optional workspace name |
| `one add` | Yes; no-arg mode picks project kind, technology stack, and project name |
| `one configure` | Yes; bare `one configure` or `one configure add` opens the local-connection wizard |
| `one skills install` | Yes; no-arg mode multi-selects target agents |
| `one env set` | Yes; hidden value input, scope selection, and overwrite confirmation; scripts pass the value |
| `one container build` | Partial; TTY mode can choose a build version, CI uses `--build-version` |
| `one deploy` | First deployment asks for project, target category/service, and local connection; scripts pass `--provider` and `--profile` |
| `one dev` | Missing Node dependencies trigger an install confirmation; otherwise starts immediately |
| `one ci disable` | Asks before removing generated workflow files; refusal exits successfully |
| `one templates` / `one run` | No wizard; behavior is controlled by arguments |
| `one serve` | Not a terminal wizard; it opens a local Dashboard for Workspaces, Projects, and local connections |

## Local Web UI

```bash
one serve [--host 127.0.0.1] [--port 0] [--open=false]
```

Starts a loopback-only HTTP server for humans to edit `env / deploy / container` Profiles, select environment-aware local bindings, and review typed Workspace Backend or Project configuration drafts before publishing them with revision checks. Workspace source code and non-allowlisted Manifest fields remain read-only. This path handles API keys, kubeconfig paths, and registry tokens, so it is intentionally not an AI-agent credential-editing interface.

Read [Serve](/en/docs/serve/).

## Containers

```bash
one container info
one container build [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
one container push  [subproject] [-p <name|path>] [--build-version <version>] [--dry-run] [--profile <name>]
```

`one container` reads each project's Dockerfile and manifest container config. Bare `build` creates a local `<workload>:<version>` image. Passing `--profile`, or resolving a machine-local registry binding/default, produces a registry-qualified tag and performs login. `push` requires a registry Profile and can retag the local image before pushing.

## Local Development

```bash
one dev [project] [--dry-run]
```

Reads project dev commands and starts every developable project in parallel. The positional project starts only one; `--project` remains for old scripts. Missing Node dependencies can be installed after confirmation.

## Deployment

```bash
one deploy [project] [--provider <target>] [--profile <connection>] [--dry-run]
```

On first deployment, One CLI shows only compatible targets already implemented by this repository, then asks for a local connection. Choosing "configure later" exits successfully without changing the workspace. Later runs reuse the saved project deployment target.

`--env <name>` overrides the deploy target for this run. `--dry-run` prints the docker / kubectl / S3 / platform CLI plan without touching remote systems.

## Continuous Integration

```bash
one ci
one ci enable [project]
one ci sync [project]
one ci disable [project]
```

CI is optional and is never added by `one create` or `one add`. The current
build generates GitHub Actions workflows. Omit `[project]` to operate on all
projects (`sync` refreshes only projects where CI is already enabled).

Read [Continuous integration](/en/docs/ci/).

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
one help --all
one <command> --help
```

`one --help` shows the six everyday tasks. Use `one help --all` for the complete command catalogue and `one <command> --help` for exact flags.
