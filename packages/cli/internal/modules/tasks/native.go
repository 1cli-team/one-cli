package tasks

import (
	"context"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
)

// Listing, planning and execution use the same static catalog, without running mise tasks.
func (s Service) Plan(_ context.Context, w execution.Workspace, opts Options) (*Plan, error) {
	return NewPlan(w, opts)
}
func (s Service) Catalog(_ context.Context, w execution.Workspace) ([]Task, error) {
	catalog, _, err := Catalog(w)
	visible := []Task{}
	for _, task := range catalog {
		if !task.hidden {
			visible = append(visible, task)
		}
	}
	return visible, err
}
