// Package gowork coordinates Go workspace membership without invoking Go or
// changing module requirements. It uses Go's modfile parser to retain comments.
package gowork

import (
	"fmt"
	"go/version"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

type Reader func(string) ([]byte, error)

type Info struct {
	Version string
	Modules []string // absolute directories, including user-owned external members
}

func maximum(a, b string) string {
	if version.Compare("go"+a, "go"+b) < 0 {
		return b
	}
	return a
}

func normalize(v string) string {
	if strings.Count(v, ".") == 1 {
		return v + ".0"
	}
	return v
}

func moduleInfo(path string, read Reader) (*modfile.File, string, error) {
	b, err := read(filepath.Join(path, "go.mod"))
	if err != nil {
		return nil, "", err
	}
	if b == nil {
		return nil, "", fmt.Errorf("missing go.mod in workspace member %s (module moved or removed)", path)
	}
	f, err := modfile.Parse(filepath.Join(path, "go.mod"), b, nil)
	if err != nil {
		return nil, "", err
	}
	if f.Module == nil {
		return nil, "", fmt.Errorf("%s/go.mod has no module directive", path)
	}
	v := "1.18.0"
	if f.Go != nil {
		v = normalize(f.Go.Version)
	}
	if f.Toolchain != nil && f.Toolchain.Name != "default" {
		v = maximum(v, strings.TrimPrefix(f.Toolchain.Name, "go"))
	}
	return f, v, nil
}

func inspect(root string, f *modfile.WorkFile, read Reader) (Info, error) {
	info := Info{Version: "1.18.0"}
	if f.Go != nil {
		info.Version = maximum(info.Version, normalize(f.Go.Version))
	}
	if f.Toolchain != nil && f.Toolchain.Name != "default" {
		info.Version = maximum(info.Version, strings.TrimPrefix(f.Toolchain.Name, "go"))
	}
	modules := map[string]string{}
	dirs := map[string]bool{}
	for _, use := range f.Use {
		dir := use.Path
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, filepath.FromSlash(dir))
		}
		dir = filepath.Clean(dir)
		if dirs[dir] {
			return Info{}, fmt.Errorf("duplicate go.work member %s", use.Path)
		}
		dirs[dir] = true
		m, v, err := moduleInfo(dir, read)
		if err != nil {
			return Info{}, err
		}
		if previous, ok := modules[m.Module.Mod.Path]; ok {
			return Info{}, fmt.Errorf("duplicate module path %s in %s and %s", m.Module.Mod.Path, previous, dir)
		}
		modules[m.Module.Mod.Path] = dir
		info.Modules = append(info.Modules, dir)
		info.Version = maximum(info.Version, v)
	}
	sort.Strings(info.Modules)
	return info, nil
}

// Inspect reads an existing workspace; absent go.work returns empty Info.
func Inspect(root string, read Reader) (Info, error) {
	b, err := read(filepath.Join(root, "go.work"))
	if err != nil || b == nil {
		return Info{}, err
	}
	f, err := modfile.ParseWork(filepath.Join(root, "go.work"), b, nil)
	if err != nil {
		return Info{}, err
	}
	return inspect(root, f, read)
}

// Build initializes on the first module and merges subsequent use entries.
// It never removes user members, replaces, comments, or toolchain directives.
func Build(root string, members []string, read Reader) ([]byte, Info, error) {
	b, err := read(filepath.Join(root, "go.work"))
	if err != nil {
		return nil, Info{}, err
	}
	if b == nil && len(members) == 0 {
		return nil, Info{}, nil
	}
	initial := b == nil
	if initial {
		b = []byte("// Go workspace initialized by One CLI. Add modules with one add.\ngo 1.18.0\n")
	}
	f, err := modfile.ParseWork(filepath.Join(root, "go.work"), b, nil)
	if err != nil {
		return nil, Info{}, err
	}
	existing := map[string]bool{}
	for _, u := range f.Use {
		path := u.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, filepath.FromSlash(path))
		}
		existing[filepath.Clean(path)] = true
	}
	changed := initial
	sort.Strings(members)
	for _, member := range members {
		dir := filepath.Join(root, filepath.FromSlash(member))
		if existing[dir] {
			continue
		}
		if err := f.AddUse("./"+filepath.ToSlash(member), ""); err != nil {
			return nil, Info{}, err
		}
		existing[dir], changed = true, true
	}
	info, err := inspect(root, f, read)
	if err != nil {
		return nil, Info{}, err
	}
	if f.Go == nil || version.Compare("go"+f.Go.Version, "go"+info.Version) < 0 {
		if err := f.AddGoStmt(info.Version); err != nil {
			return nil, Info{}, err
		}
		changed = true
	}
	if !changed {
		return b, info, nil
	}
	f.Cleanup()
	return modfile.Format(f.Syntax), info, nil
}
