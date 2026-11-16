package infisical

import (
	"context"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/environment"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	remote "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/infisical/internal/remote"
	session "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/infisical/session"
)

// BindWorkspace is the explicit boundary for creating or selecting Infisical
// storage. Variable writes only use an existing binding.
func (p *Plugin) BindWorkspace(ctx context.Context, input environment.BindWorkspaceInput) (*environment.WorkspaceBindingResult, error) {
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
	config, err := remote.LoadWorkspaceConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" && config != nil && strings.TrimSpace(config.ProjectID) != "" {
		return &environment.WorkspaceBindingResult{
			Schema: "one-cli/env-bind/v1", BindingResult: environment.BindingResult{ProjectID: config.ProjectID, ProjectName: config.ProjectName},
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
	initInput := remote.InitInput{ProjectID: projectID, Session: current, BeforeWrite: checkRevision}
	if projectID != "" {
		project, err := remote.ProjectWithSession(ctx, current, projectID)
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
	result, err := remote.Init(ctx, projectRoot, initInput)
	if err != nil {
		return nil, err
	}
	binding := &environment.BindingResult{ProjectID: result.ProjectID, ProjectName: result.ProjectName, Created: result.Created}
	requestedName := ""
	if identity := activeWorkspace.Manifest().Workspace; identity != nil {
		requestedName = identity.Name
	}
	if result.Created && requestedName != result.ProjectName {
		binding.RequestedName = requestedName
	}
	return &environment.WorkspaceBindingResult{
		Schema: "one-cli/env-bind/v1", BindingResult: *binding,
		Environments: result.Environments, WrittenTo: result.WrittenTo,
	}, nil
}
