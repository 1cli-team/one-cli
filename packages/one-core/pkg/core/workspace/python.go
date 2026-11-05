package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

// PythonProject reads the dependency declarations owned by Python and uv.
// Unrelated pyproject fields remain in the source document.
type PythonProject struct {
	Project struct {
		Name                 string              `toml:"name"`
		RequiresPython       string              `toml:"requires-python"`
		Dependencies         []string            `toml:"dependencies"`
		OptionalDependencies map[string][]string `toml:"optional-dependencies"`
		Scripts              map[string]string   `toml:"scripts"`
	} `toml:"project"`
	DependencyGroups map[string][]any `toml:"dependency-groups"`
	Tool             struct {
		One struct {
			Tasks map[string]PythonTask `toml:"tasks"`
		} `toml:"one"`
		UV struct {
			Sources   map[string]any `toml:"sources"`
			Workspace struct {
				Members []string `toml:"members"`
				Exclude []string `toml:"exclude"`
			} `toml:"workspace"`
		} `toml:"uv"`
	} `toml:"tool"`
}

// PythonTask keeps command arguments explicit in pyproject.toml. uv supplies
// the package environment; One owns task names and dependency ordering.
type PythonTask struct {
	Description string   `toml:"description"`
	Run         []string `toml:"run"`
}

var pythonNameSeparator = regexp.MustCompile(`[-_.]+`)
var pythonRequirementName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*`)
var pythonProjectName = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$`)

func PythonPackageName(name string) string {
	return pythonNameSeparator.ReplaceAllString(strings.ToLower(name), "-")
}

func ParsePythonProject(raw []byte) (PythonProject, error) {
	var project PythonProject
	if err := toml.Unmarshal(raw, &project); err != nil {
		return project, err
	}
	if project.Project.Name != "" && !pythonProjectName.MatchString(project.Project.Name) {
		return project, i18n.Errorf("python.name_invalid", project.Project.Name)
	}
	sources := make(map[string]any, len(project.Tool.UV.Sources))
	for name, source := range project.Tool.UV.Sources {
		key := PythonPackageName(name)
		if _, exists := sources[key]; exists {
			return project, i18n.Errorf("python.source_duplicate", name)
		}
		sources[key] = source
	}
	project.Tool.UV.Sources = sources
	return project, nil
}

// PythonBuildDependencies finds local workspace and path sources, including
// optional and development requirements, without resolving external packages.
func PythonBuildDependencies(root string, projects []Project, read func(string) ([]byte, error)) (map[string][]string, error) {
	result := map[string][]string{}
	var pythonProjects []Project
	for _, project := range projects {
		if project.Toolchain == "python" {
			pythonProjects = append(pythonProjects, project)
		}
	}
	if len(pythonProjects) == 0 {
		return result, nil
	}
	if read == nil {
		read = func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(root, name)) }
	}
	rootRaw, err := read("pyproject.toml")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	rootProject, err := ParsePythonProject(rootRaw)
	if err != nil {
		return nil, fmt.Errorf("pyproject.toml: %w", err)
	}
	names, dirs := map[string]string{}, map[string]string{}
	metadata := map[string]PythonProject{}
	for _, project := range pythonProjects {
		file := filepath.ToSlash(filepath.Join(project.RelativeDir, "pyproject.toml"))
		raw, err := read(file)
		if err != nil {
			return nil, err
		}
		parsed, err := ParsePythonProject(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		name := PythonPackageName(parsed.Project.Name)
		if name == "" {
			return nil, i18n.Errorf("python.name_missing", file)
		}
		if previous, exists := names[name]; exists {
			return nil, i18n.Errorf("build.duplicate_package", name, previous, project.Name)
		}
		names[name], dirs[filepath.Clean(project.RelativeDir)] = project.Name, project.Name
		metadata[project.Name] = parsed
	}
	for _, project := range pythonProjects {
		parsed := metadata[project.Name]
		requirements := append([]string{}, parsed.Project.Dependencies...)
		for _, group := range parsed.Project.OptionalDependencies {
			requirements = append(requirements, group...)
		}
		for _, group := range parsed.DependencyGroups {
			for _, entry := range group {
				if requirement, ok := entry.(string); ok {
					requirements = append(requirements, requirement)
				}
			}
		}
		deps := map[string]bool{}
		for _, requirement := range requirements {
			name := PythonPackageName(pythonRequirementName.FindString(strings.TrimSpace(requirement)))
			source, exists := parsed.Tool.UV.Sources[name]
			base := project.RelativeDir
			if !exists {
				source = rootProject.Tool.UV.Sources[name]
				base = "."
			}
			sources, ok := source.([]any)
			if !ok {
				sources = []any{source}
			}
			for _, entry := range sources {
				fields, ok := entry.(map[string]any)
				if !ok {
					continue
				}
				target := ""
				if member, _ := fields["workspace"].(bool); member {
					target = names[name]
				}
				if path, _ := fields["path"].(string); path != "" {
					target = dirs[filepath.Clean(filepath.Join(base, path))]
				}
				if target != "" && target != project.Name {
					deps[target] = true
				}
			}
		}
		for dep := range deps {
			result[project.Name] = append(result[project.Name], dep)
		}
		sort.Strings(result[project.Name])
	}
	return result, nil
}
