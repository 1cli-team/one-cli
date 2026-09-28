package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// PackageJSON captures the slice of package.json that One CLI cares about.
// These fields identify the package manager and dependencies used by project summaries.
type PackageJSON struct {
	Name            string            `json:"name"`
	PackageManager  string            `json:"packageManager,omitempty"`
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
