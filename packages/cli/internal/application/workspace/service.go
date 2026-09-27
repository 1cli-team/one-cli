// Package workspace owns transport-neutral Workspace overview and selection
// mutations used by the local Dashboard HTTP boundary.
package workspace

import (
	"errors"
	"strings"
	"sync"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

var (
	ErrInvalidInput    = errors.New("workspace: invalid input")
	ErrProjectNotFound = errors.New("workspace: project not found")
)

type Service struct {
	catalog *catalog.Catalog
	mu      sync.RWMutex
}

func NewService(backendCatalog *catalog.Catalog) (*Service, error) {
	if backendCatalog == nil {
		return nil, errors.New("workspace: backend catalog is required")
	}
	return &Service{catalog: backendCatalog}, nil
}

func (s *Service) Overview(root string, environments ...string) (workspacecore.Overview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	environment := ""
	if len(environments) > 0 {
		environment = strings.TrimSpace(environments[0])
	}
	return workspacecore.BuildOverview(root, environment)
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
