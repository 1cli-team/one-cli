package openviking

import (
	"context"
	"errors"
	"os"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func workspaceState(ctx context.Context, workspaceID string) (connection, workspaceBinding, error) {
	if workspaceID == "" {
		return connection{}, workspaceBinding{}, i18n.Errorf("openviking.workspace")
	}
	dir, err := configDirectory()
	if err != nil {
		return connection{}, workspaceBinding{}, err
	}
	saved, err := loadConnection(ctx, dir)
	if err != nil {
		return connection{}, workspaceBinding{}, err
	}
	binding, err := readConfig[workspaceBinding](bindingPath(dir, workspaceID))
	return saved, binding, err
}

func (binding workspaceBinding) matches(saved connection) bool {
	if binding.Scope != "" {
		return binding.Scope == saved.Scope
	}
	return binding.URL != "" && saved.sameCredential(connection{URL: binding.URL, APIKey: binding.APIKey})
}

func (*Plugin) Status(ctx context.Context, workspaceID string) (knowledge.Status, error) {
	saved, binding, err := workspaceState(ctx, workspaceID)
	if err != nil {
		return knowledge.Status{}, err
	}
	status := knowledge.Status{Configured: saved.URL != ""}
	if binding.URI != "" {
		status.Binding = &knowledge.Binding{URI: binding.URI}
		status.NeedsRebind = status.Configured && !binding.matches(saved)
	}
	return status, nil
}

func (*Plugin) Bind(ctx context.Context, workspaceID, uri string) (knowledge.Binding, error) {
	if workspaceID == "" {
		return knowledge.Binding{}, i18n.Errorf("openviking.workspace")
	}
	dir, err := configDirectory()
	if err != nil {
		return knowledge.Binding{}, err
	}
	if _, err := loadConnection(ctx, dir); err != nil {
		return knowledge.Binding{}, err
	}
	unlock, err := lockSettings(ctx, dir)
	if err != nil {
		return knowledge.Binding{}, err
	}
	defer unlock()
	saved, err := readConfig[connection](connectionPath(dir))
	if err != nil {
		return knowledge.Binding{}, err
	}
	if err := requireConnection(saved); err != nil {
		return knowledge.Binding{}, err
	}
	directory, err := resolveDirectory(ctx, saved, uri, true)
	if err != nil {
		return knowledge.Binding{}, err
	}
	record := workspaceBinding{URI: directory.value, Scope: saved.Scope}
	previous, err := readConfig[workspaceBinding](bindingPath(dir, workspaceID))
	if err != nil {
		return knowledge.Binding{}, err
	}
	if previous.matches(saved) {
		record.Agent = previous.Agent
	}
	if err := writeConfig(bindingPath(dir, workspaceID), record); err != nil {
		return knowledge.Binding{}, err
	}
	return knowledge.Binding{URI: record.URI}, nil
}

func (*Plugin) Unbind(ctx context.Context, workspaceID string) error {
	if workspaceID == "" {
		return i18n.Errorf("openviking.workspace")
	}
	dir, err := configDirectory()
	if err != nil {
		return err
	}
	unlock, err := lockSettings(ctx, dir)
	if err != nil {
		return err
	}
	defer unlock()
	if err := os.Remove(bindingPath(dir, workspaceID)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return fsutil.SyncDir(dir)
}

func (*Plugin) bindingFor(ctx context.Context, workspaceID string) (connection, resourceURI, error) {
	saved, binding, err := workspaceState(ctx, workspaceID)
	if err != nil {
		return connection{}, resourceURI{}, err
	}
	if err := requireConnection(saved); err != nil {
		return connection{}, resourceURI{}, err
	}
	if binding.URI == "" {
		return connection{}, resourceURI{}, cliErrors.New(cliErrors.OPENVIKING_UNBOUND, i18n.T("openviking.not_bound"))
	}
	if !binding.matches(saved) {
		return connection{}, resourceURI{}, cliErrors.New(cliErrors.OPENVIKING_UNBOUND, i18n.T("openviking.rebind_required"))
	}
	root, ok := parseResourceURI(binding.URI)
	if !ok {
		return connection{}, resourceURI{}, i18n.Errorf("openviking.storage")
	}
	return saved, root, nil
}
