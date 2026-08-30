// Package workspace owns transport-neutral Workspace overview and selection
// mutations used by the local Dashboard HTTP boundary.
package workspace

import (
	"errors"
	"strings"
	"sync"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

var (
	ErrInvalidInput    = errors.New("workspace: invalid input")
	ErrProjectNotFound = errors.New("workspace: project not found")
)

type Service struct {
	catalog  *catalog.Catalog
	profiles ProfileAccess
	mu       sync.RWMutex
}

// ProfileAccess is the narrow machine-profile capability needed by project
// settings. The configure application service implements this interface; the
// workspace package never needs profile values and only exposes the resolved
// profile name and its precedence source.
type ProfileAccess interface {
	BindWorkspaceProfile(string, string, string, string, profile.Domain, string, string) error
	UnbindWorkspaceProfile(string, string, profile.Domain, string) error
	BindEnvironmentProfile(string, string, string, string, string, profile.Domain, string, string) error
	UnbindEnvironmentProfile(string, string, string, profile.Domain, string) error
	EnvironmentProfileBinding(string, string, string, profile.Domain, string) (string, error)
	Resolve(profile.ResolveInput) (*profile.Resolved, error)
}

// NewService constructs the workspace use-case boundary. profiles is optional
// so existing non-Dashboard callers and focused tests keep their lightweight
// construction path; production composition injects the configure service.
func NewService(backendCatalog *catalog.Catalog, profiles ...ProfileAccess) (*Service, error) {
	if backendCatalog == nil {
		return nil, errors.New("workspace: backend catalog is required")
	}
	var profileAccess ProfileAccess
	if len(profiles) > 0 {
		profileAccess = profiles[0]
	}
	return &Service{catalog: backendCatalog, profiles: profileAccess}, nil
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
