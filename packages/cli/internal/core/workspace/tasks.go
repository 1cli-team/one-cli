package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"gopkg.in/yaml.v3"
)

// ProjectTask describes a native task. Commands remain in package.json/Taskfile.
type ProjectTask struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Argv        []string `json:"argv"`
	Source      string   `json:"source"`
	Interactive bool     `json:"interactive"`
	Inputs      []string `json:"inputs,omitempty"`
}

// DiscoverTasks only reads workspace files, including creation's unpublished overlay.
func DiscoverTasks(root string, p Project, read func(string) ([]byte, error)) ([]ProjectTask, error) {
	if read == nil {
		read = func(rel string) ([]byte, error) {
			b, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if os.IsNotExist(e) {
				return nil, nil
			}
			return b, e
		}
	}
	var result []ProjectTask
	switch p.Toolchain {
	case "node":
		if p.PackageManager != "" && p.PackageManager != "pnpm" {
			return nil, i18n.Errorf("workspace.package_manager_unsupported", p.PackageManager)
		}
		source := p.RelativeDir + "/package.json"
		raw, err := read(source)
		if err != nil {
			return nil, err
		}
		var pkg struct {
			Scripts        map[string]string `json:"scripts"`
			PackageManager string            `json:"packageManager"`
		}
		if err = json.Unmarshal(raw, &pkg); err != nil {
			return nil, err
		}
		if pkg.PackageManager != "" && pkg.PackageManager != "pnpm" && !strings.HasPrefix(pkg.PackageManager, "pnpm@") {
			return nil, i18n.Errorf("workspace.package_manager_unsupported", pkg.PackageManager)
		}
		for name, command := range pkg.Scripts {
			if strings.TrimSpace(command) != "" {
				result = append(result, ProjectTask{Name: name, Argv: []string{"pnpm", "run", name}, Source: source, Interactive: name == "dev" || strings.Contains(name, "watch"), Inputs: []string{source}})
			}
		}
	case "go":
		var walk func(string, string, map[string]bool) ([]ProjectTask, error)
		walk = func(file, prefix string, seen map[string]bool) ([]ProjectTask, error) {
			file = filepath.ToSlash(filepath.Clean(file))
			if filepath.IsAbs(file) || file == ".." || strings.HasPrefix(file, "../") || strings.Contains(file, "{{") || strings.Contains(file, "://") {
				return nil, i18n.Errorf("tasks.dynamic_taskfile", file)
			}
			if seen[file] {
				return nil, i18n.Errorf("tasks.include_cycle", file)
			}
			raw, err := read(file)
			if err != nil {
				return nil, err
			}
			if raw == nil {
				return nil, nil
			}
			seen[file] = true
			defer delete(seen, file)
			var doc struct {
				Tasks    map[string]yaml.Node `yaml:"tasks"`
				Includes map[string]yaml.Node `yaml:"includes"`
			}
			if err = yaml.Unmarshal(raw, &doc); err != nil {
				return nil, err
			}
			var tasks []ProjectTask
			for name, node := range doc.Tasks {
				var meta struct {
					Desc     string `yaml:"desc"`
					Internal bool   `yaml:"internal"`
				}
				if node.Kind == yaml.MappingNode {
					if err = node.Decode(&meta); err != nil {
						return nil, err
					}
				}
				if meta.Internal {
					continue
				}
				name = prefix + name
				tasks = append(tasks, ProjectTask{Name: name, Description: meta.Desc, Argv: []string{"task", name}, Source: file, Interactive: name == "dev" || strings.Contains(name, "watch"), Inputs: []string{file}})
			}
			keys := make([]string, 0, len(doc.Includes))
			for key := range doc.Includes {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				node := doc.Includes[key]
				var include struct {
					Taskfile string `yaml:"taskfile"`
					Optional bool   `yaml:"optional"`
				}
				if node.Kind == yaml.ScalarNode {
					include.Taskfile = node.Value
				} else if err = node.Decode(&include); err != nil {
					return nil, err
				}
				if include.Taskfile == "" || strings.Contains(include.Taskfile, "{{") || strings.Contains(include.Taskfile, "://") {
					return nil, i18n.Errorf("tasks.dynamic_taskfile", file)
				}
				target := filepath.ToSlash(filepath.Join(filepath.Dir(file), include.Taskfile))
				// Task also permits an included directory.
				if filepath.Ext(target) == "" {
					target += "/Taskfile.yml"
				}
				nested, err := walk(target, prefix+key+":", seen)
				if err != nil {
					return nil, err
				}
				if len(nested) == 0 && !include.Optional {
					return nil, i18n.Errorf("tasks.dynamic_taskfile", target)
				}
				tasks = append(tasks, nested...)
			}
			for i := range tasks {
				tasks[i].Inputs = append(tasks[i].Inputs, file)
			}
			return tasks, nil
		}
		for _, name := range []string{"Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "taskfile.yaml"} {
			file := p.RelativeDir + "/" + name
			raw, err := read(file)
			if err != nil {
				return nil, err
			}
			if raw == nil {
				continue
			}
			result, err = walk(file, "", map[string]bool{})
			if err != nil {
				return nil, err
			}
			break
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

type taskPackage struct {
	Name                 string            `json:"name"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	Directory            string            `json:"-"`
}

// BuildDependencies identifies local package dependencies, including composite projects.
// It returns every source dependency; callers decide whether it needs a build task.
func BuildDependencies(root string, projects []Project, read func(string) ([]byte, error)) (map[string][]string, error) {
	if read == nil {
		read = func(rel string) ([]byte, error) { return os.ReadFile(filepath.Join(root, rel)) }
	}
	names, dirs := map[string]string{}, map[string]string{}
	packages := map[string][]taskPackage{}
	result := map[string][]string{}
	for _, p := range projects {
		if p.Toolchain != "node" {
			continue
		}
		members, err := NodeProjectPackageDirs(root, p.RelativeDir, read)
		if err != nil {
			return nil, err
		}
		for _, dir := range members {
			raw, err := read(filepath.ToSlash(filepath.Join(dir, "package.json")))
			if err != nil {
				return nil, err
			}
			var pkg taskPackage
			if err = json.Unmarshal(raw, &pkg); err != nil {
				return nil, err
			}
			pkg.Directory = dir
			if previous, ok := names[pkg.Name]; pkg.Name != "" && ok {
				return nil, i18n.Errorf("build.duplicate_package", pkg.Name, previous, p.Name)
			}
			if pkg.Name != "" {
				names[pkg.Name] = p.Name
			}
			dirs[filepath.Clean(dir)] = p.Name
			packages[p.Name] = append(packages[p.Name], pkg)
		}
	}
	for _, p := range projects {
		deps := map[string]bool{}
		for _, pkg := range packages[p.Name] {
			for _, set := range []map[string]string{pkg.Dependencies, pkg.DevDependencies, pkg.OptionalDependencies} {
				for name, spec := range set {
					target := names[name]
					for _, prefix := range []string{"file:", "link:"} {
						if strings.HasPrefix(spec, prefix) {
							target = dirs[filepath.Clean(filepath.Join(pkg.Directory, strings.TrimPrefix(spec, prefix)))]
						}
					}
					if target != "" && target != p.Name {
						deps[target] = true
					}
				}
			}
		}
		for dep := range deps {
			result[p.Name] = append(result[p.Name], dep)
		}
		sort.Strings(result[p.Name])
	}
	return result, nil
}
