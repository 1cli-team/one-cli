package ci

import (
	"context"
	"fmt"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	pkgci "github.com/torchstellar-team/one-cli/packages/cli/pkg/ci"
)

type EnableRequest struct {
	Selector string
	Provider string
}

func (s *Service) Status(ctx context.Context) (*StatusResult, error) {
	activeWorkspace, err := execution.ResolveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	root := activeWorkspace.Root()
	manifest := activeWorkspace.Manifest()
	result := &StatusResult{
		Schema: "one-cli/ci-status/v1", Projects: make([]ProjectStatus, 0, len(manifest.Projects)),
		NextCommand: "one add",
	}
	for _, project := range activeWorkspace.Projects() {
		path := s.workflowPath(root, project, pkgci.DefaultProviderID)
		enabled, err := workflowExists(path)
		if err != nil {
			return nil, err
		}
		item := ProjectStatus{
			Name: project.Name, Enabled: enabled, WorkflowPath: relativeWorkflowPath(root, path),
		}
		if enabled {
			item.Provider = pkgci.DefaultProviderID
			result.Configured = true
			result.Provider = pkgci.DefaultProviderID
		}
		result.Projects = append(result.Projects, item)
	}
	for _, project := range result.Projects {
		if !project.Enabled {
			result.NextCommand = "one ci enable " + project.Name
			return result, nil
		}
	}
	if len(result.Projects) > 0 {
		result.NextCommand = "one ci sync"
	}
	return result, nil
}

func (s *Service) Enable(ctx context.Context, request EnableRequest) (*ActionResult, error) {
	activeWorkspace, err := execution.ResolveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	projects, err := s.selectProjects(activeWorkspace, request.Selector, true)
	if err != nil {
		return nil, err
	}
	providerID, err := s.resolveProviderID(request.Provider)
	if err != nil {
		return nil, err
	}
	root := activeWorkspace.Root()
	items := make([]ActionProject, 0, len(projects))
	for _, project := range projects {
		result, err := s.syncProject(root, project, providerID)
		if err != nil {
			return nil, renderError(project.Name, providerID, err)
		}
		status := "updated"
		if result.created {
			status = "created"
		}
		items = append(items, ActionProject{
			Name: project.Name, Status: status, Enabled: true, Provider: providerID,
			WorkflowPath: relativeWorkflowPath(root, result.workflowPath), Changed: true,
		})
	}
	return &ActionResult{
		Schema: "one-cli/ci-enable/v1", Action: "enable", Provider: providerID,
		Projects: items, NextCommand: "git status",
	}, nil
}

func (s *Service) Sync(ctx context.Context, selector string) (*ActionResult, error) {
	activeWorkspace, err := execution.ResolveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	projects, err := s.selectProjects(activeWorkspace, selector, true)
	if err != nil {
		return nil, err
	}
	root := activeWorkspace.Root()
	enabled := make([]workspace.Project, 0, len(projects))
	for _, project := range projects {
		exists, err := workflowExists(s.workflowPath(root, project, pkgci.DefaultProviderID))
		if err != nil {
			return nil, err
		}
		if exists {
			enabled = append(enabled, project)
		}
	}
	if len(enabled) == 0 {
		return nil, ciNotEnabledError(selector)
	}
	items := make([]ActionProject, 0, len(enabled))
	for _, project := range enabled {
		result, err := s.syncProject(root, project, pkgci.DefaultProviderID)
		if err != nil {
			return nil, renderError(project.Name, pkgci.DefaultProviderID, err)
		}
		items = append(items, ActionProject{
			Name: project.Name, Status: "updated", Enabled: true,
			Provider:     pkgci.DefaultProviderID,
			WorkflowPath: relativeWorkflowPath(root, result.workflowPath), Changed: true,
		})
	}
	return &ActionResult{
		Schema: "one-cli/ci-sync/v1", Action: "sync", Provider: pkgci.DefaultProviderID,
		Projects: items, NextCommand: "git status",
	}, nil
}

type DisablePlan struct {
	EnabledCount int
	selector     string
	workspace    execution.Workspace
	projects     []workspace.Project
}

