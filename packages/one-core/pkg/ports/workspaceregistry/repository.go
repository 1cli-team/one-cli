// Package workspaceregistry defines the persistence capability required by
// the Workspace registry application service.
package workspaceregistry

import (
	"context"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
)

// Repository loads and atomically updates the machine-local Workspace index.
// Update must execute mutate while holding its read-modify-write lock so
// concurrent One CLI processes cannot lose one another's observations.
type Repository interface {
	Load(context.Context) (workspace.Registry, error)
	Update(context.Context, func(*workspace.Registry) error) error
}

type GitWorktree struct {
	ID      string
	Root    string
	Branch  string
	Main    bool
	Missing bool
	Bare    bool
}

type GitRepository struct {
	CommonDir string
	Worktrees []GitWorktree
}

type GitInspector func(context.Context, string) (*GitRepository, error)

type LegacyRepository struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	CommonDir    string    `json:"commonDir"`
	RegisteredAt time.Time `json:"registeredAt"`
	LastOpenedAt time.Time `json:"lastOpenedAt"`
}
