package infisical

import (
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/secrets"
)

// SanitizeEnvName uses the shared environment-name contract.
func SanitizeEnvName(s string) (string, error) { return secrets.ValidateEnvName(s) }

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

// ResolveSubprojectPath derives the remote folder from the local project path.
func ResolveSubprojectPath(sub *workspace.Project) PathResolution {
	if sub == nil {
		return PathResolution{Path: "/", Chain: []string{"/"}}
	}
	path := NormalizePath("/" + sub.RelativeDir)
	return PathResolution{Path: path, Inherits: true, Chain: pathInheritanceChain("/", path, true)}
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
