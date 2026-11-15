package mirror

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

// No broad github.com rule: only public Process Compose artifacts and metadata
// can pass through this proxy. Queries and embedded credentials do not match.
func composeReplacements(base string) map[string]string {
	proxy := strings.TrimSuffix(base, composeReleases)
	return map[string]string{
		`regex:^https://github\.com/F1bonacc1/process-compose/releases/download/(v[^/?#]+/[^/?#]+)$`:                                 strings.TrimRight(base, "/") + "/$1",
		`regex:^https://api\.github\.com/repos/F1bonacc1/process-compose/(releases(?:/tags/v[^/?#]+)?|tags)$`:                        proxy + "https://api.github.com/repos/F1bonacc1/process-compose/$1",
		`regex:^https://raw\.githubusercontent\.com/aquaproj/aqua-registry/([^/?#]+)/pkgs/F1bonacc1/process-compose/registry\.yaml$`: proxy + "https://raw.githubusercontent.com/aquaproj/aqua-registry/$1/pkgs/F1bonacc1/process-compose/registry.yaml",
	}
}

var githubCredentialKeys = []string{"MISE_GITHUB_TOKEN", "GITHUB_API_TOKEN", "GITHUB_TOKEN", "GH_TOKEN", "MISE_GITHUB_ENTERPRISE_TOKEN", "GH_ENTERPRISE_TOKEN", "MISE_GITHUB_CREDENTIAL_COMMAND", "MISE_GITHUB_USE_GIT_CREDENTIALS", "MISE_GITHUB_RELAY_SOCKET", "NETRC", "NETRC_FILE"}

// Be conservative when credentials might be resolved outside the environment.
// Read only existence/config keys, never credential values or helper output.
func toolCredentials(env []string, existing inspection) (string, error) {
	if existing.credentials {
		return "mise", nil
	}
	for _, key := range githubCredentialKeys {
		if value, ok := lookup(env, key); ok && value != "" {
			return "environment:" + key, nil
		}
	}
	home, err := userdirs.Home()
	if err != nil {
		return "", err
	}
	dirs, err := userdirs.Resolve()
	if err != nil {
		return "", err
	}
	xdg, _ := lookup(env, "XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	paths := []string{filepath.Join(home, ".netrc"), filepath.Join(home, "_netrc"), filepath.Join(home, ".git-credentials"), filepath.Join(xdg, "git", "credentials"), filepath.Join(xdg, "gh", "hosts.yml")}
	for _, root := range []string{filepath.Join(xdg, "mise"), filepath.Join(dirs.Config, "mise")} {
		paths = append(paths, filepath.Join(root, "github_tokens.toml"))
	}
	stateHome, _ := lookup(env, "XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	for _, root := range []string{filepath.Join(stateHome, "mise"), filepath.Join(dirs.State, "mise")} {
		paths = append(paths, filepath.Join(root, "github-oauth-tokens.toml"))
	}
	if root, ok := lookup(env, "MISE_STATE_DIR"); ok && root != "" {
		paths = append(paths, filepath.Join(root, "github-oauth-tokens.toml"))
	}
	for _, key := range []string{"GH_CONFIG_DIR", "MISE_CONFIG_DIR"} {
		if root, ok := lookup(env, key); ok && root != "" {
			if key == "GH_CONFIG_DIR" {
				paths = append(paths, filepath.Join(root, "hosts.yml"))
			} else {
				paths = append(paths, filepath.Join(root, "github_tokens.toml"))
			}
		}
	}
	if appdata, _ := lookup(env, "APPDATA"); appdata != "" {
		paths = append(paths, filepath.Join(appdata, "GitHub CLI", "hosts.yml"))
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", i18n.Errorf("mirror.inspect_failed", path, err)
		}
	}
	return "", nil
}
