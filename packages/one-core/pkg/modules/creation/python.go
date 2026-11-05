package creation

import (
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/configedit"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func planPythonWorkspace(files *fsutil.FilePlan, dirs []string) error {
	raw, err := files.Read("pyproject.toml")
	if err != nil {
		return err
	}
	root, err := workspace.ParsePythonProject(raw)
	if err != nil {
		return err
	}
	patterns := append([]string{}, root.Tool.UV.Workspace.Members...)
	for _, exclude := range root.Tool.UV.Workspace.Exclude {
		patterns = append(patterns, "!"+exclude)
	}
	updated, err := includeProjects(patterns, dirs)
	if err != nil {
		return err
	}
	members := append([]string{}, root.Tool.UV.Workspace.Members...)
	members = append(members, updated[len(patterns):]...)
	names := map[string]string{}
	if root.Project.Name != "" {
		names[workspace.PythonPackageName(root.Project.Name)] = "pyproject.toml"
	}
	for _, dir := range dirs {
		file := filepath.ToSlash(filepath.Join(dir, "pyproject.toml"))
		data, err := files.Read(file)
		if err != nil {
			return err
		}
		project, err := workspace.ParsePythonProject(data)
		if err != nil {
			return err
		}
		name := workspace.PythonPackageName(project.Project.Name)
		if name == "" {
			return i18n.Errorf("python.name_missing", file)
		}
		if previous, exists := names[name]; exists {
			return i18n.Errorf("build.duplicate_package", name, previous, file)
		}
		names[name] = file
	}
	var document map[string]any
	if err := toml.Unmarshal(raw, &document); err != nil {
		return err
	}
	if document == nil {
		document = map[string]any{}
	}
	// Extend just the native uv membership. Preserve the root project, dependency
	// groups, indexes, sources and user comments through the TOML editor.
	table := document
	for _, key := range []string{"tool", "uv", "workspace"} {
		value, exists := table[key]
		if !exists {
			value = map[string]any{}
			table[key] = value
		}
		child, ok := value.(map[string]any)
		if !ok {
			return i18n.Errorf("python.workspace_table", key)
		}
		table = child
	}
	table["members"] = members
	desired, err := toml.Marshal(document)
	if err != nil {
		return err
	}
	after, err := configedit.UpdateTOML(raw, desired)
	if err != nil {
		return err
	}
	if err := files.Set("pyproject.toml", after, 0o644); err != nil {
		return err
	}
	ignore, err := files.Read(".gitignore")
	if err != nil {
		return err
	}
	text := strings.TrimRight(string(ignore), "\r\n")
	for _, pattern := range []string{".venv/", "__pycache__/", "*.py[cod]", "*.egg-info/"} {
		if !strings.Contains("\n"+text+"\n", "\n"+pattern+"\n") {
			text += "\n" + pattern
		}
	}
	return files.Set(".gitignore", []byte(text+"\n"), 0o644)
}
