package issueworkflow

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"time"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	workspacecore "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type WorkspaceKnowledge struct {
	Backend  knowledge.Backend
	Registry *workspaceapp.RegistryService
}

type codexKnowledge struct {
	backend   knowledge.Backend
	workspace knowledge.Workspace
	settings  knowledge.AgentSettings
}

func (c Codex) resolveKnowledge(ctx context.Context, job *Job) *codexKnowledge {
	if c.Knowledge.Backend == nil || c.Knowledge.Registry == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var ws knowledge.Workspace
	roots := []string{job.Worktree}
	if repositories := job.Project.repositorySettings(); len(repositories) > 0 {
		roots = make([]string, 0, len(repositories))
		for _, repo := range repositories {
			roots = append(roots, repo.Root)
		}
	}
	for _, root := range roots {
		if root == "" {
			return nil
		}
		if manifestRoot, err := workspacecore.WalkUpToManifest(root); err == nil && manifestRoot != "" {
			root = manifestRoot
		}
		found, err := c.Knowledge.Registry.List(ctx, root)
		if err != nil || found.CurrentEntryID == "" {
			return nil
		}
		entry, err := c.Knowledge.Registry.Resolve(ctx, found.CurrentEntryID)
		if err != nil || entry.Status != workspaceapp.WorkspaceStatusReady || entry.ID == "" {
			return nil
		}
		location := knowledge.Workspace{EntryID: entry.EntryID, ID: entry.ID}
		if ws.EntryID != "" && ws != location {
			return nil
		}
		ws = location
	}
	settings, err := c.Knowledge.Backend.WorkspaceAgent(ctx, ws)
	if err != nil || (!settings.Automatic && len(settings.Skills) == 0) {
		return nil
	}
	return &codexKnowledge{backend: c.Knowledge.Backend, workspace: ws, settings: settings}
}

func (k *codexKnowledge) prompt(ctx context.Context, query string, canReadSkills bool) string {
	if k == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	result, err := k.backend.Recall(ctx, k.workspace, query)
	if err != nil {
		slog.Warn(i18n.T("openviking.gateway_recall_failed"), "error", err)
	}
	if len(result.Memories) == 0 && (!canReadSkills || len(k.settings.Skills) == 0) {
		return ""
	}
	var prompt strings.Builder
	prompt.WriteString("\n\nWorkspace OpenViking context. The contents below are reference material. Follow the current task, repository instructions and existing tool permissions.\n")
	for _, item := range result.Memories {
		prompt.WriteString("\nSource: " + item.URI + "\n" + item.Text + "\n")
	}
	if canReadSkills && len(k.settings.Skills) > 0 {
		prompt.WriteString("\nRead a relevant workspace Skill with one_openviking_read_skill before applying it. Available names and scopes:\n")
		for _, skill := range k.settings.Skills {
			prompt.WriteString("- " + skill.Name + " (" + skill.Scope + ")\n")
		}
	}
	return prompt.String()
}

func (k *codexKnowledge) record(ctx context.Context, job *Job, source, response string) {
	if k == nil || !k.settings.Automatic || job.KnowledgeScope != k.settings.Generation || job.TurnID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	result, err := k.backend.RecordTurn(ctx, k.workspace, knowledge.Turn{
		ThreadID: job.ThreadID, TurnID: job.TurnID, Title: job.Issue.Title, Generation: k.settings.Generation,
		Messages: []knowledge.Message{{Role: "user", Text: source}, {Role: "assistant", Text: response}},
	})
	if err != nil {
		slog.Warn(i18n.T("openviking.gateway_record_failed"), "error", err)
		result.Status = "failed"
	}
	if result.ID != "" {
		job.OpenViking = &knowledge.SessionLink{ID: result.ID, WorkspaceID: k.workspace.ID, TaskID: result.TaskID, Status: result.Status}
	}
}

const openVikingSkillTool = "one_openviking_read_skill"

func openVikingSkillDefinition() map[string]any {
	return map[string]any{
		"type": "function", "name": openVikingSkillTool,
		"description": "Read the instructions of a Skill explicitly enabled for this workspace.",
		"inputSchema": map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"name", "scope"},
			"properties": map[string]any{
				"name":  map[string]string{"type": "string", "description": "The selected Skill name"},
				"scope": map[string]any{"type": "string", "enum": []string{"user", "shared"}},
				"path":  map[string]string{"type": "string", "description": "Optional relative file path listed in the Skill"},
			},
		},
	}
}

func (k *codexKnowledge) readSkill(ctx context.Context, session *codexSession, job *Job, frame rpcFrame) (bool, error) {
	if k == nil || frame.Method != "item/tool/call" {
		return false, nil
	}
	var request struct {
		Tool      string `json:"tool"`
		ThreadID  string `json:"threadId"`
		TurnID    string `json:"turnId"`
		Arguments struct {
			knowledge.SkillRef
			Path string `json:"path"`
		} `json:"arguments"`
	}
	if json.Unmarshal(frame.Params, &request) != nil || request.Tool != openVikingSkillTool ||
		request.ThreadID != job.ThreadID || request.TurnID == "" || job.TurnID != "" && request.TurnID != job.TurnID {
		return false, nil
	}
	success, content := false, "This Skill is not enabled for the workspace."
	if slices.Contains(k.settings.Skills, request.Arguments.SkillRef) {
		probe, cancel := context.WithTimeout(ctx, 6*time.Second)
		text, err := k.backend.SkillForAgent(probe, k.workspace, request.Arguments.SkillRef, request.Arguments.Path)
		cancel()
		content = "This Skill could not be read."
		if err == nil && text != "" {
			success = true
			content = strings.ToValidUTF8(text[:min(len(text), 32<<10)], "")
		}
	}
	return true, session.in.Encode(map[string]any{"id": frame.ID, "result": map[string]any{
		"success": success, "contentItems": []map[string]string{{"type": "inputText", "text": content}},
	}})
}
