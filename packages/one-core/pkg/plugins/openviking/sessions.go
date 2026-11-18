package openviking

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	ov "github.com/volcengine/OpenViking/sdk/go"
)

var archivePattern = regexp.MustCompile("^archive_[0-9]{3,9}$")

type sessionNote struct {
	knowledge.Session
	Turns map[string]string `json:"turns,omitempty"`
	Tasks map[string]string `json:"tasks,omitempty"`
}

func isMissing(err error) bool {
	var problem *ov.Error
	return errors.As(err, &problem) && (problem.StatusCode == http.StatusNotFound || problem.Code == "NOT_FOUND")
}

func sessionPrefix(workspace knowledge.Workspace) string { return workspacePeer(workspace) + "-" }

func validSession(workspace knowledge.Workspace, id string) bool {
	return taskIDPattern.MatchString(id) && strings.HasPrefix(id, sessionPrefix(workspace))
}

func sessionFile(dir string, saved connection, id string) string {
	return filepath.Join(dir, "sessions", digest(saved.Scope), id+".json")
}

func (p *Plugin) Sessions(ctx context.Context, workspace knowledge.Workspace) ([]knowledge.Session, error) {
	saved, _, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return nil, err
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return nil, err
	}
	values, err := client.ListSessions(ctx)
	if err != nil {
		return nil, sdkError(err)
	}
	dir, err := configDirectory()
	if err != nil {
		return nil, err
	}
	result := make([]knowledge.Session, 0)
	for _, value := range values {
		row, _ := value.(map[string]any)
		id, _ := row["session_id"].(string)
		if !validSession(workspace, id) {
			continue
		}
		note, err := readConfig[sessionNote](sessionFile(dir, saved, id))
		if err != nil {
			return nil, err
		}
		note.ID = id
		result = append(result, note.Session)
	}
	return result, nil
}

func (p *Plugin) Session(ctx context.Context, workspace knowledge.Workspace, id, archive string) (knowledge.Session, error) {
	if !validSession(workspace, id) || archive != "" && !archivePattern.MatchString(archive) {
		return knowledge.Session{}, outsideScope()
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	saved, _, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return knowledge.Session{}, err
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return knowledge.Session{}, err
	}
	details, err := client.GetSession(ctx, id, nil)
	if err != nil {
		return knowledge.Session{}, sdkError(err)
	}
	dir, err := configDirectory()
	if err != nil {
		return knowledge.Session{}, err
	}
	note, err := readConfig[sessionNote](sessionFile(dir, saved, id))
	if err != nil {
		return knowledge.Session{}, err
	}
	result := note.Session
	result.ID = id
	result.Archives = max(result.Archives, archiveCount(details))
	if archive == "" && result.Archives > 0 {
		archive = fmt.Sprintf("archive_%03d", result.Archives)
	}
	var content map[string]any
	if archive != "" {
		content, err = client.GetSessionArchive(ctx, id, archive)
		result.Archive = archive
		if status, ok := content["status"].(string); ok {
			result.Status = status
		}
	} else {
		content, err = client.GetSessionContext(ctx, id, 16000)
	}
	if err != nil {
		return knowledge.Session{}, sdkError(err)
	}
	result.Messages, err = sessionMessages(content)
	if err != nil {
		return knowledge.Session{}, err
	}
	if archive != "" {
		result.TaskID = note.Tasks[archive]
		if result.TaskID == "" && archive == note.Archive {
			result.TaskID = note.TaskID
		}
		addExtractionResult(ctx, client, &result)
	}
	return result, nil
}

func addExtractionResult(ctx context.Context, client *ov.Client, session *knowledge.Session) {
	if !taskIDPattern.MatchString(session.TaskID) {
		return
	}
	task, err := client.GetTask(ctx, session.TaskID)
	if err != nil || task["task_type"] != "session_commit" {
		return
	}
	result, ok := task["result"].(map[string]any)
	if !ok || result["session_id"] != session.ID {
		return
	}
	if counts, ok := result["memories_extracted"].(map[string]any); ok {
		total := 0
		for _, value := range counts {
			if count, ok := value.(float64); ok && count >= 0 && count <= 999999 {
				total += int(count)
			}
		}
		session.MemoryCount = &total
	}
}

func archiveCount(data map[string]any) int {
	if value, ok := data["commit_count"].(float64); ok && value >= 0 && value <= 999999999 {
		return int(value)
	}
	if stats, ok := data["stats"].(map[string]any); ok {
		if value, ok := stats["commit_count"].(float64); ok && value >= 0 && value <= 999999999 {
			return int(value)
		}
	}
	return 0
}

func sessionMessages(data map[string]any) ([]knowledge.Message, error) {
	if data["messages"] == nil {
		return nil, nil
	}
	values, err := decodeResult[[]struct {
		Role    string                        `json:"role"`
		Content string                        `json:"content"`
		Parts   []struct{ Type, Text string } `json:"parts"`
	}](data["messages"])
	if err != nil {
		return nil, err
	}
	messages := make([]knowledge.Message, 0, len(values))
	for _, message := range values {
		text := message.Content
		for _, part := range message.Parts {
			if part.Type == "text" {
				if text != "" {
					text += "\n"
				}
				text += part.Text
			}
		}
		if text != "" {
			messages = append(messages, knowledge.Message{Role: message.Role, Text: text})
		}
	}
	return messages, nil
}

