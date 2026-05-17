#!/usr/bin/env bash
# install.sh — One CLI installer for macOS / Linux.
#
#   curl -fsSL https://1cli.dev/install.sh | bash
#
# Default behavior:
#   - First install (no existing binary): install.
#   - Target newer than current: silent upgrade.
#   - Target same as current: skip (info message).
#   - Target older than current: refuse (downgrade requires ONE_FORCE=1).
#
# Configurable env vars:
#   ONE_VERSION       Pin a version (vX.Y.Z). Default: follow $ONE_LATEST_URL.
#   ONE_INSTALL_DIR   Install dir. Default: $HOME/.local/bin (matches `task install-local`).
#   ONE_FORCE         Set to 1 to allow downgrade, force-reinstall the same version,
#                     or overwrite an existing binary whose version can't be read.
#   ONE_REPO_URL      Override the GitHub repo URL. Default: https://github.com/1cli-team/one-cli.
#   ONE_RELEASE_BASE_URL
#                     Override release asset base. Default: $ONE_REPO_URL/releases/download.
#   ONE_LATEST_URL    Override latest release URL. Default: $ONE_REPO_URL/releases/latest.
#   ONE_SKIP_VERIFY   Set to 1 to skip SHA256 verification (debugging only).

set -euo pipefail

# Wrap the whole script in main() so a truncated curl download cannot
# leave a partial install behind: main is only called after the script
# is fully sourced.
main() {
    : "${ONE_REPO_URL:=https://github.com/1cli-team/one-cli}"
    : "${ONE_RELEASE_BASE_URL:=${ONE_REPO_URL%/}/releases/download}"
    : "${ONE_LATEST_URL:=${ONE_REPO_URL%/}/releases/latest}"
    : "${ONE_INSTALL_DIR:=$HOME/.local/bin}"
    : "${ONE_FORCE:=0}"
    : "${ONE_SKIP_VERIFY:=0}"

    detect_platform
    resolve_version
    preflight
    download_and_verify
    install_binary
    check_path
    "$ONE_INSTALL_DIR/one" --version
}

# ----- helpers --------------------------------------------------------------

red()    { printf '\033[31m%s\033[0m\n' "$*"; }
green()  { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }
info()   { printf 'one-cli: %s\n' "$*"; }
warn()   { yellow "one-cli: $*" >&2; }
err()    { red "one-cli: $*" >&2; }

# ----- platform -------------------------------------------------------------

detect_platform() {
    case "$(uname -s)" in
        Darwin) OS="darwin" ;;
        Linux)  OS="linux"  ;;
        *)
            err "unsupported OS: $(uname -s). Download a binary manually from https://github.com/1cli-team/one-cli/releases/latest"
            exit 1
            ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64)   ARCH="amd64" ;;
        arm64|aarch64)  ARCH="arm64" ;;
        *)
            err "unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
}

# ----- version --------------------------------------------------------------

resolve_version() {
    if [[ -z "${ONE_VERSION:-}" ]]; then
        info "resolving latest version from $ONE_LATEST_URL"
        local effective_url
        if ! effective_url=$(curl -fsSL --retry 3 -o /dev/null -w '%{url_effective}' "$ONE_LATEST_URL" 2>/dev/null); then
            err "failed to resolve latest GitHub release. Set ONE_VERSION=vX.Y.Z to bypass."
            exit 1
        fi
        ONE_VERSION="${effective_url##*/}"
        if [[ "$ONE_VERSION" == "latest" || "$ONE_VERSION" == "releases" || -z "$ONE_VERSION" ]]; then
            err "could not infer a release tag from $effective_url."
            err "Make sure the GitHub repo has at least one release, or set ONE_VERSION=vX.Y.Z."
            exit 1
        fi
    fi
    ONE_VERSION=$(printf '%s' "$ONE_VERSION" | tr -d '[:space:]')
    case "$ONE_VERSION" in
        v[0-9]*) ;;
        *) err "invalid version format: '$ONE_VERSION' (expected vX.Y.Z)"; exit 1 ;;
    esac
}

# current_version prints the version of an already-installed binary at
# $1, or empty if the file is missing / unreadable / version-flag fails.
# Output is normalized to "vX.Y.Z" so it lines up with $ONE_VERSION.
current_version() {
    local bin="$1"
    [[ -x "$bin" ]] || { printf ''; return; }
    local out
    out=$("$bin" --version 2>/dev/null | head -n1 | tr -d '[:space:]') || { printf ''; return; }
    [[ -n "$out" ]] || { printf ''; return; }
    case "$out" in
        v*) printf '%s' "$out" ;;
        *)  printf 'v%s' "$out" ;;
    esac
}

