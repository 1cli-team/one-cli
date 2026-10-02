// Package templatefiles defines the shared, runtime-independent bundle layout.
// Keep this package standard-library-only: the resource builder imports it.
package templatefiles

import (
	"path"
	"strings"
)

const PackedGoMod = "_go.mod"

// Excluded applies to packaging and generation, including local build artifacts.
func Excluded(name string) bool {
	for _, part := range strings.Split(name, "/") {
		switch part {
		case ".git", ".one", ".agents", "AGENTS.md", "CLAUDE.md", "SKILL.md",
			"node_modules", "dist", "out", "build", "bin", "coverage", ".next", ".source", ".astro", ".expo", ".turbo", ".cache",
			"pnpm-lock.yaml", "package-lock.json", "npm-shrinkwrap.json", "yarn.lock", "bun.lock", "bun.lockb",
			"go.work", "go.work.sum", "coverage.out", ".DS_Store":
			return true
		}
	}
	return strings.HasSuffix(name, ".tsbuildinfo")
}
func PackPath(name string) string {
	if path.Base(name) == "go.mod" {
		return path.Join(path.Dir(name), PackedGoMod)
	}
	return name
}
func LogicalPath(name string) string {
	if path.Base(name) == PackedGoMod {
		return path.Join(path.Dir(name), "go.mod")
	}
	return name
}