func (p *Plugin) Remember(ctx context.Context, workspace knowledge.Workspace, text string) (knowledge.Session, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 32<<10 {
		return knowledge.Session{}, agentInput("openviking.invalid_memory")
	}
	saved, _, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return knowledge.Session{}, err
	}
	id := sessionPrefix(workspace) + strings.ToLower(rand.Text())
	title, _, _ := strings.Cut(text, "\n")
	return appendTurn(ctx, saved, workspace, id, knowledge.Turn{
		TurnID: id, Title: title, Messages: []knowledge.Message{{Role: "user", Text: text}},
	})
}

func (p *Plugin) RecordTurn(ctx context.Context, workspace knowledge.Workspace, turn knowledge.Turn) (knowledge.Session, error) {
	saved, binding, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return knowledge.Session{}, err
	}
	if !binding.Agent.Automatic || turn.Generation == "" || turn.Generation != binding.Agent.Generation || turn.ThreadID == "" || turn.TurnID == "" {
		return knowledge.Session{}, nil
	}
	id := sessionPrefix(workspace) + digest(turn.ThreadID)[:32]
	return appendTurn(ctx, saved, workspace, id, turn)
}

func appendTurn(ctx context.Context, saved connection, workspace knowledge.Workspace, id string, turn knowledge.Turn) (knowledge.Session, error) {
	peer := workspacePeer(workspace)
	turnID := digest(turn.TurnID)[:32]
	var messages []ov.Message
	for _, entry := range turn.Messages {
		if text := strings.TrimSpace(entry.Text); text != "" && (entry.Role == "user" || entry.Role == "assistant") {
			bounded := strings.ToValidUTF8(text[:min(len(text), 32<<10)], "")
			messages = append(messages, ov.Message{Role: entry.Role, Content: &bounded, PeerID: peer, TurnID: turnID})
		}
	}
	if len(messages) == 0 {
		return knowledge.Session{}, nil
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return knowledge.Session{}, err
	}
	dir, err := configDirectory()
	if err != nil {
		return knowledge.Session{}, err
	}
	file := sessionFile(dir, saved, id)
	unlock, err := fsutil.WorkspaceLock(ctx, file, "openviking-session")
	if err != nil {
		return knowledge.Session{}, err
	}
	defer unlock()
	note, err := readConfig[sessionNote](file)
	if err != nil {
		return knowledge.Session{}, err
	}
	if note.ID == "" {
		note = sessionNote{Session: knowledge.Session{ID: id, ThreadID: turn.ThreadID,
			Title: strings.ToValidUTF8(turn.Title[:min(len(turn.Title), 256)], "")}, Turns: map[string]string{}}
	}
	switch note.Turns[turnID] {
	case "committed":
		return note.Session, nil
	case "sending":
		// Losing the response after an append leaves delivery uncertain. Do
		// not replay an append that could duplicate archived messages.
		return note.Session, agentInput("openviking.session_uncertain")
	}
	if note.Turns[turnID] != "sent" {
		if len(note.Turns) == 0 {
			if err := ensureSession(ctx, client, id); err != nil {
				return knowledge.Session{}, err
			}
		}
		note.Turns[turnID] = "sending"
		note.Status = "pending"
		if err := writeConfig(file, note); err != nil {
			return knowledge.Session{}, err
		}
		if _, err := client.BatchAddMessages(ctx, id, messages, nil); err != nil {
			return note.Session, sdkError(err)
		}
		note.Turns[turnID] = "sent"
		if err := writeConfig(file, note); err != nil {
			return note.Session, err
		}
	}
	commit, err := client.CommitSession(ctx, id, &ov.CommitSessionOptions{KeepRecentCount: new(0)})
	if err != nil {
		return note.Session, sdkError(err)
	}
	archiveURI, _ := commit["archive_uri"].(string)
	taskID, _ := commit["task_id"].(string)
	if archive := filepath.Base(archiveURI); archivePattern.MatchString(archive) {
		note.Archive = archive
		note.Archives++
		if note.Tasks == nil {
			note.Tasks = make(map[string]string)
		}
		note.Tasks[archive] = taskID
	}
	note.Status = "pending"
	if taskID == "" {
		note.Status = "skipped"
	}
	note.TaskID = taskID
	note.Turns[turnID] = "committed"
	return note.Session, writeConfig(file, note)
}

func ensureSession(ctx context.Context, client *ov.Client, id string) error {
	_, err := client.GetSession(ctx, id, nil)
	if err == nil {
		return nil
	}
	if !isMissing(err) {
		return sdkError(err)
	}
	_, err = client.CreateSession(ctx, &ov.CreateSessionOptions{SessionID: id, DisableAutoCommit: true,
		MemoryPolicy: map[string]any{
			"self": map[string]bool{"enabled": false},
			"peer": map[string]bool{"enabled": true},
		},
	})
	return sdkError(err)
}
