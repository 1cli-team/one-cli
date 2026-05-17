package infisical

import (
	"regexp"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// envNameRE constrains environment names: must start with a letter or
// digit, and contain only letters / digits / hyphens / underscores.
var envNameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_]*$`)

// SanitizeEnvName trims whitespace, validates the pattern, and returns the
// canonical env name. Returns ENV_INVALID_ENV_NAME on bad input.
func SanitizeEnvName(s string) (string, error) {
	v := strings.TrimSpace(s)
	if !envNameRE.MatchString(v) {
		return "", cliErrors.New(cliErrors.ENV_INVALID_ENV_NAME,
			"环境名称非法："+s+"（必须匹配 ^[a-zA-Z0-9][a-zA-Z0-9-_]*$，例如 dev / staging / prod）")
	}
	return v, nil
}

// AssertValidKey validates a secret key against the POSIX env-var pattern.
// Delegates to internal/secrets — the validation is a cross-backend concern
// and the canonical implementation lives there. Returns ENV_INVALID_KEY.
func AssertValidKey(s string) error { return secrets.AssertValidKey(s) }

// NormalizePath canonicalises a user-supplied Infisical folder path.
// Always returns a leading slash and no trailing slash (except the root /).
// "." / "" → "/", "//x///y/" → "/x/y", "x/y" → "/x/y".
func NormalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	if p == "" || p == "." || p == "/" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// collapse repeated slashes and trim trailing
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}

// PathResolution holds the resolved Infisical path for a subproject and the
// chain of parent paths to inherit from when --inherits is on.
type PathResolution struct {
	Path     string   // e.g. "/services/user-api"
	Inherits bool     // whether to merge parent folder keys
	Chain    []string // root → ancestors → self, for merge order
}

// ResolveSubprojectPath returns the Infisical folder path a given subproject
// maps to. Resolution order:
//  1. one.manifest.json subproject entry's env.path (explicit override)
//  2. derive from relativeDir (default: "/" + relativeDir)
//
// For workspace-root operations, pass relativeDir="" to use the workspace's
// rootPath verbatim.
func ResolveSubprojectPath(workspaceCfg *WorkspaceConfig, sub *workspace.Project, override *SubprojectConfig) PathResolution {
	if sub == nil {
		// Workspace-root scope: just the configured rootPath.
		return PathResolution{
			Path:     NormalizePath(workspaceCfg.RootPathOrDefault()),
			Inherits: false,
			Chain:    []string{NormalizePath(workspaceCfg.RootPathOrDefault())},
		}
	}
	inherits := true
	path := ""
	if override != nil {
		if override.Inherits != nil {
			inherits = *override.Inherits
		}
		path = strings.TrimSpace(override.Path)
	}
	if path == "" {
		// Default: derive from relativeDir. e.g. "services/user-api" →
		// "/services/user-api". On Windows, relativeDir is already POSIX
		// (DiscoverSubprojects normalizes it).
		path = "/" + sub.RelativeDir
	}
	path = NormalizePath(path)
	return PathResolution{
		Path:     path,
		Inherits: inherits,
		Chain:    pathInheritanceChain(workspaceCfg.RootPathOrDefault(), path, inherits),
	}
}

// pathInheritanceChain returns the merge order for a pull. With inherits
// turned off the chain is just [path]. With inherits on it starts at the
// workspace root and walks down each segment so a key at /services
// overrides /, and a key at /services/user-api overrides /services.
func pathInheritanceChain(rootPath, path string, inherits bool) []string {
	rootPath = NormalizePath(rootPath)
	path = NormalizePath(path)
	if !inherits || path == rootPath {
		return []string{path}
	}
	// Walk from rootPath down to path, accumulating segments.
	chain := []string{rootPath}
	rel := strings.TrimPrefix(path, rootPath)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return chain
	}
	parts := strings.Split(rel, "/")
	cur := rootPath
	for _, p := range parts {
		if p == "" {
			continue
		}
		if cur == "/" {
			cur = "/" + p
		} else {
			cur = cur + "/" + p
		}
		chain = append(chain, cur)
	}
	return chain
}
