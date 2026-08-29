// Package workspace owns transport-neutral Workspace overview and selection
// mutations used by the local Dashboard HTTP boundary.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

var (
	ErrInvalidInput    = errors.New("workspace: invalid input")
	ErrProjectNotFound = errors.New("workspace: project not found")
)

type Service struct {
	catalog *catalog.Catalog
}

func NewService(backendCatalog *catalog.Catalog) (*Service, error) {
	if backendCatalog == nil {
		return nil, errors.New("workspace: backend catalog is required")
	}
	return &Service{catalog: backendCatalog}, nil
}

func (s *Service) Overview(root string) (workspacecore.Overview, error) {
	return workspacecore.BuildOverview(root)
}

func (s *Service) SetEnvironment(root, backend string) (workspacecore.Overview, error) {
	backend = strings.TrimSpace(backend)
	if _, ok := s.catalog.Lookup(catalog.DomainEnv, backend); !ok {
		return workspacecore.Overview{}, fmt.Errorf("%w: unknown env backend %q", ErrInvalidInput, backend)
	}
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		return workspacecore.Overview{}, err
	}
	if manifest.Domains == nil {
		manifest.Domains = &workspacecore.WorkspaceDomains{}
	}
	if manifest.Domains.Env == nil {
		manifest.Domains.Env = &workspacecore.BackendRef{}
	}
	manifest.Domains.Env.Kind = backend
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		return workspacecore.Overview{}, err
	}
	return s.Overview(root)
}

func (s *Service) SetProjectDeployment(
	ctx context.Context,
	root, projectName, backend string,
) (workspacecore.Overview, error) {
	backend = strings.TrimSpace(backend)
	if _, ok := s.catalog.Lookup(catalog.DomainDeploy, backend); !ok {
		return workspacecore.Overview{}, fmt.Errorf("%w: unknown deploy backend %q", ErrInvalidInput, backend)
	}
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		return workspacecore.Overview{}, err
	}
	project := findProject(manifest, projectName)
	if project == nil {
		return workspacecore.Overview{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectName)
	}
	registry, err := template.Fetch(ctx, "")
	if err != nil {
		return workspacecore.Overview{}, err
	}
	if !templateAllowsDeployment(registry, project.TemplateID, backend) {
		return workspacecore.Overview{}, fmt.Errorf(
			"%w: deploy backend %q is not compatible with template %q",
			ErrInvalidInput,
			backend,
			project.TemplateID,
		)
	}
	if project.Domains == nil {
		project.Domains = &workspacecore.ProjectDomains{}
	}
	if project.Domains.Deploy == nil {
		project.Domains.Deploy = &workspacecore.ProjectDeployBackend{}
	}
	project.Domains.Deploy.Kind = backend
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		return workspacecore.Overview{}, err
	}
	return s.Overview(root)
}

func (s *Service) SetProjectContainer(
	root, projectName, backend, image string,
) (workspacecore.Overview, error) {
	backend = strings.TrimSpace(backend)
	if backend != "" {
		if _, ok := s.catalog.Lookup(catalog.DomainContainer, backend); !ok {
			return workspacecore.Overview{}, fmt.Errorf("%w: unknown container backend %q", ErrInvalidInput, backend)
		}
	}
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		return workspacecore.Overview{}, err
	}
	project := findProject(manifest, projectName)
	if project == nil {
		return workspacecore.Overview{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectName)
	}
	if project.Domains == nil {
		project.Domains = &workspacecore.ProjectDomains{}
	}
	if project.Domains.Container == nil {
		project.Domains.Container = &workspacecore.ProjectContainerOverride{}
	}
	project.Domains.Container.Kind = backend
	project.Domains.Container.Image = strings.TrimSpace(image)
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		return workspacecore.Overview{}, err
	}
	return s.Overview(root)
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

func templateAllowsDeployment(registry *template.Registry, templateID, backend string) bool {
	if registry == nil {
		return false
	}
	for _, entry := range registry.Templates {
		if entry.ID == templateID {
			return slices.Contains(entry.Compat["deploy"], backend)
		}
	}
	return false
}
