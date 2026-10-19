package environment

import (
	"context"
	"errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisical"
	session "github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisicalsession"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
	"strings"
)

func (s *Service) resolveInfisical(r resolution) (*infisical.WorkspaceConfig, *infisical.Credentials, error) {
	current, err := session.Require(r.Scope.Context())
	if err != nil {
		return nil, nil, err
	}
	if err = requireInfisicalBackend(r); err != nil {
		return nil, nil, err
	}
	binding := r.Workspace.Manifest().Env
	cfg := &infisical.WorkspaceConfig{SiteURL: binding.SiteURL, ProjectID: binding.ProjectID, Environments: binding.Environments}
	if cfg.SiteURLOrDefault() != current.SiteURL {
		return nil, nil, i18n.ForContext(r.Scope.Context()).Errorf("infisical.binding_instance_mismatch")
	}
	return cfg, &infisical.Credentials{AccessToken: current.Token}, nil
}

type BindWorkspaceInput struct {
	Scope     execution.Scope
	ProjectID string
	Create    bool
	Revision  string
}

func (s *Service) BindWorkspace(ctx context.Context, input BindWorkspaceInput) (*WorkspaceBindingResult, error) {
	if input.Create && strings.TrimSpace(input.ProjectID) != "" {
		return nil, i18n.ForContext(ctx).Errorf("env.bind.create_conflict")
	}
	active, err := s.resolveWorkspace(input.Scope)
	if err != nil {
		return nil, err
	}
	id := active.Root()
	manifest, revision, err := s.store.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Revision != "" && input.Revision != revision {
		return nil, cliErrors.New(cliErrors.SERVE_MANIFEST_CONFLICT, i18n.ForContext(ctx).T("env.bind.revision_conflict"))
	}
	current, err := session.Require(ctx)
	if err != nil {
		return nil, err
	}
	var p *infisical.RemoteProject
	created := false
	if input.ProjectID != "" {
		p, err = infisical.Project(ctx, input.ProjectID)
	} else if manifest.Env != nil && !input.Create {
		p, err = infisical.Project(ctx, manifest.Env.ProjectID)
	} else if input.Create {
		p, err = infisical.CreateBoundProject(ctx, manifest.Workspace.Name)
		created = true
	} else {
		return nil, i18n.ForContext(ctx).Errorf("env.bind.selection_required")
	}
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, e := range p.Environments {
		names = append(names, e.Slug)
	}
	if !contains(names, "dev") {
		return nil, i18n.ForContext(ctx).Errorf("manifest.dev_required")
	}
	manifest.Env = &workspace.EnvironmentConfig{SiteURL: current.SiteURL, ProjectID: p.ID, Environments: names}
	err = session.WithUnchanged(ctx, current, func() error { _, e := s.store.Save(ctx, id, manifest, revision); return e })
	if err != nil {
		if !created {
			return nil, err
		}
		code := cliErrors.ONE_CLI_ERROR
		var original *output.Error
		if errors.Is(err, store.ErrConflict) {
			code = cliErrors.SERVE_MANIFEST_CONFLICT
		} else if errors.As(err, &original) {
			code = cliErrors.Code(original.Code)
		}
		return nil, cliErrors.New(code, i18n.ForContext(ctx).Errorf("infisical.init.binding_write_failed", p.Name, p.ID, "server:"+id, err).Error()).WithContext(map[string]any{"project_id": p.ID, "project_name": p.Name, "partial_state": "project_created_binding_unsaved"}).WithCause(err)
	}
	return &WorkspaceBindingResult{BindingResult: BindingResult{ProjectID: p.ID, ProjectName: p.Name, Created: created}, Schema: "one-cli/env-bind/v1", Environments: names, WrittenTo: "server:" + id}, nil
}
func requireInfisicalBackend(r resolution) error {
	if r.Workspace.Manifest().Env == nil || r.Workspace.Manifest().Env.ProjectID == "" {
		return cliErrors.New(cliErrors.INFISICAL_NOT_CONFIGURED, i18n.ForContext(r.Scope.Context()).T("infisical.config_missing"))
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
			i18n.ForContext(activeWorkspace.Scope().Context()).Tf("workspace.project_selector_missing", selector, strings.Join(activeWorkspace.ProjectNames(), ", ")))
	}
	if project, ok := activeWorkspace.ProjectFromWorkingDirectory(); ok {
		return infisical.ResolveSubprojectPath(project).Path, nil
	}
	return "/", nil
}
