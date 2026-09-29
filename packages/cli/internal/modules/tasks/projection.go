package tasks

import (
	"context"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
)

// ProjectSettings shares the effective mise catalog with the Dashboard.
func (s Service) ProjectSettings(ctx context.Context, root, project string) ([]workspaceapp.ProjectTask, error) {
	w, err := execution.ResolveWorkspaceScope(execution.NewScope(ctx, root))
	if err != nil {
		return nil, err
	}
	catalog, err := s.Catalog(ctx, w)
	if err != nil {
		return nil, err
	}
	out := []workspaceapp.ProjectTask{}
	for _, task := range catalog {
		if task.Project != project {
			continue
		}
		out = append(out, workspaceapp.ProjectTask{Name: task.Name, Operation: task.Operation, Source: task.Source, Dependencies: task.Dependencies, Outputs: task.Outputs, CacheEnabled: task.Cached})
	}
	return out, nil
}
