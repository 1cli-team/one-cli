package docker

// version.go: fall-back chain for the image-tag version when the
// caller doesn't pass `--build-version`. Order:
//   manifest.buildVersion → `git describe --tags --exact-match HEAD`
//   → package.json#version (subproject) → package.json#version (root)
//   → workspace.DefaultBuildVersion
//
// Exposed via DefaultImageVersion so containercmd can decide whether
// an interactive prompt is necessary before invoking Build.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func defaultImageVersion(projectRoot, projectDir, projectName string) (string, error) {
	if m, err := workspace.ReadManifest(projectRoot); err == nil {
		if v := workspace.BuildVersionForProject(m, projectName); v != "" {
			return workspace.BuildTagForVersion(v), nil
		}
	}
	if v := gitExactTag(projectDir); v != "" {
		return v, nil
	}
	if v := packageVersion(projectDir); v != "" {
		return v, nil
	}
	if v := packageVersion(projectRoot); v != "" {
		return v, nil
	}
	return workspace.BuildTagForVersion(workspace.DefaultBuildVersion), nil
}

// DefaultImageVersion exposes the build tag fallback chain for callers
// that need to decide whether an interactive tag prompt is necessary
// before invoking Build.
func DefaultImageVersion(projectRoot, projectDir, projectName string) (string, error) {
	return defaultImageVersion(projectRoot, projectDir, projectName)
}

func gitExactTag(projectRoot string) string {
	c := exec.Command("git", "describe", "--tags", "--exact-match", "HEAD")
	c.Dir = projectRoot
	out, err := c.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func packageVersion(dir string) string {
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var doc struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	version := strings.TrimSpace(doc.Version)
	if version == "" || version == "0.0.0" {
		return ""
	}
	return version
}
