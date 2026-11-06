package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"gopkg.in/yaml.v3"
)

// FlutterProject reads native pub metadata without inventing a second dependency format.
type FlutterProject struct {
	Name                string            `yaml:"name"`
	Environment         map[string]string `yaml:"environment"`
	Resolution          string            `yaml:"resolution"`
	Workspace           []string          `yaml:"workspace"`
	Dependencies        map[string]any    `yaml:"dependencies"`
	DevDependencies     map[string]any    `yaml:"dev_dependencies"`
	DependencyOverrides map[string]any    `yaml:"dependency_overrides"`
}

var flutterPackageName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func ParseFlutterProject(raw []byte) (FlutterProject, error) {
	var project FlutterProject
	if err := yaml.Unmarshal(raw, &project); err != nil {
		return project, err
	}
	if project.Name != "" && !flutterPackageName.MatchString(project.Name) {
		return project, i18n.Errorf("flutter.name_invalid", project.Name)
	}
	return project, nil
}

// FlutterBuildDependencies mirrors pub workspace resolution, including local
// version dependencies, path dependencies and dependency overrides.
func FlutterBuildDependencies(root string, projects []Project, read func(string) ([]byte, error)) (map[string][]string, error) {
	result := map[string][]string{}
	var members []Project
	for _, p := range projects {
		if p.Toolchain == "flutter" {
			members = append(members, p)
		}
	}
	if len(members) == 0 {
		return result, nil
	}
	if read == nil {
		read = func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(root, name)) }
	}
	raw, err := read("pubspec.yaml")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	rootProject, err := ParseFlutterProject(raw)
	if err != nil {
		return nil, fmt.Errorf("pubspec.yaml: %w", err)
	}
	if err := mergeFlutterOverrides(&rootProject, "pubspec_overrides.yaml", read); err != nil {
		return nil, err
	}
	names, dirs := map[string]string{}, map[string]string{}
	metadata := map[string]FlutterProject{}
	for _, p := range members {
		file := filepath.ToSlash(filepath.Join(p.RelativeDir, "pubspec.yaml"))
		raw, err := read(file)
		if err != nil {
			return nil, err
		}
		parsed, err := ParseFlutterProject(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if parsed.Name == "" {
			return nil, i18n.Errorf("flutter.name_missing", file)
		}
		if err := mergeFlutterOverrides(&parsed, filepath.ToSlash(filepath.Join(p.RelativeDir, "pubspec_overrides.yaml")), read); err != nil {
			return nil, err
		}
		if previous, exists := names[parsed.Name]; exists {
			return nil, i18n.Errorf("build.duplicate_package", parsed.Name, previous, p.Name)
		}
		names[parsed.Name], dirs[filepath.Clean(p.RelativeDir)] = p.Name, p.Name
		metadata[p.Name] = parsed
	}
	for _, p := range members {
		parsed := metadata[p.Name]
		deps := map[string]bool{}
		for _, set := range []map[string]any{parsed.Dependencies, parsed.DevDependencies} {
			for name, source := range set {
				base := p.RelativeDir
				if override, ok := parsed.DependencyOverrides[name]; ok {
					source = override
				}
				if override, ok := rootProject.DependencyOverrides[name]; ok {
					source, base = override, "."
				}
				target := names[name]
				if target == "" {
					if fields, ok := source.(map[string]any); ok {
						if path, ok := fields["path"].(string); ok {
							target = dirs[filepath.Clean(filepath.Join(base, path))]
						}
					}
				}
				if target != "" && target != p.Name {
					deps[target] = true
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

func mergeFlutterOverrides(project *FlutterProject, file string, read func(string) ([]byte, error)) error {
	raw, err := read(file)
	if os.IsNotExist(err) || (err == nil && raw == nil) {
		return nil
	}
	if err != nil {
		return err
	}
	overrides, err := ParseFlutterProject(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	if project.DependencyOverrides == nil {
		project.DependencyOverrides = map[string]any{}
	}
	for name, source := range overrides.DependencyOverrides {
		project.DependencyOverrides[name] = source
	}
	return nil
}
