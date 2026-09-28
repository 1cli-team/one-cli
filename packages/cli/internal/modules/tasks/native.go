package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/miseconfig"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

// Plan uses mise's effective catalog for real runs, including file tasks and
// native config precedence. Dry runs use NewPlan and never execute a child.
func (s Service) Plan(ctx context.Context, w execution.Workspace, opts Options) (*Plan, error) {
	if s.Provider == nil {
		return nil, i18n.Errorf("exec.mise_missing")
	}
	configuration, err := miseconfig.Build(w.Root(), miseconfig.Options{})
	if err != nil {
		return nil, err
	}
	if err = configuration.Apply(ctx); err != nil {
		return nil, err
	}
	command, err := s.Provider.PrepareCLI(ctx, runtimeport.Command{Directory: w.Root(), Argv: []string{"tasks", "ls", "--all", "--local", "--json"}, Env: os.Environ()})
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
		task.Directory = item.Dir
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
		_, operation, _ := strings.Cut(item.Name, ":")
		task.Operation = operation
		for _, project := range w.Projects() {
			if strings.HasPrefix(item.Name, "//"+project.RelativeDir+":") {
				task.Project = project.Name
				break
			}
		}
		task.Managed = false
		for _, command := range item.Run {
			if strings.HasPrefix(command, "one __task ") {
				task.Managed = true
			}
		}
		if rel, e := filepath.Rel(w.Root(), task.Source); e == nil {
			task.Source = filepath.ToSlash(rel)
		}
		catalog = append(catalog, task)
		for _, alias := range item.Aliases {
			if !strings.HasPrefix(alias, "//") {
				prefix, _, _ := strings.Cut(item.Name, ":")
				alias = prefix + ":" + alias
			}
			copy := task
			copy.Name = alias
			catalog = append(catalog, copy)
		}
	}
	// The configuration is already published. Build a fresh no-op plan so Apply
	// still detects changes between preparation and execution.
	current, err := miseconfig.Build(w.Root(), miseconfig.Options{})
	if err != nil {
		return nil, err
	}
	plan, err := planCatalog(w, opts, catalog, current)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (s Service) inspectCache(ctx context.Context, w execution.Workspace, plan *Plan, opts Options) error {
	for i := range plan.Tasks {
		task := &plan.Tasks[i]
		command, err := s.Provider.PrepareCLI(ctx, runtimeport.Command{Directory: w.Root(), Argv: []string{"tasks", "info", task.Name, "--json"}, Env: os.Environ()})
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
		if err := validateCacheContext(*task, opts); err != nil {
			return err
		}
	}
	return nil
}
