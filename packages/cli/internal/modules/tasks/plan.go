// Package tasks owns finite workspace task planning and one mise invocation.
package tasks

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/miseconfig"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type Options struct {
	Name        string
	Projects    []string
	Arguments   []string
	Environment string
	Cache       string
	Force       bool
	Jobs        int
	UI          string
}
type Task struct {
	Name         string   `json:"name"`
	Project      string   `json:"project,omitempty"`
	Operation    string   `json:"operation"`
	Directory    string   `json:"directory"`
	Source       string   `json:"source"`
	Description  string   `json:"description,omitempty"`
	Run          any      `json:"run,omitempty"`
	Dependencies []string `json:"depends"`
	Sources      []string `json:"sources,omitempty"`
	Outputs      []string `json:"outputs,omitempty"`
	Cached       bool     `json:"cache_enabled"`
	Interactive  bool     `json:"interactive"`
	Raw          bool     `json:"raw"`
	Managed      bool     `json:"managed"`
	Status       string   `json:"status"`
	cacheInputs  []string
}
type Plan struct {
	Schema        string              `json:"schema"`
	Runtime       string              `json:"runtime"`
	Environment   string              `json:"environment,omitempty"`
	DryRun        bool                `json:"dry_run"`
	Entries       []string            `json:"entries"`
	Arguments     []string            `json:"arguments,omitempty"`
	Tasks         []Task              `json:"tasks"`
	ConfigChanges []miseconfig.Change `json:"config_changes,omitempty"`
	configuration *miseconfig.Plan
}

func (p *Plan) RenderTTY(w io.Writer) {
	for _, t := range p.Tasks {
		fmt.Fprintf(w, "%s  %s\n", t.Name, t.Source)
	}
	if p.DryRun {
		fmt.Fprintln(w, i18n.T("tasks.preview"))
	}
}

func RenderCatalog(w io.Writer, tasks []Task) {
	for _, task := range tasks {
		fmt.Fprintln(w, task.Name)
		if task.Description != "" {
			fmt.Fprintln(w, "  "+task.Description)
		}
		boolean := func(value bool) string {
			if value {
				return i18n.T("common.yes")
			}
			return i18n.T("common.no")
		}
		fmt.Fprintln(w, i18n.Tf("tasks.list_details", task.Source, boolean(task.Cached), boolean(task.Interactive || task.Raw)))
	}
}

