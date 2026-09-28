package miseconfig

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"golang.org/x/mod/modfile"
)

func dependencyClosure(name string, edges map[string][]string) ([]string, error) {
	seen := map[string]int{}
	result := []string{}
	var walk func(string) error
	walk = func(current string) error {
		if seen[current] == 1 {
			return i18n.Errorf("build.dependency_cycle", name+" -> "+current)
		}
		if seen[current] == 2 {
			return nil
		}
		seen[current] = 1
		for _, dep := range edges[current] {
			if err := walk(dep); err != nil {
				return err
			}
		}
		seen[current] = 2
		if current != name {
			result = append(result, current)
		}
		return nil
	}
	err := walk(name)
	sort.Strings(result)
	return result, err
}

// Add every Go workspace member and local replace directory, including external
// members. Changes outside the selected module must invalidate its build.
func (p *Plan) goCacheSources(root, projectDir string, members []string) ([]string, error) {
	dirs := map[string]bool{}
	pending := append([]string{projectDir}, members...)
	read := func(path string) ([]byte, error) {
		rel, _ := filepath.Rel(root, path)
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			raw, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				err = nil
			}
			if err == nil {
				p.inputs[path] = raw
			}
			return raw, err
		}
		return p.readOptional(filepath.ToSlash(rel))
	}
	addReplaces := func(base string, replaces []*modfile.Replace) {
		for _, r := range replaces {
			if r.New.Version == "" {
				dir := r.New.Path
				if !filepath.IsAbs(dir) {
					dir = filepath.Join(base, dir)
				}
				pending = append(pending, dir)
			}
		}
	}
	raw, err := read(filepath.Join(root, "go.work"))
	if err != nil {
		return nil, err
	}
	if raw != nil {
		f, err := modfile.ParseWork("go.work", raw, nil)
		if err != nil {
			return nil, err
		}
		addReplaces(root, f.Replace)
	}
	for len(pending) > 0 {
		dir := filepath.Clean(pending[0])
		pending = pending[1:]
		if dirs[dir] {
			continue
		}
		dirs[dir] = true
		raw, err := read(filepath.Join(dir, "go.mod"))
		if err != nil {
			return nil, err
		}
		if raw == nil {
			continue
		}
		f, err := modfile.Parse("go.mod", raw, nil)
		if err != nil {
			return nil, err
		}
		addReplaces(dir, f.Replace)
	}
	sources := []string{}
	for dir := range dirs {
		if dir == projectDir {
			continue
		}
		rel, _ := filepath.Rel(projectDir, dir)
		rel = filepath.ToSlash(rel)
		sources = append(sources, rel+"/**/*")
		for _, exclude := range []string{".git", ".mise", "node_modules", "bin", "dist", ".cache"} {
			sources = append(sources, "!"+strings.TrimSuffix(rel, "/")+"/"+exclude+"/**")
		}
	}
	sort.Strings(sources)
	return sources, nil
}