# version_compare returns:
#   0 if v1 == v2
#   1 if v1 >  v2
#   2 if v1 <  v2
# Pre-release suffixes (e.g. "-rc1") are stripped — the base X.Y.Z is
# what we compare. Pin via ONE_VERSION if you need finer control.
version_compare() {
    local v1="${1#v}" v2="${2#v}"
    v1="${v1%%-*}"; v2="${v2%%-*}"
    local IFS=.
    local -a a1=($v1) a2=($v2)
    local i n1 n2
    for i in 0 1 2; do
        n1="${a1[$i]:-0}"; n2="${a2[$i]:-0}"
        # Strip non-digit tails defensively (e.g. "1abc" → "1").
        n1="${n1//[^0-9]/}"; n2="${n2//[^0-9]/}"
        n1="${n1:-0}"; n2="${n2:-0}"
        if (( 10#$n1 > 10#$n2 )); then return 1; fi
        if (( 10#$n1 < 10#$n2 )); then return 2; fi
    done
    return 0
}

# ----- preflight ------------------------------------------------------------

preflight() {
    target="$ONE_INSTALL_DIR/one"

    if [[ -e "$target" ]]; then
        local existing
        existing=$(current_version "$target")
        if [[ -z "$existing" ]]; then
            # Binary exists but won't report a version (corrupt, old, or
            # something else writing to this path). Fall back to the
            # plain "force or fail" rule.
            if [[ "$ONE_FORCE" != "1" ]]; then
                err "$target exists but '--version' failed to read."
                err "Set ONE_FORCE=1 to overwrite, or remove the file."
                exit 1
            fi
            warn "overwriting unrecognised binary at $target (ONE_FORCE=1)"
        else
            set +e
            version_compare "$ONE_VERSION" "$existing"
            local cmp=$?
            set -e
            case "$cmp" in
                0)  # same version
                    if [[ "$ONE_FORCE" != "1" ]]; then
                        info "$target is already at $existing, skipping."
                        info "Set ONE_FORCE=1 to force a reinstall (e.g. to repair a corrupted binary)."
                        exit 0
                    fi
                    info "reinstalling $existing (ONE_FORCE=1)"
                    ;;
                1)  # target is newer
                    info "upgrading $existing → $ONE_VERSION"
                    ;;
                2)  # target is older
                    if [[ "$ONE_FORCE" != "1" ]]; then
                        err "downgrade blocked: installed $existing is newer than target $ONE_VERSION."
                        err "Re-run with ONE_FORCE=1 to allow the downgrade."
                        exit 1
                    fi
                    warn "downgrading $existing → $ONE_VERSION (ONE_FORCE=1)"
                    ;;
            esac
        fi
    fi

    for cmd in curl tar mktemp awk grep; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            err "required command not found: $cmd"
            exit 1
        fi
    done
    if [[ "$ONE_SKIP_VERIFY" != "1" ]]; then
        if command -v shasum >/dev/null 2>&1; then
            SHA256_CMD="shasum -a 256"
        elif command -v sha256sum >/dev/null 2>&1; then
            SHA256_CMD="sha256sum"
        else
            err "neither shasum nor sha256sum found; install one or set ONE_SKIP_VERIFY=1"
            exit 1
        fi
    fi
}

# ----- download -------------------------------------------------------------

download_and_verify() {
    archive="one-cli_${OS}_${ARCH}.tar.gz"
    archive_url="${ONE_RELEASE_BASE_URL%/}/$ONE_VERSION/$archive"
    checksum_url="${ONE_RELEASE_BASE_URL%/}/$ONE_VERSION/checksums.txt"

    TMP=$(mktemp -d -t one-cli-installer.XXXXXX)
    trap 'rm -rf "$TMP"' EXIT

    info "downloading $archive ($ONE_VERSION, $OS/$ARCH)"
    if ! curl -fsSL --retry 3 -o "$TMP/$archive" "$archive_url"; then
        err "failed to download $archive_url"
        exit 1
    fi

    if [[ "$ONE_SKIP_VERIFY" == "1" ]]; then
        warn "skipping SHA256 verification (ONE_SKIP_VERIFY=1)"
        return
    fi

    info "verifying SHA256"
    if ! curl -fsSL --retry 3 -o "$TMP/checksums.txt" "$checksum_url"; then
        err "failed to download $checksum_url"
        exit 1
    fi
    expected=$(grep "  $archive\$" "$TMP/checksums.txt" | awk '{print $1}')
    if [[ -z "$expected" ]]; then
        # Fall back to single-space (some sha tools emit one space).
        expected=$(grep " $archive\$" "$TMP/checksums.txt" | awk '{print $1}')
    fi
    if [[ -z "$expected" ]]; then
        err "no checksum entry for $archive in checksums.txt"
        exit 1
    fi
    actual=$($SHA256_CMD "$TMP/$archive" | awk '{print $1}')
    if [[ "$expected" != "$actual" ]]; then
        err "checksum mismatch for $archive:"
        err "  expected $expected"
        err "  got      $actual"
        exit 1
    fi
}

# ----- install --------------------------------------------------------------

install_binary() {
    info "extracting"
    tar -xzf "$TMP/$archive" -C "$TMP"

    if [[ ! -f "$TMP/one" ]]; then
        err "archive does not contain 'one' binary at top level"
        exit 1
    fi

    mkdir -p "$ONE_INSTALL_DIR"
    mv -f "$TMP/one" "$target"
    chmod +x "$target"
    green "one-cli: installed $target ($ONE_VERSION)"
}

# ----- PATH check -----------------------------------------------------------

check_path() {
    case ":$PATH:" in
        *":$ONE_INSTALL_DIR:"*) return ;;
    esac
    warn "$ONE_INSTALL_DIR is not in PATH."
    warn ""
    warn "Add it to your shell profile:"
    warn "  zsh:  echo 'export PATH=\"$ONE_INSTALL_DIR:\$PATH\"' >> ~/.zshrc"
    warn "  bash: echo 'export PATH=\"$ONE_INSTALL_DIR:\$PATH\"' >> ~/.bashrc"
    warn ""
    warn "Then start a new shell and run: one --version"
    exit 0
}

main "$@"
