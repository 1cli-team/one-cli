// Package git defines the shared Git capability used by CLI and Desktop.
package git

import (
	"context"
	"path/filepath"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	registryport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/workspaceregistry"
)

type Worktree struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	Head          string `json:"head"`
	Branch        string `json:"branch"`
	Main          bool   `json:"main"`
	Bare          bool   `json:"bare"`
	Detached      bool   `json:"detached"`
	Locked        bool   `json:"locked"`
	LockReason    string `json:"lockReason,omitempty"`
	Prunable      bool   `json:"prunable"`
	Missing       bool   `json:"missing"`
	Dirty         bool   `json:"dirty"`
	StatusError   string `json:"statusError,omitempty"`
	WorkspaceRoot string `json:"workspaceRoot,omitempty"`
}

type Repository struct {
	Schema        string     `json:"schema"`
	ID            string     `json:"id,omitempty"`
	Name          string     `json:"name,omitempty"`
	Path          string     `json:"path,omitempty"`
	CommonDir     string     `json:"commonDir"`
	DefaultParent string     `json:"defaultParent"`
	Remotes       []Remote   `json:"remotes"`
	Branches      []string   `json:"branches"`
	Worktrees     []Worktree `json:"worktrees"`
}

type Remote struct {
	Name     string `json:"name"`
	FetchURL string `json:"fetchUrl"`
	PushURL  string `json:"pushUrl"`
}

type AddInput struct {
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	StartPoint string `json:"startPoint"`
	NewBranch  bool   `json:"newBranch"`
	Detached   bool   `json:"detached"`
}

type Backend interface {
	Inspect(context.Context, string) (*Repository, error)
	CreateBranch(context.Context, string, CreateBranchInput) (*BranchResult, error)
	Add(context.Context, string, AddInput) (*Worktree, error)
	Remove(context.Context, string, string) error
	WorkspaceIdentity(string) (string, error)
	InspectWorkspaceRepository(context.Context, string) (*registryport.GitRepository, error)
	Clone(context.Context, string, string) (string, error)
}

type Service struct {
	backend  Backend
	registry *workspaceapp.RegistryService
}

func NewService(backend Backend, registry *workspaceapp.RegistryService) *Service {
	return &Service{backend: backend, registry: registry}
}

func (s *Service) List(ctx context.Context, root string) (*Repository, error) {
	repository, err := s.backend.Inspect(ctx, root)
	if err != nil {
		return nil, err
	}
	return describeWorkspaces(repository, root)
}

func describeWorkspaces(repository *Repository, root string) (*Repository, error) {
	// A One workspace may be nested below the Git root. Project the same
	// manifest location into every linked worktree.
	source, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return nil, err
	}
	// Linked worktrees can live inside the main worktree. Prefer the closest
	// matching Git root so the main tree cannot capture a nested checkout.
	var worktreeRoot string
	for _, tree := range repository.Worktrees {
		rel, err := filepath.Rel(tree.Path, source)
		if err == nil && filepath.IsLocal(rel) && len(tree.Path) > len(worktreeRoot) {
			worktreeRoot = tree.Path
		}
	}
	relative := "."
	if manifestRoot, err := workspace.WalkUpToManifest(source); worktreeRoot != "" && err == nil {
		if rel, err := filepath.Rel(worktreeRoot, manifestRoot); err == nil && filepath.IsLocal(rel) {
			relative = rel
		}
	}
	for index := range repository.Worktrees {
		tree := &repository.Worktrees[index]
		candidate := filepath.Join(tree.Path, relative)
		if _, err := workspace.ReadManifest(candidate); err == nil {
			tree.WorkspaceRoot = candidate
		}
	}
	return repository, nil
}

type ChangeResult struct {
	Schema    string                            `json:"schema"`
	Worktree  *Worktree                         `json:"worktree"`
	Workspace *workspaceapp.RegisteredWorkspace `json:"workspace,omitempty"`
	Warning   string                            `json:"warning,omitempty"`
}

func (s *Service) Add(ctx context.Context, root string, input AddInput) (*ChangeResult, error) {
	tree, err := s.backend.Add(ctx, root, input)
	if err != nil {
		return nil, err
	}
	result := &ChangeResult{Schema: "one-cli/git-worktree-change/v1", Worktree: tree}
	// The Git operation has committed. Registration failures are returned with
	// the successful worktree so callers can recover without replaying creation.
	repository, err := s.List(ctx, root)
	if err != nil {
		result.Warning = err.Error()
		return result, nil
	}
	for _, entry := range repository.Worktrees {
		if entry.ID != tree.ID {
			continue
		}
		result.Worktree = &entry
		if entry.WorkspaceRoot != "" && s.registry != nil {
			registered, err := s.registry.Observe(ctx, entry.WorkspaceRoot, "git")
			if err != nil {
				result.Warning = err.Error()
			} else {
				result.Workspace = &registered
			}
		}
		break
	}
	return result, nil
}

func (s *Service) Open(ctx context.Context, root, id string) (*workspaceapp.RegisteredWorkspace, error) {
	repository, err := s.List(ctx, root)
	if err != nil {
		return nil, err
	}
	for _, tree := range repository.Worktrees {
		if tree.ID == id && tree.WorkspaceRoot != "" && s.registry != nil {
			entry, err := s.registry.Observe(ctx, tree.WorkspaceRoot, "git")
			return &entry, err
		}
	}
	return nil, i18n.Errorf("git.workspace_unavailable")
}

func (s *Service) Remove(ctx context.Context, root, id string) error {
	// The registry already prunes missing entries on its next read. Its refresh
	// is independent of the committed Git removal.
	return s.backend.Remove(ctx, root, id)
}
