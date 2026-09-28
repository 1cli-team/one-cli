package tasks

import (
	"context"
	"io"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// PrepareDevelopment runs the finite prerequisites of selected dev tasks once.
// The existing development supervisor continues to own interactive processes.
func (s Service) PrepareDevelopment(ctx context.Context, w execution.Workspace, projects []string, environment string, in io.Reader, out, errOut io.Writer) error {
	opts := Options{Name: "dev", Projects: projects, Environment: environment, Jobs: 1, Cache: "local-only", UI: "stream"}
	p, err := NewPlan(w, opts)
	if err != nil {
		return err
	}
	// Prepare the complete scope once, including source projects and finite
	// prerequisites. Development may create or update the pnpm lockfile.
	prepare := s.Prepare
	if prepare == nil {
		prepare = (dependencies.Service{Provider: s.Provider}).Prepare
	}
	runtimeKind, err := execution.RuntimeKind(w.Root())
	if err != nil {
		return err
	}
	input := dependencies.Input{Root: w.Root(), Manifest: w.Manifest(), Projects: projectNames(p), Runtime: runtimeKind, Development: true, Log: errOut}
	selected := map[string]bool{}
	for _, entry := range p.Entries {
		selected[entry] = true
	}
	entries := []string{}
	for _, task := range p.Tasks {
		if selected[task.Name] {
			prefix, _, _ := strings.Cut(task.Name, ":")
			for _, dependency := range task.Dependencies {
				if !strings.HasPrefix(dependency, "//") {
					dependency = prefix + ":" + dependency
				}
				entries = append(entries, dependency)
			}
		}
	}
	if len(entries) == 0 {
		return prepare(ctx, input)
	}
	catalog, config, err := Catalog(w)
	if err != nil {
		return err
	}
	catalog = append(catalog, Task{Name: "//:__one_dev_prepare", Operation: "__one_dev_prepare", Dependencies: entries, Status: "unknown"})
	opts.Name, opts.Projects = "__one_dev_prepare", nil
	p, err = planCatalog(w, opts, catalog, config)
	if err != nil {
		return err
	}
	p.Tasks = p.Tasks[:len(p.Tasks)-1]
	p.Entries = entries
	for _, task := range p.Tasks {
		if task.Interactive || task.Raw {
			return i18n.Errorf("tasks.prebuild_interactive", task.Name)
		}
	}
	if err := prepare(ctx, input); err != nil {
		return err
	}
	s.Prepare = func(context.Context, dependencies.Input) error { return nil }
	_, err = s.Execute(ctx, w, p, opts, in, out, errOut)
	return err
}
