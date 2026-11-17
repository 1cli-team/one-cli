package environment

import (
	"context"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type BindWorkspaceInput struct {
	Scope     execution.Scope
	ProjectID string
	Create    bool
	// Revision is required by Desktop callers, and omitted by the CLI.
	Revision string
}

func (s *Service) BindWorkspace(ctx context.Context, input BindWorkspaceInput) (*WorkspaceBindingResult, error) {
	resolved, err := s.resolve(resolveInput{Scope: input.Scope, AllowUnknown: true})
	if err != nil {
		return nil, err
	}
	provider, err := s.backend(resolved)
	if err != nil {
		return nil, err
	}
	return provider.BindWorkspace(ctx, input)
}

func (s *Service) RequireBackend(scope execution.Scope, environment string) error {
	resolved, err := s.resolve(resolveInput{Scope: scope, Requested: environment, AllowUnknown: true})
	if err != nil {
		return err
	}
	provider, err := s.backend(resolved)
	if err != nil {
		return err
	}
	return provider.Require(resolved.Workspace.Root())
}

func resolveFolderPath(activeWorkspace execution.Workspace, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector != "" {
		if project, ok := activeWorkspace.Project(selector); ok {
			return "/" + project.RelativeDir, nil
		}
		if strings.HasPrefix(selector, "/") {
			return selector, nil
		}
		return "", cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			i18n.Tf("workspace.project_selector_missing", selector, strings.Join(activeWorkspace.ProjectNames(), ", ")))
	}
	if project, ok := activeWorkspace.ProjectFromWorkingDirectory(); ok {
		return "/" + project.RelativeDir, nil
	}
	return "/", nil
}
