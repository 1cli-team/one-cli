// Package build plans and executes finite workspace build tasks.
package build

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type Task struct {
	Project      string   `json:"project"`
	Directory    string   `json:"directory"`
	Argv         []string `json:"argv,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Status       string   `json:"status"`
	Reason       string   `json:"reason,omitempty"`
	ExitCode     int      `json:"exit_code"`
	DurationMS   int64    `json:"duration_ms"`
}

type Plan struct {
	Schema      string `json:"schema"`
	Runtime     string `json:"runtime"`
	Environment string `json:"environment,omitempty"`
	DryRun      bool   `json:"dry_run"`
	Tasks       []Task `json:"tasks"`
}

type nodePackage struct {
	Name                 string            `json:"name"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

// NewPlan reads project configuration only. It never prepares a runtime,
// installs dependencies, loads secrets, or runs a child command.
func NewPlan(w execution.Workspace, selector, environment string) (*Plan, error) {
	var selectors []string
	if selector != "" {
		selectors = []string{selector}
	}
	return NewPlanForProjects(w, selectors, environment)
}

func NewPlanForProjects(w execution.Workspace, selectors []string, environment string) (*Plan, error) {
	kind, err := execution.RuntimeKind(w.Root())
	if err != nil {
		return nil, err
	}
	environment, _, err = secrets.ResolveEnvName(w.Root(), environment, false)
	if err != nil {
		return nil, err
	}
	projects := w.Projects()
	if len(selectors) > 0 {
		names, err := w.SelectProjects(selectors, "")
		if err != nil {
			return nil, err
		}
		projects = projects[:0:0]
		for _, name := range names {
			p, _ := w.Project(name)
			projects = append(projects, *p)
		}
	}
	plan := &Plan{Schema: "one-cli/build-plan/v1", Runtime: kind, Environment: environment, DryRun: true, Tasks: []Task{}}
	packages := map[string]nodePackage{}
	names := map[string]string{}
	directories := map[string]string{}
	tasks := map[string]Task{}
	order := []string{}
	ready := 0
	for _, p := range projects {
		task := Task{Project: p.Name, Directory: p.TargetDir, Status: "pending"}
		task.Argv, err = execution.OperationArgs(w, p.Name, "build")
		if err != nil {
			var missing *output.Error
			if len(selectors) > 0 || !errors.As(err, &missing) || missing.Code != string(cliErrors.RUNTIME_TASK_NOT_FOUND) {
				return nil, fmt.Errorf("%s: %w", p.Name, err)
			}
			task.Status, task.Reason = "skipped", "no-build-task"
		} else {
			ready++
		}
		if p.Toolchain == "node" && len(projects) > 1 {
			raw, err := os.ReadFile(filepath.Join(p.TargetDir, "package.json"))
			if err != nil {
				return nil, err
			}
			var pkg nodePackage
			if err := json.Unmarshal(raw, &pkg); err != nil {
				return nil, err
			}
			if pkg.Name != "" {
				if previous, ok := names[pkg.Name]; ok {
					return nil, fmt.Errorf("duplicate Node package name %q in %s and %s", pkg.Name, previous, p.Name)
				}
				names[pkg.Name] = p.Name
			}
			packages[p.Name] = pkg
			directories[filepath.Clean(p.TargetDir)] = p.Name
		}
		tasks[p.Name] = task
		order = append(order, p.Name)
	}
	if ready == 0 {
		return nil, cliErrors.New(cliErrors.RUNTIME_TASK_NOT_FOUND, "No projects have a build task.")
	}
	// Package names, not manifest aliases, identify local Node dependencies.
	// Keep manifest order among otherwise independent projects.
	for name, pkg := range packages {
		local := map[string]bool{}
		for _, deps := range []map[string]string{pkg.Dependencies, pkg.DevDependencies, pkg.OptionalDependencies} {
			for dep, spec := range deps {
				target := names[dep]
				for _, prefix := range []string{"file:", "link:"} {
					if strings.HasPrefix(spec, prefix) {
						target = directories[filepath.Clean(filepath.Join(tasks[name].Directory, strings.TrimPrefix(spec, prefix)))]
					}
				}
				if target != "" {
					local[target] = true
				}
			}
		}
		task := tasks[name]
		for _, dep := range order {
			if local[dep] {
				task.Dependencies = append(task.Dependencies, dep)
			}
		}
		tasks[name] = task
	}
	state := map[string]int{}
	stack := []string{}
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 2 {
			return nil
		}
		if state[name] == 1 {
			start := 0
			for i, item := range stack {
				if item == name {
					start = i
					break
				}
			}
			return fmt.Errorf("local build dependency cycle: %s", strings.Join(append(append([]string{}, stack[start:]...), name), " -> "))
		}
		state[name] = 1
		stack = append(stack, name)
		for _, dep := range tasks[name].Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = 2
		plan.Tasks = append(plan.Tasks, tasks[name])
		return nil
	}
	for _, name := range order {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return plan, nil
}
