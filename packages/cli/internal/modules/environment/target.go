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

type BindWorkspaceInput struct {
	Scope     execution.Scope
	ProjectID string
	Create    bool
	// Revision is required by Dashboard callers, and omitted by the CLI.
	Revision string
}

// BindWorkspace is the explicit boundary for creating or selecting Infisical
// storage. Variable writes only use an existing binding.
func (s *Service) BindWorkspace(ctx context.Context, input BindWorkspaceInput) (*WorkspaceBindingResult, error) {
	if input.Create && strings.TrimSpace(input.ProjectID) != "" {
		return nil, i18n.Errorf("env.bind.create_conflict")
	}
	activeWorkspace, err := execution.ResolveWorkspaceScope(input.Scope)
	if err != nil {
		return nil, err
	}
	projectRoot := activeWorkspace.Root()
	unlock, err := fsutil.WorkspaceLock(ctx, projectRoot, "infisical-binding")
	if err != nil {
		return nil, err
	}
	defer unlock()
	unlockManifest, err := fsutil.WorkspaceLock(ctx, projectRoot, "manifest")
	if err != nil {
		return nil, err
	}
	defer unlockManifest()
	checkRevision := func() error {
		if input.Revision == "" {
			return nil
		}
		_, revision, err := workspace.ReadManifestSnapshot(projectRoot)
		if err != nil {
			return err
		}
		if input.Revision != revision {
			return cliErrors.New(cliErrors.SERVE_MANIFEST_CONFLICT, i18n.T("env.bind.revision_conflict")).WithContext(map[string]any{
				"expected_revision": input.Revision, "current_revision": revision,
			})
		}
		return nil
	}
	if err := checkRevision(); err != nil {
		return nil, err
	}
	// Re-read after locking: another explicit binding may have completed.
	config, err := infisical.LoadWorkspaceConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" && config != nil && strings.TrimSpace(config.ProjectID) != "" {
		return &WorkspaceBindingResult{
			Schema: "one-cli/env-bind/v1", BindingResult: BindingResult{ProjectID: config.ProjectID, ProjectName: config.ProjectName},
			Environments: config.Environments, WrittenTo: workspace.ManifestPath(projectRoot),
		}, nil
	}
	if projectID == "" && !input.Create {
		return nil, i18n.Errorf("env.bind.selection_required")
	}
	current, err := session.Require()
	if err != nil {
		return nil, err
	}
	initInput := infisical.InitInput{ProjectID: projectID, Session: current, BeforeWrite: checkRevision}
	if projectID != "" {
		project, err := infisical.ProjectWithSession(ctx, current, projectID)
		if err != nil {
			return nil, err
		}
		initInput.ProjectName = project.Name
		for _, environment := range project.Environments {
			initInput.Environments = append(initInput.Environments, environment.Slug)
		}
		if !contains(initInput.Environments, "dev") {
			return nil, i18n.Errorf("manifest.dev_required")
		}
	}
	result, err := s.initInfisical(ctx, projectRoot, initInput)
	if err != nil {
		return nil, err
	}
	binding := &BindingResult{ProjectID: result.ProjectID, ProjectName: result.ProjectName, Created: result.Created}
	requestedName := ""
	if identity := activeWorkspace.Manifest().Workspace; identity != nil {
		requestedName = identity.Name
	}
	if result.Created && requestedName != result.ProjectName {
		binding.RequestedName = requestedName
	}
	return &WorkspaceBindingResult{
		Schema: "one-cli/env-bind/v1", BindingResult: *binding,
		Environments: result.Environments, WrittenTo: result.WrittenTo,
	}, nil
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

// EnsureInfisicalReady is the explicit Dashboard initialization endpoint.
// Saving variables never calls it.
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
	result, err := s.BindWorkspace(ctx, BindWorkspaceInput{Scope: scope, Create: true})
	if err != nil {
		return nil, err
	}
	return &result.BindingResult, nil
}

func (s *Service) resolveInfisicalFolderPath(
	activeWorkspace execution.Workspace,
	config *infisical.WorkspaceConfig,
	selector string,
) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector != "" {
		if project, ok := activeWorkspace.Project(selector); ok {
			return infisical.ResolveSubprojectPath(project).Path, nil
		}
		if strings.HasPrefix(selector, "/") {
			return infisical.NormalizePath(selector), nil
		}
		return "", cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			i18n.Tf("workspace.project_selector_missing", selector, strings.Join(activeWorkspace.ProjectNames(), ", ")))
	}
	if project, ok := activeWorkspace.ProjectFromWorkingDirectory(); ok {
		return infisical.ResolveSubprojectPath(project).Path, nil
	}
	return "/", nil
}
