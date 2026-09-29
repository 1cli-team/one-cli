package tasks

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/miseconfig"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// Catalog is a deliberately static preview. It never evaluates templates,
// command inputs, env plugins, or task scripts. Real runs use mise's catalog.
func Catalog(w execution.Workspace) ([]Task, *miseconfig.Plan, error) {
	all := map[string]Task{}
	roots, err := staticScope(w, "", all)
	if err != nil {
		return nil, nil, err
	}
	for _, pattern := range roots {
		if strings.Contains(pattern, "{{") || strings.Contains(pattern, "**") || strings.HasPrefix(pattern, "!") || filepath.IsAbs(pattern) || strings.Contains(pattern, "..") {
			return nil, nil, i18n.Errorf("tasks.dynamic_config", pattern)
		}
		dirs, err := filepath.Glob(filepath.Join(w.Root(), pattern))
		if err != nil {
			return nil, nil, err
		}
		for _, dir := range dirs {
			info, err := os.Stat(dir)
			if err != nil {
				return nil, nil, err
			}
			if !info.IsDir() {
				continue
			}
			relative, _ := filepath.Rel(w.Root(), dir)
			if relative == "." {
				continue
			}
			if _, err = staticScope(w, filepath.ToSlash(relative), all); err != nil {
				return nil, nil, err
			}
		}
	}
	out := make([]Task, 0, len(all))
	for _, task := range all {
		out = append(out, task)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil, nil
}

func staticPatterns() []string {
	patterns := configPatterns(os.Environ())
	profile := os.Getenv("MISE_ENV")
	if profile == "" {
		profile = os.Getenv("MISE_ENVIRONMENT")
	}
	for _, env := range strings.Split(profile, ",") {
		if env == "" {
			continue
		}
		for _, suffix := range []string{".toml", ".local.toml"} {
			for _, stem := range []string{".config/mise/config.", ".config/mise.", "mise/config.", "mise.", ".mise/config.", ".mise."} {
				patterns = append(patterns, stem+env+suffix)
			}
		}
	}
	return patterns
}

type staticDocument struct {
	MonorepoRoot bool           `toml:"monorepo_root"`
	Tasks        map[string]any `toml:"tasks"`
	TaskConfig   struct {
		Includes []string `toml:"includes"`
	} `toml:"task_config"`
	Monorepo struct {
		Roots []string `toml:"config_roots"`
	} `toml:"monorepo"`
}

func staticScope(w execution.Workspace, scope string, all map[string]Task) ([]string, error) {
	type layer struct {
		path string
		doc  staticDocument
	}
	var layers []layer
	var includes, roots []string
	monorepoRoot := false
	seen := map[string]bool{}
	for _, pattern := range staticPatterns() {
		files, err := filepath.Glob(filepath.Join(w.Root(), scope, pattern))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if seen[file] {
				continue
			}
			seen[file] = true
			raw, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			var doc staticDocument
			if err = toml.Unmarshal(raw, &doc); err != nil {
				return nil, err
			}
			if doc.TaskConfig.Includes != nil {
				includes = doc.TaskConfig.Includes
			}
			monorepoRoot = monorepoRoot || doc.MonorepoRoot
			if doc.Monorepo.Roots != nil {
				roots = doc.Monorepo.Roots
			}
			layers = append(layers, layer{file, doc})
		}
	}
	scripts, err := fileTasks(w.Root(), scope, "", includes)
	if err != nil {
		return nil, err
	}
	aliases := map[string][]string{}
	for name, task := range scripts {
		task.Project = projectForDirectory(w, task.Directory)
		all[name] = task
	}
	for _, layer := range layers {
		for name, value := range layer.doc.Tasks {
			canonical := "//" + scope + ":" + name
			fields, ok := value.(map[string]any)
			if !ok {
				fields = map[string]any{"run": value}
			}
			task := all[canonical]
			// A command-bearing definition replaces the earlier task. Metadata-only
			// definitions merge into an existing executable (including file tasks).
			_, hasFile := fields["file"]
			command := fields["run"]
			if runtime.GOOS == "windows" && hasRun(fields["run_windows"]) {
				command = fields["run_windows"]
			}
			if task.Name == "" || hasRun(command) || hasFile || !hasRun(task.Run) && task.File == "" {
				_, op := taskIdentity(w, canonical)
				if scope != "" {
					op = name
				}
				source, _ := filepath.Rel(w.Root(), layer.path)
				task = Task{Name: canonical, Operation: op, Directory: filepath.Join(w.Root(), scope), Source: filepath.ToSlash(source), Status: "unknown", Dependencies: []string{}}
				aliases[canonical] = nil
			}
			if command != nil {
				task.Run = command
			}
			if file, ok := fields["file"].(string); ok {
				task.File = filepath.Join(w.Root(), scope, file)
			}
			if v, ok := fields["description"].(string); ok && v != "" {
				task.Description = v
			}
			if v, ok := fields["dir"].(string); ok {
				if strings.Contains(v, "{{") {
					return nil, i18n.Errorf("tasks.dynamic_config", canonical)
				}
				task.Directory = filepath.Join(w.Root(), scope, v)
				if filepath.IsAbs(v) {
					task.Directory = v
				}
			}
			for key, target := range map[string]*[]string{"depends": &task.Dependencies, "sources": &task.Sources} {
				if value, ok := fields[key]; ok {
					values, e := stringList(value)
					if e != nil {
						return nil, i18n.Errorf("tasks.dynamic_config", canonical)
					}
					*target = append(*target, values...)
				}
			}
			if value, ok := fields["depends_post"]; ok {
				values, e := stringList(value)
				if e != nil || len(values) > 0 {
					return nil, i18n.Errorf("tasks.dynamic_config", canonical)
				}
			}
			if value, ok := fields["outputs"]; ok {
				task.Outputs, _ = stringList(value)
			}
			if value, ok := fields["alias"]; ok {
				values, _ := stringList(value)
				aliases[canonical] = append(aliases[canonical], values...)
			}
			if value, ok := fields["aliases"]; ok {
				values, _ := stringList(value)
				aliases[canonical] = append(aliases[canonical], values...)
			}
			if v, ok := fields["interactive"].(bool); ok {
				task.Interactive = v
			}
			if v, ok := fields["raw"].(bool); ok {
				task.Raw = v
			}
			if v, ok := fields["cache"].(map[string]any); ok {
				task.Cached, _ = v["enabled"].(bool)
				task.cacheInputs, _ = stringList(v["command_inputs"])
			}
			task.Project = projectForDirectory(w, task.Directory)
			all[canonical] = task
		}
	}
	for canonical, names := range aliases {
		for _, name := range names {
			alias := all[canonical]
			alias.nativeName = canonical
			alias.Name = "//" + scope + ":" + name
			all[alias.Name] = alias
		}
	}
	if scope == "" && monorepoRoot && roots == nil {
		for _, project := range w.Projects() {
			roots = append(roots, project.RelativeDir)
		}
	}
	return roots, nil
}
