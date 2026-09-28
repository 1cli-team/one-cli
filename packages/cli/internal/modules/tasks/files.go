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
	var fields struct {
		Description string   `toml:"description"`
		Dir         string   `toml:"dir"`
		Depends     []string `toml:"depends"`
		Post        []string `toml:"depends_post"`
		Sources     []string `toml:"sources"`
		Outputs     []string `toml:"outputs"`
		Interactive bool     `toml:"interactive"`
		Raw         bool     `toml:"raw"`
		Cache       struct {
			Enabled bool `toml:"enabled"`
		} `toml:"cache"`
	}
	if err := toml.Unmarshal(raw, &fields); err != nil {
		return Task{}, err
	}
	if strings.Contains(fields.Dir, "{{") || len(fields.Post) > 0 {
		return Task{}, i18n.Errorf("tasks.dynamic_config", path)
	}
	dir := filepath.Join(root, directory, fields.Dir)
	if filepath.IsAbs(fields.Dir) {
		dir = fields.Dir
	}
	source, _ := filepath.Rel(root, path)
	return Task{Name: "//" + directory + ":" + name, Project: project, Operation: name, Directory: dir, Source: filepath.ToSlash(source), Description: fields.Description, Run: []string{path}, Dependencies: fields.Depends, Sources: fields.Sources, Outputs: fields.Outputs, Cached: fields.Cache.Enabled, Interactive: fields.Interactive, Raw: fields.Raw, Status: "unknown"}, nil
}
