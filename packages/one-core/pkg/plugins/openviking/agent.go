package openviking

import (
	"context"
	"crypto/rand"
	"net/url"
	"slices"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	ov "github.com/volcengine/OpenViking/sdk/go"
)

func agentInput(key string) error {
	return cliErrors.New(cliErrors.OPENVIKING_INVALID_INPUT, i18n.T(key))
}

func (p *Plugin) agentConnection(ctx context.Context, workspace knowledge.Workspace) (connection, workspaceBinding, error) {
	if workspace.ID == "" {
		return connection{}, workspaceBinding{}, agentInput("openviking.workspace")
	}
	saved, binding, err := workspaceState(ctx, workspace.EntryID)
	if err != nil {
		return connection{}, workspaceBinding{}, err
	}
	if err := requireConnection(saved); err != nil {
		return connection{}, workspaceBinding{}, err
	}
	if binding.URI == "" {
		return connection{}, workspaceBinding{}, cliErrors.New(cliErrors.OPENVIKING_UNBOUND, i18n.T("openviking.not_bound"))
	}
	if !binding.matches(saved) {
		return connection{}, workspaceBinding{}, cliErrors.New(cliErrors.OPENVIKING_UNBOUND, i18n.T("openviking.rebind_required"))
	}
	return saved, binding, nil
}

func (p *Plugin) WorkspaceAgent(ctx context.Context, workspace knowledge.Workspace) (knowledge.AgentSettings, error) {
	_, binding, err := p.agentConnection(ctx, workspace)
	return binding.Agent, err
}

func (p *Plugin) SetWorkspaceAgent(ctx context.Context, workspace knowledge.Workspace, input knowledge.AgentSettingsUpdate) (knowledge.AgentSettings, error) {
	if input.Skill != nil && !validSkillRef(*input.Skill) {
		return knowledge.AgentSettings{}, agentInput("openviking.invalid_skill")
	}
	dir, err := configDirectory()
	if err != nil {
		return knowledge.AgentSettings{}, err
	}
	// Resolve any legacy binding before taking the settings lock.
	if _, err := loadConnection(ctx, dir); err != nil {
		return knowledge.AgentSettings{}, err
	}
	unlock, err := lockSettings(ctx, dir)
	if err != nil {
		return knowledge.AgentSettings{}, err
	}
	defer unlock()
	saved, err := readConfig[connection](connectionPath(dir))
	if err != nil {
		return knowledge.AgentSettings{}, err
	}
	if err := requireConnection(saved); err != nil {
		return knowledge.AgentSettings{}, err
	}
	if workspace.ID == "" || workspace.EntryID == "" {
		return knowledge.AgentSettings{}, agentInput("openviking.workspace")
	}
	file := bindingPath(dir, workspace.EntryID)
	binding, err := readConfig[workspaceBinding](file)
	if err != nil {
		return knowledge.AgentSettings{}, err
	}
	if binding.URI == "" || !binding.matches(saved) {
		return knowledge.AgentSettings{}, cliErrors.New(cliErrors.OPENVIKING_UNBOUND, i18n.T("openviking.rebind_required"))
	}
	if input.Automatic != nil {
		if *input.Automatic && !binding.Agent.Automatic {
			binding.Agent.Generation = rand.Text()
		}
		binding.Agent.Automatic = *input.Automatic
	}
	if input.Skill != nil {
		index := slices.Index(binding.Agent.Skills, *input.Skill)
		switch {
		case input.Enabled && index < 0:
			if len(binding.Agent.Skills) >= 32 {
				return knowledge.AgentSettings{}, agentInput("openviking.skill_limit")
			}
			binding.Agent.Skills = append(binding.Agent.Skills, *input.Skill)
		case !input.Enabled && index >= 0:
			binding.Agent.Skills = slices.Delete(binding.Agent.Skills, index, index+1)
		}
	}
	return binding.Agent, writeConfig(file, binding)
}

func workspacePeer(workspace knowledge.Workspace) string {
	return "one-" + digest(workspace.ID)[:24]
}

func peerMemoryRoot(workspace knowledge.Workspace) string {
	return "viking://~/peers/" + workspacePeer(workspace) + "/memories"
}

