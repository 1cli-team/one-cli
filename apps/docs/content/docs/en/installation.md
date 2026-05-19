---
title: Installation
description: Install the one binary on your machine, including macOS / Linux one-line install, Windows manual download, upgrades, downgrades, and uninstall.
---

Install the `one` binary onto your `PATH`. It should take only a few seconds.

**For**: first-time install, upgrade / downgrade, custom install location, uninstall.

**You will finish with**: `one --version` working in your shell, plus a clear understanding of upgrade semantics and installer environment variables.

## macOS / Linux One-line Install

```bash
curl -fsSL https://1cli.dev/install.sh | bash
```

The script:

1. Detects `$os/$arch` (`darwin` / `linux`, `amd64` / `arm64`)
2. Resolves the latest version from the GitHub Releases latest redirect
3. Downloads the matching tarball from release assets and verifies SHA256
4. Extracts `one` into `~/.local/bin/one`
5. Tells you if `PATH` needs an update

**Audit the script**: open `https://1cli.dev/install.sh` in a browser. It is plain text.

Verify after installation:

```bash
one --version
# 0.1.0 (or later)
```

If `PATH` is missing, the script will tell you what to add:

```bash
# zsh:
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc

# bash:
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
```

Open a new shell.

## Windows / Manual Download

Download the matching archive from [GitHub Releases](https://github.com/1cli-team/one-cli/releases/latest) (`darwin/linux/windows x amd64/arm64`), unzip it, and put `one` on `PATH`.

Example for Linux amd64:

```bash
curl -L -o one.tar.gz \
  https://github.com/1cli-team/one-cli/releases/latest/download/one-cli_linux_amd64.tar.gz
tar -xzf one.tar.gz
mv one ~/.local/bin/
one --version
```

On Windows, download `one-cli_windows_amd64.zip`, unzip it, and put `one.exe` in a directory on `PATH`.

## Upgrade And Downgrade

`install.sh` checks the installed `one --version` before deciding what to do:

| Current state | Behavior |
|---|---|
| Not installed | Install |
| Target is newer | Upgrade automatically |
| Target is the same | **Skip**; set `ONE_FORCE=1` to reinstall a damaged binary |
| Target is older | **Refuse** downgrade; set `ONE_FORCE=1` if you intentionally want to downgrade |

For normal upgrades, rerun the install command. Use `ONE_FORCE` only for downgrade or repair.

## After Install: Run `one skills install`

`install.sh` only puts the `one` binary on `PATH`. To let Claude Code / Cursor / Codex and similar agents discover One CLI skills, run this once:

```bash
one skills install
```

It detects supported agents on your machine and lets you choose where to install the skills. Only Claude Code is pre-selected by default; use Up/Down to move, Space to check or uncheck, and Enter to start installing.

Non-interactive usage:

```bash
one skills install --yes              # install to every detected agent
one skills install --agent claude-code # install to a single target
```

After upgrading the binary, rerun `one skills install` to refresh the skill content. It is idempotent. Current skills include `one-cli` for create/add/dependencies/reference workflows and `one-migrate` for migrating existing projects.

Provider credentials are configured once with `one configure add <domain>/<backend> --profile <name>` and can be reused across workspaces. Current configurable pairs are:

| pair | use when |
|---|---|
| `env/infisical` | Infisical machine identity |
| `deploy/aliyun-oss` | Aliyun OSS, S3 protocol object storage |
| `deploy/tencent-cos` | Tencent COS, S3 protocol object storage |
| `deploy/aws-s3` | AWS S3 |
| `deploy/minio` | self-hosted MinIO |
| `deploy/rustfs` | self-hosted RustFS |
| `deploy/r2` | Cloudflare R2 |
| `deploy/kustomize` | Kubernetes kubeconfig + context |
| `deploy/vercel` | Vercel API token |
| `deploy/cloudflare` | Cloudflare API token |
| `deploy/edgeone` | Tencent EdgeOne Pages API token |
| `container/docker` | Generic Docker registry login |
| `container/dockerhub` | Docker Hub login |
| `container/ghcr` | GitHub Container Registry login |
| `container/acr` | Aliyun ACR login |

`env/dotenv` does not need remote credentials; it reads and writes local project `.env` files. The S3-compatible deploy backends share the same profile shape, but their backend IDs stay explicit (`deploy/aws-s3`, `deploy/aliyun-oss`, `deploy/r2`, etc.).

Common examples:

```bash
one configure add env/infisical --profile work         # Infisical credentials
one configure add deploy/aws-s3 --profile web-prod     # AWS S3 endpoint + AK/SK
one configure add deploy/kustomize --profile prod-k8s  # kubeconfig context
one configure add container/ghcr --profile ghcr        # GHCR username + PAT
```

See [Install skill to agent](/en/tutorials/skills-install/).

## Environment Variables

`install.sh` accepts:

| Variable | Default | Meaning |
|---|---|---|
| `ONE_VERSION` | resolved from the latest GitHub release | Lock the version, for example `v0.1.0` |
| `ONE_INSTALL_DIR` | `$HOME/.local/bin` | Install directory |
| `ONE_FORCE` | `0` | Set to `1` to allow downgrade, same-version reinstall, or overwrite a binary whose version cannot be read |
| `ONE_REPO_URL` | `https://github.com/1cli-team/one-cli` | GitHub repo URL override for debugging |
| `ONE_RELEASE_BASE_URL` | `$ONE_REPO_URL/releases/download` | Release asset download base override |
| `ONE_LATEST_URL` | `$ONE_REPO_URL/releases/latest` | Latest release resolver override |
| `ONE_SKIP_VERIFY` | `0` | Set to `1` to skip SHA256 verification; debugging only |

Install a specific older version into a custom directory:

```bash
curl -fsSL https://1cli.dev/install.sh | ONE_VERSION=v0.1.0 ONE_INSTALL_DIR=/opt/bin bash
```

## Uninstall

```bash
rm ~/.local/bin/one
```

If you also want to remove skills installed by `one skills install`, delete the corresponding `~/.<agent>/skills/one-cli` / `~/.<agent>/skills/one-migrate` symlinks, or delete the whole `~/.one/skills-store/` directory. To remove local profile credentials and cache, delete `~/.config/one`.

## Local Repo Build For Contributors

If you are changing One CLI itself, read [CONTRIBUTING.md](https://github.com/1cli-team/one-cli/blob/master/CONTRIBUTING.md). Short version:

```bash
git clone https://github.com/1cli-team/one-cli
cd one-cli
brew install go go-task     # macOS; adapt for Linux
task install-local           # build current branch and symlink to ~/.local/bin/one
hash -r
which one
one --version
```

For the full contributor flow, see [CONTRIBUTING.md](https://github.com/1cli-team/one-cli/blob/master/CONTRIBUTING.md). For command-surface reference, see [Command overview](/en/docs/cli-overview/).

## Installed?

Go to [Quick start](/en/docs/quick-start/) and create your first workspace.
