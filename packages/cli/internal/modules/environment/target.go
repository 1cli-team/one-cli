package environment

import (
	"context"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

func (s *Service) resolveInfisical(
	activeWorkspace execution.Workspace,
	profileFlag, environment, projectName string,
) (*infisical.WorkspaceConfig, *infisical.Credentials, error) {
	resolved, err := s.resolveInfisicalProfile(
		activeWorkspace, profileFlag, environment, projectName,
	)
	if err != nil || resolved == nil {
		return nil, nil, err
	}
	if resolved.Profile.Infisical == nil {
		return nil, nil, nil
	}
	value := resolved.Profile.Infisical
	config := &infisical.WorkspaceConfig{
		SiteURL: value.SiteURL, ProfileName: resolved.Name,
	}
	var credentials *infisical.Credentials
	if value.Credentials != nil {
		credentials = &infisical.Credentials{
			ClientID: value.Credentials.ClientID, ClientSecret: value.Credentials.ClientSecret,
		}
	}
	return config, credentials, nil
}

func (s *Service) resolveInfisicalProfile(
	activeWorkspace execution.Workspace,
	profileFlag, environment, projectName string,
) (*profile.Resolved, error) {
	environment = workspace.ProfileBindingEnvironment(activeWorkspace.Manifest(), environment)
	resolved, err := s.profiles.Resolve(profile.ResolveInput{
		Domain:        profile.DomainEnv,
		Backend:       workspace.EnvBackendInfisical,
		FlagOverride:  profileFlag,
		WorkspaceID:   workspace.WorkspaceID(activeWorkspace.Manifest()),
		WorkspaceRoot: activeWorkspace.Root(),
		Environment:   environment,
		ProjectName:   projectName,
	})
	if err != nil {
		if coded, ok := err.(interface{ ErrorCode() string }); ok &&
			coded.ErrorCode() == "PROFILE_NONE_CONFIGURED" {
			return nil, nil
		}
		return nil, err
	}
	return resolved, nil
}

func (s *Service) ensureInfisicalBound(
	ctx context.Context,
	activeWorkspace execution.Workspace,
	profileFlag, environment, projectName string,
) error {
	projectRoot := activeWorkspace.Root()
	config, _ := infisical.LoadWorkspaceConfig(projectRoot)
	if config != nil && strings.TrimSpace(config.ProjectID) != "" {
		return nil
	}
	resolved, err := s.resolveInfisicalProfile(
		activeWorkspace, profileFlag, environment, projectName,
	)
	if err != nil {
		return err
	}
	profileName := ""
	if resolved != nil {
		profileName = resolved.Name
	}
	_, err = s.initInfisical(ctx, projectRoot, infisical.InitInput{ProfileName: profileName})
	return err
}

func (s *Service) resolveInfisicalFolderPath(
	activeWorkspace execution.Workspace,
	config *infisical.WorkspaceConfig,
	selector string,
) (string, error) {
	projectRoot := activeWorkspace.Root()
	if config == nil {
		if existing, err := infisical.LoadWorkspaceConfig(projectRoot); err == nil && existing != nil {
			config = &infisical.WorkspaceConfig{RootPath: existing.RootPath}
		} else {
			config = &infisical.WorkspaceConfig{}
		}
	}
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
			"找不到名字或路径匹配 "+selector+" 的项目。已声明: "+
				strings.Join(activeWorkspace.ProjectNames(), ", "))
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
