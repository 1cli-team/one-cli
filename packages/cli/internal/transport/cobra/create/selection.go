package createcmd

import (
	"path/filepath"
	"strings"
)

// domainOf extracts the domain component of a namespaced id
// ("env/infisical" -> "env"). Returns "" if the id has no slash.
func domainOf(id string) string {
	if i := strings.Index(id, "/"); i > 0 {
		return id[:i]
	}
	return ""
}

func selectedEnvironmentBackend(backends []string) string {
	for _, id := range backends {
		if domainOf(id) == "env" {
			return strings.TrimPrefix(id, "env/")
		}
	}
	return ""
}

// resolveTargetPath turns a user-supplied target ("." | "./foo" | absolute |
// relative) into an absolute path rooted at cwd. Used by both the pre-form
// validator and the post-form scaffold step so they always agree on what
// directory we're talking about.
func resolveTargetPath(cwd, raw string) string {
	switch raw {
	case ".", "./":
		return cwd
	}
	if filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(cwd, raw)
}
