// Package workspaceregistry defines the persistence capability required by
// the Workspace registry application service.
package workspaceregistry

import (
	"context"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

// Repository loads and atomically updates the machine-local Workspace index.
// Update must execute mutate while holding its read-modify-write lock so
// concurrent One CLI processes cannot lose one another's observations.
type Repository interface {
	Load(context.Context) (workspace.Registry, error)
	Update(context.Context, func(*workspace.Registry) error) error
}
