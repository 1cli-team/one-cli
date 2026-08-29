package deployment

import (
	"context"
	"fmt"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	deployport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type ProfileResolver interface {
	Resolve(profile.ResolveInput) (*profile.Resolved, error)
}

type Service struct {
	catalog   *catalog.Catalog
	providers *deployport.Registry
	profiles  ProfileResolver
	loaders   *secrets.Registry
	builder   deployport.Builder
}

func NewService(
	backendCatalog *catalog.Catalog,
	providers *deployport.Registry,
	profiles ProfileResolver,
	loaders *secrets.Registry,
	builder deployport.Builder,
) (*Service, error) {
	if backendCatalog == nil || providers == nil || profiles == nil || loaders == nil || builder == nil {
		return nil, fmt.Errorf("application: deploy catalog, providers, profiles, loaders, and builder are required")
	}
	return &Service{
		catalog: backendCatalog, providers: providers, profiles: profiles, loaders: loaders, builder: builder,
	}, nil
}

func (s *Service) apply(ctx context.Context, backend string, input deployport.ApplyInput) (*deployport.ApplyResult, error) {
	spec, ok := s.catalog.Lookup(catalog.DomainDeploy, backend)
	if !ok || !spec.Has(catalog.CapabilityDeploy) {
		return nil, cliErrors.New(
			cliErrors.BACKEND_NOT_ENABLED,
			fmt.Sprintf("deploy backend %q 不支持 deploy capability", backend),
		)
	}
	provider, ok := s.providers.Get(backend)
	if !ok {
		return nil, cliErrors.New(
			cliErrors.BACKEND_NOT_ENABLED,
			fmt.Sprintf("未知 deploy 后端 %q（可用：%v）", backend, s.providers.IDs()),
		)
	}
	return provider.Apply(ctx, input)
}
