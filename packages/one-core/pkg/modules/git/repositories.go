package git

import (
	"context"
	"errors"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

var ErrRepositoryNotFound = workspaceapp.ErrRegistryEntryNotFound

type RegisteredRepository struct {
	RepositoryEntry
	Status    string                            `json:"status"`
	Error     string                            `json:"error,omitempty"`
	Remotes   []Remote                          `json:"remotes"`
	Workspace *workspaceapp.RegisteredWorkspace `json:"workspace"`
}

type RepositoriesResult struct {
	Schema       string                 `json:"schema"`
	Repositories []RegisteredRepository `json:"repositories"`
}

func (s *Service) registeredRepository(ctx context.Context, workspace workspaceapp.RegisteredWorkspace) RegisteredRepository {
	result := RegisteredRepository{
		RepositoryEntry: RepositoryEntry{ID: workspace.EntryID, Name: workspace.Name, Path: workspace.Root, CommonDir: workspace.GitCommonDir, LastOpenedAt: workspace.LastSeenAt},
		Status:          "ready", Remotes: []Remote{}, Workspace: &workspace,
	}
	info, err := s.backend.Inspect(ctx, workspace.Root)
	if err != nil {
		result.Status = "unavailable"
		result.Error = err.Error()
	} else {
		result.Remotes = info.Remotes
	}
	return result
}

func (s *Service) Repositories(ctx context.Context) (*RepositoriesResult, error) {
	registry, err := s.registry.List(ctx, "")
	if err != nil {
		return nil, err
	}
	result := &RepositoriesResult{Schema: "one-cli/git-repositories/v1", Repositories: []RegisteredRepository{}}
	for _, entry := range registry.Workspaces {
		if entry.GitCommonDir != "" {
			result.Repositories = append(result.Repositories, s.registeredRepository(ctx, entry))
		}
	}
	return result, nil
}

func (s *Service) ResolveRepository(ctx context.Context, id string) (string, error) {
	entry, err := s.registry.GitLocation(ctx, id)
	if err != nil {
		return "", err
	}
	return entry.Root, nil
}

func (s *Service) Repository(ctx context.Context, id string) (*Repository, error) {
	workspace, err := s.registry.GitLocation(ctx, id)
	if err != nil {
		return nil, err
	}
	info, err := s.List(ctx, workspace.Root)
	if err != nil {
		return nil, err
	}
	info.ID, info.Name, info.Path = workspace.EntryID, workspace.Name, workspace.Root
	return info, nil
}

func (s *Service) Associate(ctx context.Context, path string) (*RegisteredRepository, error) {
	workspace, err := s.registry.Observe(ctx, path, "git")
	if err != nil {
		return nil, err
	}
	result := s.registeredRepository(ctx, workspace)
	return &result, nil
}

func (s *Service) Relocate(ctx context.Context, id, path string) (*RegisteredRepository, error) {
	workspace, err := s.registry.Relocate(ctx, id, path)
	if err != nil {
		return nil, err
	}
	result := s.registeredRepository(ctx, workspace)
	return &result, nil
}

func (s *Service) MarkRepositoryOpened(ctx context.Context, id string) error {
	workspace, err := s.registry.GitLocation(ctx, id)
	if err != nil {
		return err
	}
	_, err = s.registry.Observe(ctx, workspace.Root, "git")
	return err
}

func (s *Service) ForgetRepository(ctx context.Context, id string) error {
	err := s.registry.Forget(ctx, id)
	if errors.Is(err, workspaceapp.ErrRegistryEntryNotFound) {
		return nil
	}
	return err
}

type CloneResult struct {
	Schema     string                            `json:"schema"`
	Path       string                            `json:"path"`
	Repository *RegisteredRepository             `json:"repository,omitempty"`
	Workspace  *workspaceapp.RegisteredWorkspace `json:"workspace,omitempty"`
	Warning    string                            `json:"warning,omitempty"`
}

func (s *Service) Clone(ctx context.Context, url, path string) (*CloneResult, error) {
	created, err := s.backend.Clone(ctx, url, path)
	if err != nil {
		return nil, err
	}
	result := &CloneResult{Schema: "one-cli/git-clone/v1", Path: created}
	result.Repository, err = s.Associate(ctx, created)
	if err != nil {
		result.Warning = i18n.Tf("git.clone_workspace_failed", created, err)
	} else {
		result.Workspace = result.Repository.Workspace
	}
	return result, nil
}
