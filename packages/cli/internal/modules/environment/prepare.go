package environment

import (
	"context"
	"fmt"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

// PrepareWorkspaceInput contains the workspace-level environment setup that
// runs after create has persisted its backend selection.
type PrepareWorkspaceInput struct {
	ProjectRoot string
	ProjectName string
	Backend     string
}

// PrepareWorkspaceResult keeps remote binding best-effort while letting the
// transport decide how to present the warning.
type PrepareWorkspaceResult struct {
	InfisicalBound bool
	BindWarning    error
}

// PrepareWorkspace owns the create-time environment setup order. Local env
// ignore rules are required for every backend because remote pulls also write
// local .env files. Infisical binding remains best-effort and is retried by
// the first remote env operation when it has not completed here.
func (s *Service) PrepareWorkspace(
	ctx context.Context,
	input PrepareWorkspaceInput,
) (PrepareWorkspaceResult, error) {
	backend := strings.TrimPrefix(strings.TrimSpace(input.Backend), "env/")
	spec, ok := s.catalog.Lookup(catalog.DomainEnv, backend)
	if !ok || !spec.Has(catalog.CapabilityScaffold) {
		return PrepareWorkspaceResult{}, cliErrors.New(
			cliErrors.ENV_BACKEND_INVALID,
			fmt.Sprintf("不支持的 env backend %q", backend),
		)
	}
	if err := dotenv.Sync(input.ProjectRoot); err != nil {
		return PrepareWorkspaceResult{}, cliErrors.New(
			cliErrors.STATUS_FIX_FAILED,
			fmt.Sprintf("env/dotenv 同步失败: %v", err),
		)
	}
	if backend == workspace.EnvBackendDotenv {
		return PrepareWorkspaceResult{}, nil
	}

	_, err := infisical.Init(ctx, input.ProjectRoot, infisical.InitInput{
		ProjectName: input.ProjectName,
	})
	if err != nil {
		return PrepareWorkspaceResult{BindWarning: err}, nil
	}
	return PrepareWorkspaceResult{InfisicalBound: true}, nil
}
