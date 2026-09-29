package tasks

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// File metadata is TOML in #MISE comments. Reading it never runs the script.
func fileTasks(root, directory, project string, includes []string) (map[string]Task, error) {
	if includes == nil {
		includes = []string{"mise-tasks", ".mise-tasks", ".mise/tasks", ".config/mise/tasks", "mise/tasks"}
	}
	out := map[string]Task{}
	for _, include := range includes {
		if strings.Contains(include, "{{") || strings.Contains(include, "::") || strings.Contains(include, "://") {
			return nil, i18n.Errorf("tasks.dynamic_config", include)
		}
		base := filepath.Join(root, directory, include)
		if filepath.Ext(base) == ".toml" {
			raw, err := os.ReadFile(base)
			if err != nil {
				return nil, err
			}
			var entries map[string]any
			if err := toml.Unmarshal(raw, &entries); err != nil {
				return nil, err
			}
			for name, value := range entries {
				fields, ok := value.(map[string]any)
				if !ok {
					fields = map[string]any{"run": value}
				}
				metadata, err := toml.Marshal(fields)
				if err != nil {
					return nil, err
				}
				task, err := fileTaskMetadata(root, directory, project, name, base, metadata)
				if err != nil {
					return nil, err
				}
				task.Run = fields["run"]
				if runtime.GOOS == "windows" && hasRun(fields["run_windows"]) {
					task.Run = fields["run_windows"]
				}
				task.File = ""
				out[task.Name] = task
			}
			continue
		}
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
				return nil
			}
			if runtime.GOOS == "windows" && !strings.HasPrefix(string(raw), "#!") && !strings.Contains("|.exe|.bat|.cmd|.com|.ps1|.vbs|", "|"+strings.ToLower(filepath.Ext(path))+"|") {
				return nil
			}
			var metadata strings.Builder
			for _, line := range strings.Split(string(raw), "\n") {
				for _, prefix := range []string{"#MISE ", "//MISE ", "# [MISE] "} {
					if strings.HasPrefix(line, prefix) {
						metadata.WriteString(strings.TrimPrefix(line, prefix))
						metadata.WriteByte('\n')
					}
				}
			}
			relative, _ := filepath.Rel(base, path)
			name := strings.ReplaceAll(filepath.ToSlash(relative), "/", ":")
			name = strings.TrimSuffix(name, ":_default")
			task, err := fileTaskMetadata(root, directory, project, name, path, []byte(metadata.String()))
			if err != nil {
				return err
			}
			out[task.Name] = task
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func fileTaskMetadata(root, directory, project, name, path string, raw []byte) (Task, error) {
	metadata := map[string]any{}
	if err := toml.Unmarshal(raw, &metadata); err != nil {
		return Task{}, err
	}
	source, _ := filepath.Rel(root, path)
	task := Task{Name: "//" + directory + ":" + name, Project: project, Operation: name, Directory: filepath.Join(root, directory), Source: filepath.ToSlash(source), File: path, Status: "unknown"}
	validateTaskFields(&task, metadata)
	if dir, ok := metadata["dir"].(string); ok {
		task.Directory = filepath.Join(root, directory, dir)
		if filepath.IsAbs(dir) {
			task.Directory = dir
		}
	}
	task.Description, _ = metadata["description"].(string)
	task.Interactive, _ = metadata["interactive"].(bool)
	task.Raw, _ = metadata["raw"].(bool)
	task.Dependencies, _ = stringList(metadata["depends"])
	task.waitFor, _ = stringList(metadata["wait_for"])
	task.Sources, _ = stringList(metadata["sources"])
	task.Outputs, _ = stringList(metadata["outputs"])
	return task, nil
}