func Catalog(w execution.Workspace) ([]Task, *miseconfig.Plan, error) {
	config, err := miseconfig.Build(w.Root(), miseconfig.Options{})
	if err != nil {
		return nil, nil, err
	}
	overlay := map[string][]byte{}
	for _, change := range config.Changes {
		overlay[change.Path] = []byte(change.After)
	}
	all := map[string]Task{}
	dirs := map[string]string{"": ""}
	for _, p := range w.Projects() {
		dirs[p.RelativeDir] = p.Name
	}
	for dir, project := range dirs {
		var includes []string
		files, err := filepath.Glob(filepath.Join(w.Root(), dir, ".mise/conf.d/*.toml"))
		if err != nil {
			return nil, nil, err
		}
		managed := filepath.Join(w.Root(), dir, miseconfig.Filename)
		found := false
		for _, f := range files {
			if f == managed {
				found = true
			}
		}
		if !found {
			files = append(files, managed)
		}
		sort.Strings(files)
		for _, name := range []string{".mise.toml", "mise.toml", ".mise.local.toml", "mise.local.toml"} {
			files = append(files, filepath.Join(w.Root(), dir, name))
		}
		for _, file := range files {
			rel, _ := filepath.Rel(w.Root(), file)
			rel = filepath.ToSlash(rel)
			raw, ok := overlay[rel]
			if !ok {
				raw, err = os.ReadFile(file)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return nil, nil, err
				}
			}
			var doc struct {
				Tasks      map[string]any `toml:"tasks"`
				TaskConfig struct {
					Includes []string `toml:"includes"`
				} `toml:"task_config"`
			}
			if err = toml.Unmarshal(raw, &doc); err != nil {
				return nil, nil, err
			}
			if doc.TaskConfig.Includes != nil {
				includes = doc.TaskConfig.Includes
			}
			for name, value := range doc.Tasks {
				fields, ok := value.(map[string]any)
				if !ok {
					fields = map[string]any{"run": value}
				}
				canonical := "//" + dir + ":" + name
				task, exists := all[canonical]
				if !exists {
					task = Task{Name: canonical, Project: project, Operation: name, Directory: filepath.Join(w.Root(), dir), Status: "unknown", Dependencies: []string{}}
				}
				task.Source = rel
				if filepath.Clean(file) == filepath.Clean(managed) {
					task.Managed = true
				} else if _, replaced := fields["run"]; replaced {
					task.Managed = false
				}
				if v, ok := fields["description"].(string); ok {
					task.Description = v
				}
				if v, ok := fields["run"]; ok {
					task.Run = v
				}
				if v, ok := fields["depends"]; ok {
					task.Dependencies, err = stringList(v)
					if err != nil {
						return nil, nil, i18n.Errorf("tasks.dynamic_config", canonical)
					}
				}
				if v, ok := fields["depends_post"]; ok {
					post, e := stringList(v)
					if e != nil || len(post) > 0 {
						return nil, nil, i18n.Errorf("tasks.dynamic_config", canonical)
					}
				}
				if v, ok := fields["sources"]; ok {
					task.Sources, _ = stringList(v)
				}
				if v, ok := fields["outputs"]; ok {
					task.Outputs, _ = stringList(v)
				}
				if v, ok := fields["cache"].(map[string]any); ok {
					if value, ok := v["enabled"].(bool); ok {
						task.Cached = value
					}
					task.cacheInputs, _ = stringList(v["command_inputs"])
				}
				if v, ok := fields["interactive"].(bool); ok {
					task.Interactive = v
				}
				if v, ok := fields["raw"].(bool); ok {
					task.Raw = v
				}
				if v, ok := fields["dir"].(string); ok {
					if strings.Contains(v, "{{") {
						return nil, nil, i18n.Errorf("tasks.dynamic_config", canonical)
					}
					if filepath.IsAbs(v) {
						task.Directory = v
					} else {
						task.Directory = filepath.Join(w.Root(), dir, v)
					}
				}
				all[canonical] = task
			}
		}
		scripts, err := fileTasks(w.Root(), dir, project, includes)
		if err != nil {
			return nil, nil, err
		}
		for name, task := range scripts {
			if _, exists := all[name]; !exists {
				all[name] = task
			}
		}
	}
	out := make([]Task, 0, len(all))
	for _, task := range all {
		out = append(out, task)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, config, nil
}
func stringList(value any) ([]string, error) {
	if s, ok := value.(string); ok {
		return []string{s}, nil
	}
	out := []string{}
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("unsupported task dependencies")
	}
	for _, v := range values {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("unsupported task dependency")
		}
		out = append(out, s)
	}
	return out, nil
}
func NewPlan(w execution.Workspace, opts Options) (*Plan, error) {
	catalog, configuration, err := Catalog(w)
	if err != nil {
		return nil, err
	}
	return planCatalog(w, opts, catalog, configuration)
}

