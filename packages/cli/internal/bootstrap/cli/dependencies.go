package cli

import (
	"context"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/ci/githubactions"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	miseruntime "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/runtime/mise"
	internaltoolchain "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/toolchain"
	workspaceregistrylocal "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/workspaceregistry/local"
	ciapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/ci"
	manifestapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/manifest"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	creationmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/creation"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
	pkgci "github.com/torchstellar-team/one-cli/packages/cli/pkg/ci"
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
	ci           *ciapp.Service
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
		loaders:      secrets.MustRegistry(infisical.Loader(), dotenv.Loader()),
		ci:           mustCIService(pkgci.MustRegistry(githubactions.Provider{})),
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
	service, err := workspaceapp.NewService(backendCatalog)
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
	return service
}

func mustCIService(providers *pkgci.Registry) *ciapp.Service {
	service, err := ciapp.NewService(providers)
	if err != nil {
		panic(err)
	}
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
