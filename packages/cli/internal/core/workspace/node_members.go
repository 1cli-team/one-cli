package workspace

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// NodeProjectPackageDirs returns a logical project's package and the members
// declared in its package.json workspaces. Paths are relative to the One root.
// A composite project stays one manifest entry while its packages share the
// root package manager and dependency cache. Read can be a FilePlan overlay.
func NodeProjectPackageDirs(root, dir string, read func(string) ([]byte, error)) ([]string, error) {
	if read == nil {
		read = func(name string) ([]byte, error) {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if os.IsNotExist(err) {
				return nil, nil
			}
			return b, err
		}
	}
	seen := map[string]bool{}
	var visit func(string) error
	visit = func(dir string) error {
		dir = filepath.ToSlash(filepath.Clean(dir))
		if seen[dir] {
			return nil
		}
		if err := fsutil.SafeWritePath(root, filepath.Join(root, dir, "package.json")); err != nil {
			return err
		}
		seen[dir] = true
		raw, err := read(path.Join(dir, "package.json"))
		if err != nil || raw == nil {
			return err
		}
		var pkg struct {
			Workspaces json.RawMessage `json:"workspaces"`
		}
		if err := json.Unmarshal(raw, &pkg); err != nil {
			return err
		}
		if len(pkg.Workspaces) == 0 {
			return nil
		}
		var patterns []string
		if err := json.Unmarshal(pkg.Workspaces, &patterns); err != nil {
			var object struct {
				Packages []string `json:"packages"`
			}
			if err := json.Unmarshal(pkg.Workspaces, &object); err != nil {
				return i18n.Errorf("creation.workspaces_invalid")
			}
			patterns = object.Packages
		}
		members := map[string]bool{}
		var exclusions []string
		for _, pattern := range patterns {
			negative := strings.HasPrefix(pattern, "!")
			pattern = strings.TrimPrefix(strings.TrimPrefix(pattern, "!"), "./")
			// Keep discovery bounded to this project. Complex glob syntax is
			// rejected rather than silently registering only some members.
			if pattern == "" || path.IsAbs(pattern) || strings.Contains(pattern, "\\") || strings.ContainsAny(pattern, "{}()") || strings.Contains(pattern, "**") {
				return i18n.Errorf("workspace.node_member_pattern", dir, pattern)
			}
			for _, segment := range strings.Split(pattern, "/") {
				if segment == ".." || segment == "node_modules" || segment == ".git" {
					return i18n.Errorf("workspace.node_member_pattern", dir, pattern)
				}
			}
			if _, err := path.Match(pattern, ""); err != nil {
				return i18n.Errorf("workspace.node_member_pattern", dir, pattern)
			}
			if negative {
				exclusions = append(exclusions, pattern)
				continue
			}
			// Validate the literal prefix before Glob can traverse a symlink.
			prefix := strings.Split(pattern, "/")
			safe := dir
			for _, segment := range prefix {
				if strings.ContainsAny(segment, "*?[") {
					break
				}
				safe = path.Join(safe, segment)
			}
			if err := fsutil.SafeWritePath(root, filepath.Join(root, safe, "package.json")); err != nil {
				return err
			}
			matches, err := filepath.Glob(filepath.Join(root, dir, filepath.FromSlash(pattern), "package.json"))
			if err != nil {
				return err
			}
			for _, match := range matches {
				if err := fsutil.SafeWritePath(root, match); err != nil {
					return err
				}
				rel, err := filepath.Rel(filepath.Join(root, dir), filepath.Dir(match))
				if err != nil {
					return err
				}
				members[filepath.ToSlash(rel)] = true
			}
		}
		var sorted []string
		for member := range members {
			excluded := false
			for _, pattern := range exclusions {
				match, _ := path.Match(pattern, member)
				excluded = excluded || match
			}
			if !excluded {
				sorted = append(sorted, member)
			}
		}
		sort.Strings(sorted)
		for _, member := range sorted {
			if err := visit(path.Join(dir, member)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(dir); err != nil {
		return nil, err
	}
	dirs := make([]string, 0, len(seen))
	for dir := range seen {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs, nil
}
