package cli

import (
	"context"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/ci/githubactions"
	deploybuild "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/deploy/build"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/deploy/cloudflare"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/deploy/edgeone"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/deploy/kustomize"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/deploy/s3compat"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/deploy/vercel"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	internaltoolchain "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/toolchain"
	workspaceregistrylocal "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/workspaceregistry/local"
	ciapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/ci"
	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	deploymentapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/deployment"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	containermodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/container"
	creationmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/creation"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	deployport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
	pkgci "github.com/torchstellar-team/one-cli/packages/cli/pkg/ci"
)

// dependencies is the process composition graph. Transports receive
// application services, cohesive feature modules, or narrow real ports;
// vertical modules may compose their built-in adapters internally.
type dependencies struct {
	catalog      *catalog.Catalog
	profiles     *configureapp.ProfileService
	containers   *containermodule.Service
	creation     *creationmodule.Service
	environments *environmentmodule.Service
	loaders      *secrets.Registry
	ci           *ciapp.Service
	workspaces   *workspaceapp.Service
	registry     *workspaceapp.RegistryService
}

func composeDependencies() dependencies {
	internaltoolchain.RegisterBundled()

	backendCatalog := catalog.Builtin()
	profiles := mustProfileService(backendCatalog)
	containers := mustContainerService(backendCatalog)
	environments := mustEnvironmentService(backendCatalog, profiles)
	registry := mustWorkspaceRegistryService()
	creation := mustCreationService(environments, registry)

	return dependencies{
		catalog:      backendCatalog,
		profiles:     profiles,
		containers:   containers,
		creation:     creation,
		environments: environments,
		loaders:      secrets.MustRegistry(infisical.Loader(), dotenv.Loader()),
		ci:           mustCIService(pkgci.MustRegistry(githubactions.Provider{})),
		workspaces:   mustWorkspaceService(backendCatalog, profiles),
		registry:     registry,
	}
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
	profiles *configureapp.ProfileService,
) *workspaceapp.Service {
	service, err := workspaceapp.NewService(backendCatalog, profiles)
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
	profiles *configureapp.ProfileService,
) *environmentmodule.Service {
	service, err := environmentmodule.NewService(backendCatalog, profiles)
	if err != nil {
		panic(err)
	}
	return service
}

func (d dependencies) newDeploymentService(buildVersion string) *deploymentapp.Service {
	providers := []deployport.Provider{
		kustomize.NewProvider(buildVersion),
		vercel.Provider(),
		cloudflare.Provider(),
		edgeone.Provider(),
	}
	providers = append(providers, s3compat.Providers()...)
	service, err := deploymentapp.NewService(
		d.catalog,
		deployport.MustRegistry(providers...),
		d.profiles,
		d.loaders,
		deploybuild.Local{},
	)
	if err != nil {
		panic(err)
	}
	return service
}

func mustProfileService(backendCatalog *catalog.Catalog) *configureapp.ProfileService {
	service, err := configureapp.NewProfileService(backendCatalog, configureapp.LocalProfileRepository{})
	if err != nil {
		panic(err)
	}
	return service
}

func mustContainerService(
	backendCatalog *catalog.Catalog,
) *containermodule.Service {
	service, err := containermodule.NewService(backendCatalog)
	if err != nil {
		panic(err)
	}
	return service
}