func (s *Service) PlanDisable(ctx context.Context, selector string) (DisablePlan, error) {
	activeWorkspace, err := execution.ResolveWorkspace(ctx)
	if err != nil {
		return DisablePlan{}, err
	}
	projects, err := s.selectProjects(activeWorkspace, selector, false)
	if err != nil {
		return DisablePlan{}, err
	}
	plan := DisablePlan{selector: selector, workspace: activeWorkspace, projects: projects}
	plan.EnabledCount, err = s.enabledCount(activeWorkspace.Root(), projects)
	if err != nil {
		return DisablePlan{}, err
	}
	return plan, nil
}

func (s *Service) Disable(plan DisablePlan, confirmed bool) (*ActionResult, error) {
	if plan.workspace.Manifest() == nil {
		return nil, cliErrors.New(cliErrors.ONE_CLI_ERROR, "CI disable plan is required")
	}
	if !confirmed {
		enabledCount, err := s.enabledCount(plan.workspace.Root(), plan.projects)
		if err != nil {
			return nil, err
		}
		if enabledCount > 0 {
			return nil, disableConfirmationError(plan.selector, enabledCount)
		}
	}
	root := plan.workspace.Root()
	items := make([]ActionProject, 0, len(plan.projects))
	changed := false
	for _, project := range plan.projects {
		path := s.workflowPath(root, project, pkgci.DefaultProviderID)
		removed, err := removeWorkflow(path)
		if err != nil {
			return nil, cliErrors.New(
				cliErrors.ONE_CLI_ERROR,
				fmt.Sprintf("remove CI workflow for %s: %v", project.Name, err),
			).WithContext(map[string]any{
				"project": project.Name, "workflow_path": relativeWorkflowPath(root, path),
			})
		}
		status := "unchanged"
		if removed {
			status = "removed"
			changed = true
		}
		items = append(items, ActionProject{
			Name: project.Name, Status: status, Enabled: false,
			WorkflowPath: relativeWorkflowPath(root, path), Changed: removed,
		})
	}
	nextCommand := ""
	if changed {
		nextCommand = "git status"
	}
	return &ActionResult{
		Schema: "one-cli/ci-disable/v1", Action: "disable", Provider: pkgci.DefaultProviderID,
		Projects: items, NextCommand: nextCommand,
	}, nil
}

func (s *Service) enabledCount(root string, projects []workspace.Project) (int, error) {
	count := 0
	for _, project := range projects {
		exists, err := workflowExists(s.workflowPath(root, project, pkgci.DefaultProviderID))
		if err != nil {
			return 0, err
		}
		if exists {
			count++
		}
	}
	return count, nil
}

func disableConfirmationError(selector string, enabledCount int) error {
	command := "one ci disable --yes"
	if selector != "" {
		command = "one ci disable " + selector + " --yes"
	}
	return cliErrors.New(
		cliErrors.CI_DISABLE_CONFIRMATION_REQUIRED,
		i18n.T("ci.error.confirmation_required"),
	).WithContext(map[string]any{
		"enabled_projects": enabledCount, "selector": selector,
	}).WithRemediation(output.Remediation{
		Action: "confirm-disable", Hint: i18n.T("ci.error.confirmation_hint"),
		Command: command, Destructive: true,
	})
}

func renderError(project, provider string, err error) error {
	return cliErrors.New(
		cliErrors.CI_RENDER_FAILED,
		fmt.Sprintf("render CI workflow for %s: %v", project, err),
	).WithContext(map[string]any{"project": project, "provider": provider})
}

func ciNotEnabledError(selector string) error {
	command := "one ci enable"
	message := i18n.T("ci.error.not_enabled")
	context := map[string]any{}
	if selector != "" {
		command += " " + selector
		message = i18n.Tf("ci.error.project_not_enabled", selector)
		context["project"] = selector
	}
	return cliErrors.New(cliErrors.CI_NOT_ENABLED, message).
		WithContext(context).
		WithRemediation(output.Remediation{
			Action: "enable-ci", Hint: i18n.T("ci.error.enable_hint"), Command: command,
		})
}
