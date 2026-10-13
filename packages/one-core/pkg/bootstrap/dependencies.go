package bootstrap

import (
	"context"

	"github.com/torchstellar-team/one-cli/packages/one-core/internal/adapters/env/infisical"
	miseruntime "github.com/torchstellar-team/one-cli/packages/one-core/internal/adapters/runtime/mise"
	workspaceregistrylocal "github.com/torchstellar-team/one-cli/packages/one-core/internal/adapters/workspaceregistry/local"
	manifestapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/manifest"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	catalog "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/backend"
	creationmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/creation"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/environment"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/prompt"
	runtimeport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/secrets"
)

// dependencies is the process composition graph. Transports receive
// application services, cohesive feature modules, or narrow real ports;
// vertical modules may compose their built-in adapters internally.
type Dependencies struct {
	Runtime      runtimeport.Provider
	Catalog      *catalog.Catalog
	Creation     *creationmodule.Service
	Environments *environmentmodule.Service
	Manifest     *manifestapp.Service
	Loaders      *secrets.Registry
	Workspaces   *workspaceapp.Service
	Registry     *workspaceapp.RegistryService
}

func Compose() Dependencies {
	backendCatalog := catalog.Builtin()

	environments := mustEnvironmentService(backendCatalog)
	manifest := mustManifestService(backendCatalog)
	registry := mustWorkspaceRegistryService()
	creation := mustCreationService(environments, registry)

	return Dependencies{
		Runtime: miseruntime.Provider{},
		Catalog: backendCatalog,

		Creation:     creation,
		Environments: environments,
		Manifest:     manifest,
		Loaders:      secrets.MustRegistry(infisical.Loader()),
		Workspaces:   workspaceapp.NewService(),
		Registry:     registry,
	}
}

func mustManifestService(backendCatalog *catalog.Catalog) *manifestapp.Service {
	service, err := manifestapp.NewService(backendCatalog)
	if err != nil {
		panic(err)
	}
	return service
}

func mustWorkspaceRegistryService() *workspaceapp.RegistryService {
	repository, err := workspaceregistrylocal.New()
	if err != nil {
		panic(err)
	}
	service, err := workspaceapp.NewRegistryService(repository)
	if err != nil {
		panic(err)
	}
	return service
}

func mustCreationService(
	environments *environmentmodule.Service,
	registry *workspaceapp.RegistryService,
) *creationmodule.Service {
	observe := creationmodule.WorkspaceObserver(func(ctx context.Context, root, source string) error {
		_, err := registry.Observe(ctx, root, source)
		return err
	})
	service, err := creationmodule.NewService(environments, observe)
	if err != nil {
		panic(err)
	}
	service.Runtime = miseruntime.Provider{}
	service.Skills = skillsmodule.Installer{Run: (skillsmodule.Runner{Runtime: service.Runtime}).Run, Progress: prompt.ReportProgress}
	return service
}

func mustEnvironmentService(
	backendCatalog *catalog.Catalog,
) *environmentmodule.Service {
	service, err := environmentmodule.NewService(backendCatalog)
	if err != nil {
		panic(err)
	}
	return service
}
