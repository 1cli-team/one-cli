package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

// Plan uses mise's effective catalog for real runs, including file tasks and
// native config precedence. Dry runs use NewPlan and never execute a child.
func (s Service) Plan(ctx context.Context, w execution.Workspace, opts Options) (*Plan, error) {
	// Reject selectors before publishing configuration or bootstrapping mise.
	if _, err := w.SelectProjects(opts.Projects, ""); err != nil {
		return nil, err
	}
	if s.Provider == nil {
		return nil, i18n.Errorf("exec.mise_missing")
	}
	catalog, err := s.catalog(ctx, w, os.Environ())
	if err != nil {
		return nil, err
	}
	return planCatalog(w, opts, catalog, nil)
}

// Catalog queries mise's effective tasks without generating configuration.
func (s Service) Catalog(ctx context.Context, w execution.Workspace) ([]Task, error) {
	catalog, err := s.catalog(ctx, w, os.Environ())
	if err != nil {
		return nil, err
	}
	plan := &Plan{Tasks: catalog}
	if err = s.inspectCache(ctx, w, plan); err != nil {
		return nil, err
	}
	return plan.Tasks, nil
}
func (s Service) catalog(ctx context.Context, w execution.Workspace, env []string) ([]Task, error) {
	command, err := s.Provider.PrepareCLI(ctx, runtimeport.Command{Directory: w.Root(), Argv: []string{"tasks", "ls", "--all", "--local", "--json"}, Env: env})
	if err != nil {
		return nil, err
	}
	cmd := process.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	cmd.Dir = command.Directory
	cmd.Env = command.Env
	process.CancelProcessTree(cmd)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err = cmd.Run(); err != nil {
		return nil, i18n.Errorf("tasks.discovery_failed", err, strings.TrimSpace(errOut.String()))
	}
	var native []struct {
		Name        string   `json:"name"`
		Env         []string `json:"env"`
		File        string   `json:"file"`
		Description string   `json:"description"`
		Source      string   `json:"source"`
		Dir         string   `json:"dir"`
		Depends     []string `json:"depends"`
		Post        []string `json:"depends_post"`
		Run         []string `json:"run"`
		Sources     []string `json:"sources"`
		Outputs     []string `json:"outputs"`
		Raw         bool     `json:"raw"`
		Interactive bool     `json:"interactive"`
		Aliases     []string `json:"aliases"`
	}
	if err = json.Unmarshal(out.Bytes(), &native); err != nil {
		return nil, err
	}
	catalog := []Task{}
	for _, item := range native {
		task := Task{}
		task.Name = item.Name
		task.nativeName = item.Name
		if !strings.HasPrefix(task.Name, "//") {
			task.Name = "//:" + task.Name
		}
		task.environmentDirectives = item.Env
		task.File = item.File
		task.Directory = item.Dir
		if task.Directory == "" {
			task.Directory = w.Root()
		}
		task.Source = item.Source
		task.Description = item.Description
		for _, dependency := range append(item.Depends, item.Post...) {
			// mise has already resolved task expressions. Dependency parameters
			// belong to mise; only the referenced task is needed for this scope.
			if fields := strings.Fields(dependency); len(fields) > 0 {
				task.Dependencies = append(task.Dependencies, fields[0])
			}
		}
		task.Run = item.Run
		task.Raw = item.Raw
		task.Interactive = item.Interactive
		task.Sources = item.Sources
		task.Outputs = item.Outputs
		task.Status = "unknown"
		task.Project, task.Operation = taskIdentity(w, task.Name)
		task.Project = projectForDirectory(w, task.Directory)
		if scope, local, found := strings.Cut(task.Name, ":"); found && scope != "//" {
			task.Operation = local
		}
		if rel, e := filepath.Rel(w.Root(), task.Source); e == nil {
			task.Source = filepath.ToSlash(rel)
		}
		catalog = append(catalog, task)
		for _, alias := range item.Aliases {
			if !strings.HasPrefix(alias, "//") {
				prefix, _, _ := strings.Cut(task.Name, ":")
				alias = prefix + ":" + alias
			}
			copy := task
			copy.Name = alias
			copy.Project, copy.Operation = taskIdentity(w, alias)
			copy.Project = task.Project
			if scope, local, found := strings.Cut(alias, ":"); found && scope != "//" {
				copy.Operation = local
			}
			catalog = append(catalog, copy)
		}
	}
	return catalog, nil
}

