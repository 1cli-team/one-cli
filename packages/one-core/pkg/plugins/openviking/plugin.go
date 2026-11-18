package openviking

import (
	"context"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins"
	ov "github.com/volcengine/OpenViking/sdk/go"
)

type Plugin struct{}

func New() *Plugin         { return &Plugin{} }
func (*Plugin) ID() string { return "openviking" }
func (p *Plugin) Contributions() plugins.Contributions {
	return plugins.Contributions{Knowledge: p}
}

func (p *Plugin) Find(ctx context.Context, workspaceID, query string) ([]knowledge.Match, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 8192 {
		return nil, cliErrors.New(cliErrors.OPENVIKING_INVALID_INPUT, i18n.T("openviking.invalid_query"))
	}
	saved, root, err := p.bindingFor(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return nil, err
	}
	found, err := client.Find(ctx, query, &ov.FindOptions{TargetURI: root.value, Limit: 20})
	if err != nil {
		return nil, sdkError(err)
	}
	matches := make([]knowledge.Match, 0, len(found.Resources))
	for _, result := range found.Resources {
		uri, ok := parseResourceURI(result.URI)
		if ok && root.contains(uri) {
			matches = append(matches, knowledge.Match{URI: result.URI, Abstract: result.Abstract, Score: result.Score})
		}
	}
	return matches, nil
}

func (p *Plugin) Read(ctx context.Context, workspaceID, value string) (knowledge.Content, error) {
	saved, root, err := p.bindingFor(ctx, workspaceID)
	if err != nil {
		return knowledge.Content{}, err
	}
	uri, ok := parseResourceURI(value)
	if !ok || !root.contains(uri) {
		return knowledge.Content{}, outsideScope()
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client, err := sdk(saved, 0)
	if err != nil {
		return knowledge.Content{}, err
	}
	info, err := stat(ctx, client, uri.value)
	if err != nil {
		return knowledge.Content{}, sdkError(err)
	}
	canonical, ok := parseResourceURI(info.URI)
	if !ok || !root.contains(canonical) {
		return knowledge.Content{}, outsideScope()
	}
	result := knowledge.Content{URI: canonical.value, IsDirectory: info.IsDir}
	if info.IsDir {
		result.Text, err = client.Overview(ctx, canonical.value)
	} else {
		result.Text, err = client.Read(ctx, canonical.value, 0, -1)
	}
	return result, sdkError(err)
}
