package environment

import (
	"context"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
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
) (*BindingResult, error) {
	projectRoot := activeWorkspace.Root()
	unlock, err := fsutil.WorkspaceLock(ctx, projectRoot, "infisical-binding")
	if err != nil {
		return nil, err
	}
	defer unlock()
	// Re-read after locking: another CLI or Dashboard save may have bound it.
	config, err := infisical.LoadWorkspaceConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	if config != nil && strings.TrimSpace(config.ProjectID) != "" {
		return &BindingResult{ProjectID: config.ProjectID, ProjectName: config.ProjectName}, nil
	}
	result, err := s.initInfisical(ctx, projectRoot, infisical.InitInput{})
	if err != nil {
		return nil, err
	}
	binding := &BindingResult{ProjectID: result.ProjectID, ProjectName: result.ProjectName, Created: result.Created}
	requestedName := ""
	if identity := activeWorkspace.Manifest().Workspace; identity != nil {
		requestedName = identity.Name
	}
	if config != nil && strings.TrimSpace(config.ProjectName) != "" {
		requestedName = config.ProjectName
	}
	if requestedName != result.ProjectName {
		binding.RequestedName = requestedName
	}
	return binding, nil
}

func requireInfisicalBackend(resolution resolution) error {
	_, err := infisical.RequireWorkspaceConfig(resolution.Workspace.Root())
	return err
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

// EnsureInfisicalReady is the Dashboard initialization write boundary, called
// only when saving the first secret. Reads and retries never initialize storage.
func (s *Service) EnsureInfisicalReady(
	ctx context.Context,
	scope execution.Scope,
	environment, project string,
) (*BindingResult, error) {
	resolution, err := s.resolve(resolveInput{
		Scope: scope, Requested: environment, AllowUnknown: true,
	})
	if err != nil {
		return nil, err
	}
	if err := validateWriteEnvironment(resolution); err != nil {
		return nil, err
	}
	declared := resolution.Declared
	if len(declared) == 0 {
		declared = workspace.DefaultEnvironments
	}
	if environment != "" && !contains(declared, environment) {
		return nil, cliErrors.New(cliErrors.ENV_UNKNOWN_ENVIRONMENT,
			i18n.Tf("env.environment_unknown", environment, strings.Join(declared, ", ")))
	}
	if _, err := s.resolveInfisicalFolderPath(resolution.Workspace, nil, project); err != nil {
		return nil, err
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
