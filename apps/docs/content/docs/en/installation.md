---
title: Installation
description: Install the one binary on macOS, Linux, or Windows, including one-line installers, upgrades, downgrades, and uninstall.
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
# 0.1.1 (or later)
```

If `PATH` is missing, the script will tell you what to add:

```bash
# zsh:
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc

# bash:
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
```

Open a new shell.

## Windows PowerShell Install

Windows 10/11 x64 users can install from PowerShell:

```powershell
irm https://1cli.dev/install.ps1 | iex
```

The script downloads `one-cli_windows_amd64.zip`, verifies it against `checksums.txt`, installs `one.exe` under `%LOCALAPPDATA%\Programs\one\bin`, and adds that directory to your user PATH. Open a new terminal after the first install.

**Audit the script**: open `https://1cli.dev/install.ps1` in a browser. It is plain text.

## Manual Download

Download the matching archive from [GitHub Releases](https://github.com/1cli-team/one-cli/releases/latest), unpack it, and put `one` (or `one.exe`) on PATH. Windows currently publishes an x64 archive; macOS and Linux publish x64 and arm64 archives.

Example for Linux amd64:

```bash
curl -L -o one.tar.gz \
  https://github.com/1cli-team/one-cli/releases/latest/download/one-cli_linux_amd64.tar.gz
tar -xzf one.tar.gz
mv one ~/.local/bin/
one --version
```

On Windows, the archive is `one-cli_windows_amd64.zip`.

## Upgrade And Downgrade

`install.sh` and `install.ps1` check the installed `one --version` before deciding what to do:

| Current state | Behavior |
|---|---|
| Not installed | Install |
| Target is newer | Upgrade automatically |
| Target is the same | **Skip**; set `ONE_FORCE=1` to reinstall a damaged binary |
| Target is older | **Refuse** downgrade; set `ONE_FORCE=1` if you intentionally want to downgrade |

For normal upgrades, rerun the install command. Use `ONE_FORCE` only for downgrade or repair.

## mise Runtime

One does not bundle mise. When a command needs it, One uses an explicit `ONE_MISE_BINARY` first, then a compatible mise on PATH (minimum 2026.9.7), then its own verified installation. If none is available, it downloads the pinned official mise 2026.9.7 release, verifies both archive and executable SHA256, and installs it atomically. Supported targets are macOS/Linux x64 and arm64, and Windows x64; Linux uses musl releases. [Official distribution](https://mise.jdx.dev/installing-mise.html).

Selection happens again on each invocation. Removing system mise switches to the managed runtime; a missing or damaged managed executable is repaired automatically. A compatible system installation takes priority again when restored. Invalid explicit overrides report an error instead of falling back. No shell activation is required.

| Managed content | Default location | Root override |
|---|---|---|
| mise executable | `~/.local/share/one/runtimes/mise/<version>/<platform>/` | `XDG_DATA_HOME` |
| Tools and plugins | `~/.local/share/one/mise/` | `XDG_DATA_HOME` |
| Global configuration | `~/.config/one/mise/` | `XDG_CONFIG_HOME` |
| State and trust records | `~/.local/state/one/mise/` | `XDG_STATE_HOME` |
| Disposable cache | `~/.cache/one/mise/` | `XDG_CACHE_HOME` |

The same effective Home convention applies on Windows. XDG roots must be absolute. For managed mise, One sets `MISE_DATA_DIR`, `MISE_CONFIG_DIR`, `MISE_STATE_DIR`, and `MISE_CACHE_DIR` in the child environment, replacing inherited values. External mise retains its existing directory settings. Project configuration stays in the project. Switching to managed mise can require reinstalling tools and granting trust again; One does not copy external configuration or trust records.

Managed mise updates with One; automatic self-updates are disabled for that child process. Manually replacing it with `mise self-update` causes the next invocation to restore the pinned executable. A verified executable from an older One cache can be migrated without downloading, preserving the old file. Cache cleanup does not remove tools or trust records.

**Offline use:** a valid external, managed, or migratable legacy executable can be reused offline. A fresh environment without any of them needs network access. Prepare mise and the required tools/dependencies beforehand, or point `ONE_MISE_BINARY` to a compatible external executable. `ONE_RUNTIME=builtin` is a temporary diagnostic escape hatch using existing tools.

Download, migration, or verification failures return `MISE_INSTALL_FAILED`. Check network/proxy access to GitHub Releases and permissions on One's runtime directory, then retry the same command. Help, dry-run, and static project/configuration generation do not prepare mise.

Use `one mise --version`, `one mise doctor`, or `one mise trust <config-path>` to work with the same selected runtime. Review configuration before trusting it; `MISE_PARANOID=1` requires explicit trust. Arguments, IO, and exit codes are forwarded, without One project secrets; use `one run` when those secrets are needed.

## Infisical login

Run `one login` to sign in with a browser. The session is saved in your system keyring. See [login and shared credentials](/en/docs/login/).

## Environment Variables

Both installers accept the variables below. PowerShell reads them from `$env:NAME`; its default install directory is `%LOCALAPPDATA%\Programs\one\bin`.

| Variable | Default | Meaning |
|---|---|---|
| `ONE_VERSION` | resolved from the latest GitHub release | Lock the version, for example `v0.1.1` |
| `ONE_INSTALL_DIR` | `$HOME/.local/bin`; Windows: `%LOCALAPPDATA%\Programs\one\bin` | Install directory |
| `ONE_FORCE` | `0` | Set to `1` to allow downgrade, same-version reinstall, or overwrite a binary whose version cannot be read |
| `ONE_REPO_URL` | `https://github.com/1cli-team/one-cli` | GitHub repo URL override for debugging |
| `ONE_RELEASE_BASE_URL` | `$ONE_REPO_URL/releases/download` | Release asset download base override |
| `ONE_LATEST_URL` | `$ONE_REPO_URL/releases/latest` | Latest release resolver override |
| `ONE_SKIP_VERIFY` | `0` | Set to `1` to skip SHA256 verification; debugging only |
| `ONE_NO_PATH_UPDATE` | `0` | Set to `1` to install without changing the user PATH |

Install a specific older version into a custom directory:

```bash
curl -fsSL https://1cli.dev/install.sh | ONE_VERSION=v0.1.0 ONE_INSTALL_DIR=/opt/bin bash
```

## Uninstall

PowerShell:

```powershell
Remove-Item "$env:LOCALAPPDATA\Programs\one\bin\one.exe"
```

macOS / Linux:

```bash
rm ~/.local/bin/one
```

Run `one logout` to remove the active session from the system keyring.

## Local Repo Build For Contributors

If you are changing One CLI itself, read [CONTRIBUTING.md](https://github.com/1cli-team/one-cli/blob/master/CONTRIBUTING.md). Short version:

```bash
git clone https://github.com/1cli-team/one-cli
cd one-cli
brew install go go-task     # macOS; adapt for Linux
task install                 # package Dashboard + CLI, then create a native launcher
hash -r
which one
one --version
```

Windows creates `~/.local/bin/one.exe`; when file symlinks are unavailable,
it falls back to a `one.cmd` forwarding shim. The extensionless `one` symlink
created by older versions is migrated safely.

For the full contributor flow, see [CONTRIBUTING.md](https://github.com/1cli-team/one-cli/blob/master/CONTRIBUTING.md). For command-surface reference, see [Command overview](/en/docs/cli-overview/).

## Installed?

Go to [Quick start](/en/docs/quick-start/) and create your first workspace.

## Agent skill

Run `one skills install` to install the bundled `one-cli` skill into your coding agent's user skills directory. Use `one skills install --help` for target selection and supported agents. See [Skills](./skills).
