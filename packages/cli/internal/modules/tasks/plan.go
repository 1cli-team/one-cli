// Package tasks owns workspace task planning and one mise invocation.
package tasks

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/miseconfig"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type Options struct {
	Name         string
	Projects     []string
	Arguments    []string
	Environment  string
	Cache        string
	Force        bool
	Jobs         int
	JobsExplicit bool
	UI           string
}
type Task struct {
	Name                  string   `json:"name"`
	Project               string   `json:"project,omitempty"`
	Operation             string   `json:"operation"`
	Directory             string   `json:"directory"`
	Source                string   `json:"source"`
	Description           string   `json:"description,omitempty"`
	Run                   any      `json:"run,omitempty"`
	Dependencies          []string `json:"depends"`
	Sources               []string `json:"sources,omitempty"`
	Outputs               []string `json:"outputs,omitempty"`
	Cached                bool     `json:"cache_enabled"`
	Interactive           bool     `json:"interactive"`
	Raw                   bool     `json:"raw"`
	Managed               bool     `json:"managed"`
	Status                string   `json:"status"`
	cacheInputs           []string
	File                  string `json:"file,omitempty"`
	environmentDirectives []string
	nativeName            string
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

// Project task names are namespaced in the single root configuration.
func taskIdentity(w execution.Workspace, canonical string) (string, string) {
	name := strings.TrimPrefix(canonical, "//:")
	project, operation, found := strings.Cut(name, ":")
	if found {
		for _, entry := range w.Projects() {
			if entry.Name == project {
				return project, operation
			}
		}
	}
	return "", name
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
	p := &Plan{Schema: "one-cli/task-plan/v1", Runtime: "mise", Environment: opts.Environment, DryRun: true, Tasks: []Task{}, Arguments: opts.Arguments, configuration: configuration}
	byName := map[string]Task{}
	for _, t := range catalog {
		byName[t.Name] = t
	}
	if len(opts.Projects) == 0 {
		name := opts.Name
		if !strings.HasPrefix(name, "//") {
			name = "//:" + name
		}
		p.Entries = []string{name}
	} else {
		names, err := w.SelectProjects(opts.Projects, "")
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			candidate := "//:" + name + ":" + opts.Name
			if _, ok := byName[candidate]; !ok {
				project, _ := w.Project(name)
				candidate = "//" + filepath.ToSlash(project.RelativeDir) + ":" + opts.Name
			}
			if task, ok := byName[candidate]; ok && task.Project != name {
				return nil, i18n.Errorf("tasks.project_mismatch", candidate, name)
			}
			p.Entries = append(p.Entries, candidate)
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
	if err := executionOptions(p, &opts); err != nil {
		return nil, err
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

// Project ownership follows the effective working directory, including nested projects.
func projectForDirectory(w execution.Workspace, directory string) string {
	canonical := func(path string) string {
		if real, err := filepath.EvalSymlinks(path); err == nil {
			return real
		}
		return filepath.Clean(path)
	}
	directory = canonical(directory)
	best, name := "", ""
	for _, project := range w.Projects() {
		root := canonical(project.TargetDir)
		relative, err := filepath.Rel(root, directory)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) && len(root) > len(best) {
			best, name = root, project.Name
		}
	}
	return name
}

func (t Task) runtimeName() string {
	if t.nativeName != "" {
		return t.nativeName
	}
	return t.Name
}