func (s Service) inspectCache(ctx context.Context, w execution.Workspace, plan *Plan) error {
	for i := range plan.Tasks {
		task := &plan.Tasks[i]
		command, err := s.Provider.PrepareCLI(ctx, runtimeport.Command{Directory: w.Root(), Argv: []string{"tasks", "info", task.runtimeName(), "--json"}, Env: os.Environ()})
		if err != nil {
			return err
		}
		cmd := process.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
		cmd.Dir, cmd.Env = command.Directory, command.Env
		process.CancelProcessTree(cmd)
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			return i18n.Errorf("tasks.discovery_failed", err, strings.TrimSpace(errOut.String()))
		}
		var details struct {
			Cache struct {
				Enabled       bool     `json:"enabled"`
				CommandInputs []string `json:"command_inputs"`
			} `json:"cache"`
		}
		if err := json.Unmarshal(out.Bytes(), &details); err != nil {
			return err
		}
		task.Cached, task.cacheInputs = details.Cache.Enabled, details.Cache.CommandInputs
	}
	return nil
}

// Re-read before launching: precedence changes must never silently drop a binding
// or change a command, dependency, or directory after its project was selected.
func (s Service) verifyBindings(ctx context.Context, w execution.Workspace, p *Plan, session *environmentSession) error {
	if len(session.bindings) == 0 {
		return nil
	}
	// Config filename overrides must preserve every original layer, including
	// environment/tool-only layers selected by a .miserc.toml configuration.
	scopes := map[string]bool{w.Root(): true}
	for _, task := range p.Tasks {
		if name := task.runtimeName(); strings.HasPrefix(name, "//") {
			scope, _, _ := strings.Cut(name, ":")
			scopes[filepath.Join(w.Root(), filepath.FromSlash(strings.TrimPrefix(scope, "//")))] = true
		}
	}
	for scope := range scopes {
		before, err := s.configPaths(ctx, scope, session.base, nil)
		if err != nil {
			return err
		}
		after, err := s.configPaths(ctx, scope, session.env, session.dirs)
		if err != nil {
			return err
		}
		if !slices.Equal(before, after) {
			return i18n.Errorf("tasks.binding_changed", scope)
		}
	}
	catalog, err := s.catalog(ctx, w, session.env)
	if err != nil {
		return err
	}
	actual := map[string]Task{}
	for _, t := range catalog {
		actual[t.Name] = t
	}
	for _, before := range p.Tasks {
		after, ok := actual[before.Name]
		if !ok || before.Directory != after.Directory || before.File != after.File || !reflect.DeepEqual(before.Run, after.Run) || !reflect.DeepEqual(before.Dependencies, after.Dependencies) || before.Raw != after.Raw || before.Interactive != after.Interactive {
			return i18n.Errorf("tasks.binding_changed", before.Name)
		}
		_, bound := session.bindings[before.Name]
		if bound != slices.Contains(after.environmentDirectives, "module "+session.plugin) {
			return i18n.Errorf("tasks.binding_overridden", before.Name)
		}
	}
	return nil
}

func (s Service) configPaths(ctx context.Context, directory string, env, ignored []string) ([]string, error) {
	command, err := s.Provider.PrepareCLI(ctx, runtimeport.Command{Directory: directory, Argv: []string{"config", "ls", "--json"}, Env: env})
	if err != nil {
		return nil, err
	}
	cmd := process.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	cmd.Dir, cmd.Env = command.Directory, command.Env
	process.CancelProcessTree(cmd)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err = cmd.Run(); err != nil {
		return nil, i18n.Errorf("tasks.discovery_failed", err, strings.TrimSpace(stderr.String()))
	}
	var entries []struct {
		Path string `json:"path"`
	}
	if err = json.Unmarshal(out.Bytes(), &entries); err != nil {
		return nil, err
	}
	// mise resolves symlinks in config paths, while session directories retain
	// the workspace spelling (for example, /var vs /private/var on macOS).
	// Compare physical paths on both sides without changing config precedence.
	ignoredDirs := make(map[string]bool, len(ignored))
	for _, dir := range ignored {
		path, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return nil, err
		}
		ignoredDirs[path] = true
	}
	paths := []string{}
	for _, entry := range entries {
		path, err := filepath.EvalSymlinks(entry.Path)
		if err != nil {
			return nil, err
		}
		if !ignoredDirs[filepath.Dir(path)] {
			paths = append(paths, path)
		}
	}
	return paths, nil
}