func planCatalog(w execution.Workspace, opts Options, catalog []Task, configuration *miseconfig.Plan) (*Plan, error) {
	environment, _, err := secrets.ResolveEnvName(w.Root(), opts.Environment, false)
	if err != nil {
		return nil, err
	}
	opts.Environment = environment
	p := &Plan{Schema: "one-cli/task-plan/v1", Runtime: "mise", Environment: opts.Environment, DryRun: true, Tasks: []Task{}, Arguments: opts.Arguments, configuration: configuration, ConfigChanges: configuration.Changes}
	byName := map[string]Task{}
	for _, t := range catalog {
		byName[t.Name] = t
	}
	if len(opts.Projects) == 0 {
		p.Entries = []string{"//:" + opts.Name}
	} else {
		names, err := w.SelectProjects(opts.Projects, "")
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			project, _ := w.Project(name)
			p.Entries = append(p.Entries, "//"+project.RelativeDir+":"+opts.Name)
		}
	}
	if len(p.Entries) > 1 && len(opts.Arguments) > 0 {
		return nil, i18n.Errorf("tasks.multi_arguments")
	}
	seen := map[string]int{}
	stack := []string{}
	var visit func(string) error
	visit = func(name string) error {
		if seen[name] == 2 {
			return nil
		}
		if seen[name] == 1 {
			return i18n.Errorf("build.dependency_cycle", strings.Join(append(stack, name), " -> "))
		}
		task, ok := byName[name]
		if !ok {
			return i18n.Errorf("tasks.missing", name)
		}
		seen[name] = 1
		if task.Managed && task.Project != "" {
			project, ok := w.Project(task.Project)
			if !ok {
				return i18n.Errorf("tasks.missing", task.Name)
			}
			// The hidden leaf executes in its registered project directory. A
			// different mise directory would hash and restore the wrong files.
			if filepath.Clean(task.Directory) != filepath.Clean(project.TargetDir) {
				return i18n.Errorf("tasks.directory_override", task.Name, project.TargetDir)
			}
		}
		if err := validateCacheContext(task, opts); err != nil {
			return err
		}
		stack = append(stack, name)
		for _, dep := range task.Dependencies {
			if strings.Contains(dep, "{{") || strings.ContainsAny(dep, " \t") {
				return i18n.Errorf("tasks.dynamic_config", name)
			}
			if !strings.HasPrefix(dep, "//") {
				i := strings.Index(name, ":")
				dep = name[:i+1] + dep
			}
			if strings.ContainsAny(dep, "*?[") {
				matched := false
				for _, candidate := range catalog {
					ok, _ := filepath.Match(dep, candidate.Name)
					if ok {
						matched = true
						if err := visit(candidate.Name); err != nil {
							return err
						}
					}
				}
				if !matched {
					return i18n.Errorf("tasks.missing", dep)
				}
			} else if err := visit(dep); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		seen[name] = 2
		p.Tasks = append(p.Tasks, task)
		return nil
	}
	for _, entry := range p.Entries {
		if err := visit(entry); err != nil {
			return nil, err
		}
	}
	for i, a := range p.Tasks {
		for _, out := range a.Outputs {
			if filepath.IsAbs(out) || out == ".." || strings.HasPrefix(filepath.Clean(out), ".."+string(filepath.Separator)) {
				return nil, i18n.Errorf("tasks.output_escape", a.Name, out)
			}
			pathA := outputBoundary(a.Directory, out)
			if err := fsutil.SafeWritePath(a.Directory, pathA); err != nil {
				return nil, err
			}
			for _, b := range p.Tasks[i+1:] {
				for _, other := range b.Outputs {
					pathB := outputBoundary(b.Directory, other)
					if pathA == pathB || strings.HasPrefix(pathA, pathB+string(filepath.Separator)) || strings.HasPrefix(pathB, pathA+string(filepath.Separator)) {
						return nil, i18n.Errorf("tasks.output_conflict", a.Name, b.Name)
					}
				}
			}
		}
	}
	return p, nil
}

// A glob may match files that do not exist yet. Reserve its fixed directory
// prefix so parallel tasks cannot publish overlapping artifact trees.
func outputBoundary(directory, pattern string) string {
	parts := strings.Split(filepath.ToSlash(pattern), "/")
	for i, part := range parts {
		if strings.ContainsAny(part, "*?[") {
			return filepath.Join(directory, filepath.FromSlash(strings.Join(parts[:i], "/")))
		}
	}
	return filepath.Join(directory, pattern)
}

// MarshalCatalog keeps an explicit schema and never exposes resolved environment values.
func MarshalCatalog(tasks []Task) any {
	return struct {
		Schema string `json:"schema"`
		Tasks  []Task `json:"tasks"`
	}{"one-cli/tasks/v1", tasks}
}

func validContextInput(command, project, operation string) bool {
	fields := strings.Fields(command)
	expected := []string{"one", "__task-input", "--project", project, "--task", operation}
	if len(fields) != len(expected) {
		return false
	}
	for i, field := range fields {
		if strings.Trim(field, "\"'") != expected[i] {
			return false
		}
	}
	return true
}

func validateCacheContext(task Task, opts Options) error {
	if !task.Managed || task.Project == "" || !task.Cached || task.Interactive || task.Raw || opts.Cache == "off" || opts.UI == "raw" || len(opts.Arguments) > 0 {
		return nil
	}
	for _, input := range task.cacheInputs {
		if validContextInput(input, task.Project, task.Operation) {
			return nil
		}
	}
	return i18n.Errorf("tasks.cache_context_required", task.Name)
}
