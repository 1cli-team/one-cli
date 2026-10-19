// Package workspace owns read-only Workspace metadata used by the local
// Dashboard HTTP boundary.
package workspace

import (
	"context"
	"errors"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
	"strings"

	workspacecore "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
)

var (
	ErrInvalidInput    = errors.New("workspace: invalid input")
	ErrProjectNotFound = errors.New("workspace: project not found")
)

type Service struct{ store *store.Store }

func NewService(db *store.Store) *Service { return &Service{store: db} }

func (s *Service) Overview(ctx context.Context, root string, environments ...string) (workspacecore.Overview, error) {
	environment := ""
	if len(environments) > 0 {
		environment = strings.TrimSpace(environments[0])
	}
	m, _, err := s.store.Load(ctx, root)
	if err != nil {
		return workspacecore.Overview{}, err
	}
	return workspacecore.OverviewFromManifest(m, "", environment), nil
}

func findProject(manifest *workspacecore.Manifest, name string) *workspacecore.ManifestProject {
	if manifest == nil {
		return nil
	}
	for index := range manifest.Projects {
		if manifest.Projects[index].Name == name {
			return &manifest.Projects[index]
		}
	}
	return nil
}
