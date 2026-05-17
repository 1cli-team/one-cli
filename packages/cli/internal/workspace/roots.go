package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// DefaultRootDirs are the directories `discoverProjects` walks when
// scanning a workspace. These are hard-wired: workspaces always
// follow the layout produced by `one create`. (User overrides via
// `manifest.workspace.roots` were dropped from the manifest.)
var DefaultRootDirs = []string{"apps", "services", "packages"}

// PackageJSON captures the slice of package.json that One CLI cares about.
// As of manifest v2 the workspace marker / configuration moved out of
// package.json entirely; this struct only keeps the fields needed for
// dependency-based project classification.
type PackageJSON struct {
	Name            string            `json:"name"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
}

// ReadPackageJSON loads the package.json at projectRoot. Returns (nil, nil)
// when the file does not exist — caller decides if that's an error.
func ReadPackageJSON(projectRoot string) (*PackageJSON, error) {
	path := filepath.Join(projectRoot, "package.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var p PackageJSON
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ResolveRootDirs returns the workspace scan-root dirs. The current manifest hard-wires the
// list to DefaultRootDirs; the override parameter is preserved only for
// backwards-compatible call sites passing nil. Non-nil override is honored
// (it lets `--roots` flag work for ad-hoc invocations) but the manifest
// itself never carries a roots override.
func ResolveRootDirs(_ string, override []string) ([]string, error) {
	if len(override) > 0 {
		return dedupe(override), nil
	}
	return append([]string{}, DefaultRootDirs...), nil
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
