package bootstrap

import (
	"context"

	miseruntime "github.com/torchstellar-team/one-cli/packages/one-core/internal/adapters/runtime/mise"
	workspaceregistrylocal "github.com/torchstellar-team/one-cli/packages/one-core/internal/adapters/workspaceregistry/local"
	manifestapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/manifest"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	catalog "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/backend"
	creationmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/creation"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/environment"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/workspacegroups"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/prompt"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins"
	runtimeport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/secrets"
)

// dependencies is the process composition graph. Transports receive
// application services, cohesive feature modules, or narrow real ports;
// vertical modules may compose their built-in adapters internally.
type Dependencies struct {
	Git          *gitmodule.Service
	Runtime      runtimeport.Provider
	Catalog      *catalog.Catalog
	Creation     *creationmodule.Service
	Environments *environmentmodule.Service
	Manifest     *manifestapp.Service
	Loaders      *secrets.Registry
	Global       secrets.GlobalLoader
	Workspaces   *workspaceapp.Service
	Registry     *workspaceapp.RegistryService
	Groups       *workspacegroups.Service
}

// ComposePlugins builds an isolated application from explicitly installed plugins.
func ComposePlugins(installed ...plugins.Plugin) (Dependencies, error) {
	pluginRegistry, err := plugins.New(installed...)
	if err != nil {
		return Dependencies{}, err
	}
	backendCatalog, environments := pluginRegistry.Catalog, pluginRegistry.Environments
	manifest := mustManifestService(backendCatalog)
	var registryOptions workspaceapp.RegistryOptions
	if pluginRegistry.Git != nil {
		registryOptions.WorktreeIdentity = pluginRegistry.Git.WorkspaceIdentity
		registryOptions.Git = pluginRegistry.Git.InspectWorkspaceRepository
		registryOptions.LegacyGit = gitmodule.LegacyRepositories
	}
	registry := mustWorkspaceRegistryService(registryOptions)
	var gitService *gitmodule.Service
	if pluginRegistry.Git != nil {
		gitService = gitmodule.NewService(pluginRegistry.Git, registry)
	}
	creation := mustCreationService(backendCatalog, registry)

	return Dependencies{
		Git:     gitService,
		Runtime: miseruntime.Provider{},
		Catalog: backendCatalog,

		Creation:     creation,
		Environments: environments,
		Manifest:     manifest,
		Loaders:      pluginRegistry.Loaders,
		Global:       pluginRegistry.Global,
		Workspaces:   workspaceapp.NewService(),
		Registry:     registry,
		Groups:       workspacegroups.NewService(registry),
	}, nil
}

func mustManifestService(backendCatalog *catalog.Catalog) *manifestapp.Service {
	service, err := manifestapp.NewService(backendCatalog)
	if err != nil {
		panic(err)
	}
	return service
}

func mustWorkspaceRegistryService(options workspaceapp.RegistryOptions) *workspaceapp.RegistryService {
	repository, err := workspaceregistrylocal.New()
	if err != nil {
		panic(err)
	}
	service, err := workspaceapp.NewRegistryService(repository, options)
	if err != nil {
		panic(err)
	}
	return service
}

func mustCreationService(
	backendCatalog *catalog.Catalog,
	registry *workspaceapp.RegistryService,
) *creationmodule.Service {
	observe := creationmodule.WorkspaceObserver(func(ctx context.Context, root, source string) error {
		_, err := registry.Observe(ctx, root, source)
		return err
	})
	service, err := creationmodule.NewService(backendCatalog, observe)
	if err != nil {
		panic(err)
	}
	service.Runtime = miseruntime.Provider{}
	skillsRunner := skillsmodule.Runner{Runtime: service.Runtime}
	service.Skills = skillsmodule.Installer{Run: skillsRunner.Run, Hash: skillsRunner.Hash, Inspect: skillsRunner.Inspect, InstallGroup: skillsRunner.InstallGroup, Progress: prompt.ReportProgress}
	return service
}
