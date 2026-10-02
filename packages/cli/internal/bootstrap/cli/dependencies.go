package cli

import (
	"context"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	miseruntime "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/runtime/mise"
	internaltoolchain "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/toolchain"
	workspaceregistrylocal "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/workspaceregistry/local"
	manifestapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/manifest"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	creationmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/creation"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/skills"
	tasksmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/tasks"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

// dependencies is the process composition graph. Transports receive
// application services, cohesive feature modules, or narrow real ports;
// vertical modules may compose their built-in adapters internally.
type dependencies struct {
	runtime      runtimeport.Provider
	catalog      *catalog.Catalog
	creation     *creationmodule.Service
	environments *environmentmodule.Service
	manifest     *manifestapp.Service
	loaders      *secrets.Registry
	workspaces   *workspaceapp.Service
	registry     *workspaceapp.RegistryService
}

func composeDependencies() dependencies {
	internaltoolchain.RegisterBundled()

	backendCatalog := catalog.Builtin()

	environments := mustEnvironmentService(backendCatalog)
	manifest := mustManifestService(backendCatalog)
	registry := mustWorkspaceRegistryService()
	creation := mustCreationService(environments, registry)

	return dependencies{
		runtime: miseruntime.Provider{},
		catalog: backendCatalog,

		creation:     creation,
		environments: environments,
		manifest:     manifest,
		loaders:      secrets.MustRegistry(infisical.Loader()),
		workspaces:   mustWorkspaceService(backendCatalog),
		registry:     registry,
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

func mustWorkspaceService(
	backendCatalog *catalog.Catalog,
) *workspaceapp.Service {
	service, err := workspaceapp.NewService(backendCatalog, (tasksmodule.Service{Provider: miseruntime.Provider{}}).ProjectSettings)
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