func inPeerMemory(saved connection, workspace knowledge.Workspace, value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "viking" || u.User != nil || strings.ContainsAny(value, "?#%\\") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(strings.TrimSuffix(u.Path, "/"), "/"), "/")
	for _, part := range parts {
		if part == "." || part == ".." || part == "" {
			return false
		}
	}
	switch u.Host {
	case "~":
	case "user":
		if len(parts) < 4 || (saved.User != "" && parts[0] != saved.User) {
			return false
		}
		parts = parts[1:]
	default:
		return false
	}
	return len(parts) > 3 && parts[0] == "peers" && parts[1] == workspacePeer(workspace) && parts[2] == "memories"
}

// Accept canonical result URIs, but always resolve the path under the
// authenticated user's home namespace when reading memory.
func ownMemoryURI(workspace knowledge.Workspace, value string) string {
	_, relative, _ := strings.Cut(value, "/peers/"+workspacePeer(workspace)+"/memories")
	return peerMemoryRoot(workspace) + relative
}

func memoryMatches(ctx context.Context, client *ov.Client, saved connection, workspace knowledge.Workspace, query string, limit int) ([]knowledge.Match, error) {
	root := peerMemoryRoot(workspace)
	matches := make([]knowledge.Match, 0)
	if query == "" {
		items, err := client.List(ctx, root, &ov.ListOptions{Recursive: true, Limit: limit})
		if isMissing(err) {
			return matches, nil
		}
		if err != nil {
			return nil, sdkError(err)
		}
		for _, value := range items {
			item, _ := value.(map[string]any)
			uri, _ := item["uri"].(string)
			isDir, _ := item["isDir"].(bool)
			if !isDir && inPeerMemory(saved, workspace, uri) {
				matches = append(matches, knowledge.Match{URI: uri})
			}
		}
		return matches, nil
	}
	result, err := client.Find(ctx, query, &ov.FindOptions{TargetURI: root, Limit: limit, ContextType: "memory"})
	if isMissing(err) {
		return matches, nil
	}
	if err != nil {
		return nil, sdkError(err)
	}
	for _, item := range result.Memories {
		if inPeerMemory(saved, workspace, item.URI) {
			matches = append(matches, knowledge.Match{URI: item.URI, Abstract: item.Abstract, Score: item.Score})
		}
	}
	return matches, nil
}

func (p *Plugin) Memories(ctx context.Context, workspace knowledge.Workspace, query string) ([]knowledge.Match, error) {
	saved, _, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return nil, err
	}
	if len(query) > 8192 {
		return nil, agentInput("openviking.invalid_query")
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return nil, err
	}
	return memoryMatches(ctx, client, saved, workspace, strings.TrimSpace(query), 40)
}

func (p *Plugin) Memory(ctx context.Context, workspace knowledge.Workspace, uri string) (knowledge.Content, error) {
	saved, _, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return knowledge.Content{}, err
	}
	if !inPeerMemory(saved, workspace, uri) {
		return knowledge.Content{}, outsideScope()
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return knowledge.Content{}, err
	}
	text, err := client.Read(ctx, ownMemoryURI(workspace, uri), 0, -1)
	return knowledge.Content{URI: uri, Text: text}, sdkError(err)
}

func (p *Plugin) Recall(ctx context.Context, workspace knowledge.Workspace, query string) (knowledge.AgentContext, error) {
	saved, binding, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return knowledge.AgentContext{}, err
	}
	result := knowledge.AgentContext{Skills: binding.Agent.Skills}
	if !binding.Agent.Automatic || strings.TrimSpace(query) == "" {
		return result, nil
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return result, err
	}
	query = strings.ToValidUTF8(query[:min(len(query), 8192)], "")
	matches, err := memoryMatches(ctx, client, saved, workspace, query, 4)
	if err != nil {
		return result, err
	}
	for _, match := range matches {
		text, err := client.Read(ctx, ownMemoryURI(workspace, match.URI), 0, -1)
		if err == nil {
			text = strings.ToValidUTF8(text[:min(len(text), 1800)], "")
			result.Memories = append(result.Memories, knowledge.Content{URI: match.URI, Text: text})
		}
	}
	return result, nil
}
