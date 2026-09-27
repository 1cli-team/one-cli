package environment

import (
	"context"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
)

func (s *Service) resolveInfisical() (*infisical.WorkspaceConfig, *infisical.Credentials, error) {
	current, err := session.Require()
	if err != nil {
		return nil, nil, err
	}
	return &infisical.WorkspaceConfig{SiteURL: current.SiteURL}, &infisical.Credentials{AccessToken: current.Token}, nil
}

func (s *Service) ensureInfisicalBound(
	ctx context.Context,
	activeWorkspace execution.Workspace,
) error {
	projectRoot := activeWorkspace.Root()
	config, _ := infisical.LoadWorkspaceConfig(projectRoot)
	if config != nil && strings.TrimSpace(config.ProjectID) != "" {
		return nil
	}
	_, err := s.initInfisical(ctx, projectRoot, infisical.InitInput{})
	return err
}

func requireInfisicalBackend(resolution resolution) error {
	if resolution.Scope.Backend().Name != workspace.EnvBackendInfisical {
		return cliErrors.New(
			cliErrors.ENV_BACKEND_INVALID,
			i18n.T("env.infisical_required"),
		)
	}
	return nil
}

func (s *Service) RequireInfisicalBackend(
	scope execution.Scope,
	environment string,
) error {
	resolution, err := s.resolve(resolveInput{
		Scope: scope, Requested: environment, AllowUnknown: true,
	})
	if err != nil {
		return err
	}
	return requireInfisicalBackend(resolution)
}

// EnsureInfisicalReady completes a missing project binding for a Workspace
// whose selected env backend is already Infisical. It is the explicit repair
// boundary used by the Dashboard; ordinary secret reads remain read-only.
func (s *Service) EnsureInfisicalReady(
	ctx context.Context,
	scope execution.Scope,
	environment, project string,
) error {
	resolution, err := s.resolve(resolveInput{
		Scope: scope, Requested: environment, AllowUnknown: true,
	})
	if err != nil {
		return err
	}
	if err := requireInfisicalBackend(resolution); err != nil {
		return err
	}
	return s.ensureInfisicalBound(ctx, resolution.Workspace)
}

func (s *Service) resolveInfisicalFolderPath(
	activeWorkspace execution.Workspace,
	config *infisical.WorkspaceConfig,
	selector string,
) (string, error) {
	projectRoot := activeWorkspace.Root()
	// Path metadata always comes from the workspace, independently of session credentials.
	stored, err := infisical.LoadWorkspaceConfig(projectRoot)
	if err != nil {
		return "", err
	}
	pathConfig := &infisical.WorkspaceConfig{}
	if config != nil {
		*pathConfig = *config
	}
	if stored != nil {
		pathConfig.RootPath = stored.RootPath
	}
	config = pathConfig

	selector = strings.TrimSpace(selector)
	if selector != "" {
		if project, ok := activeWorkspace.Project(selector); ok {
			override, err := infisical.LoadSubprojectConfig(projectRoot, project.RelativeDir)
			if err != nil {
				return "", err
			}
			return infisical.ResolveSubprojectPath(config, project, override).Path, nil
		}
		if strings.HasPrefix(selector, "/") {
			return infisical.NormalizePath(selector), nil
		}
		return "", cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			i18n.Tf("workspace.project_selector_missing", selector, strings.Join(activeWorkspace.ProjectNames(), ", ")))
	}
	if project, ok := activeWorkspace.ProjectFromWorkingDirectory(); ok {
		override, err := infisical.LoadSubprojectConfig(projectRoot, project.RelativeDir)
		if err != nil {
			return "", err
		}
		return infisical.ResolveSubprojectPath(config, project, override).Path, nil
	}
	return infisical.NormalizePath(config.RootPathOrDefault()), nil
}
