package openviking

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
)

func TestWorkspaceSessionsContinueWithoutReplayingATurn(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	ctx := t.Context()
	workspace := knowledge.Workspace{EntryID: "one-entry", ID: "workspace-one"}
	other := knowledge.Workspace{EntryID: "other-entry", ID: "workspace-two"}
	sessionID := sessionPrefix(workspace) + digest("codex-thread")[:32]
	root := peerMemoryRoot(workspace)
	memoryURI := "viking://user/alice/peers/" + workspacePeer(workspace) + "/memories/events/task.md"
	var archives, appends atomic.Int32

	mux := http.NewServeMux()
	reply := func(pattern string, result func(*http.Request) any) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "result": result(r)})
		})
	}
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"account_id": "account", "user_id": "alice"})
	})
	reply("GET /api/v1/fs/ls", func(*http.Request) any { return []any{} })
	reply("GET /api/v1/fs/stat", func(r *http.Request) any {
		return resourceStat{URI: r.URL.Query().Get("uri"), IsDir: true}
	})
	mux.HandleFunc("GET /api/v1/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != sessionID || archives.Load() == 0 {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "result": map[string]any{"commit_count": archives.Load()}})
	})
	reply("POST /api/v1/sessions", func(r *http.Request) any {
		var fields struct {
			ID     string                            `json:"session_id"`
			Policy map[string]struct{ Enabled bool } `json:"memory_policy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&fields); err != nil || fields.ID != sessionID || fields.Policy["self"].Enabled || !fields.Policy["peer"].Enabled {
			t.Errorf("session must write only to its peer: %+v, %v", fields, err)
		}
		return map[string]string{"session_id": sessionID}
	})
	reply("POST /api/v1/sessions/{id}/messages/batch", func(r *http.Request) any {
		var payload struct {
			Messages []struct {
				Peer string `json:"peer_id"`
				Turn string `json:"turn_id"`
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || r.PathValue("id") != sessionID || len(payload.Messages) != 2 {
			t.Errorf("unexpected messages: %+v, %v", payload, err)
		}
		for _, message := range payload.Messages {
			if message.Peer != workspacePeer(workspace) || message.Turn == "" {
				t.Errorf("unscoped message: %+v", message)
			}
		}
		appends.Add(1)
		return map[string]int{"saved_count": len(payload.Messages)}
	})
	reply("POST /api/v1/sessions/{id}/commit", func(r *http.Request) any {
		archive := archives.Add(1)
		return map[string]any{"task_id": fmt.Sprintf("task-%d", archive), "status": "accepted",
			"archive_uri": fmt.Sprintf("viking://user/alice/sessions/%s/history/archive_%03d", r.PathValue("id"), archive)}
	})
	reply("GET /api/v1/sessions/{id}/archives/{archive}", func(r *http.Request) any {
		return map[string]any{"status": "completed", "messages": []any{
			map[string]any{"role": "assistant", "parts": []any{map[string]any{"type": "text", "text": r.PathValue("archive")}}},
		}}
	})
	reply("GET /api/v1/sessions", func(*http.Request) any {
		return []any{map[string]string{"session_id": sessionID}, map[string]string{"session_id": "external-session"}}
	})
	reply("POST /api/v1/search/find", func(r *http.Request) any {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["target_uri"] != root {
			t.Errorf("unscoped memory search: %+v, %v", body, err)
		}
		return map[string]any{"memories": []any{
			map[string]string{"uri": memoryURI, "abstract": "same workspace"},
			map[string]string{"uri": "viking://user/alice/peers/" + workspacePeer(other) + "/memories/secret.md"},
		}}
	})
	reply("GET /api/v1/content/read", func(r *http.Request) any {
		if got := r.URL.Query().Get("uri"); got != root+"/events/task.md" {
			t.Errorf("escaped memory scope: %s", got)
		}
		return "deployment plan"
	})
	reply("GET /api/v1/skills/{name}", func(r *http.Request) any {
		return map[string]any{"name": r.PathValue("name"), "root_uri": "viking://user/alice/skills/review",
			"skill_md_uri": "viking://user/alice/skills/review/SKILL.md", "content": "# Review"}
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	p := New()
	if _, err := p.Configure(ctx, knowledge.ConnectionInput{URL: server.URL}); err != nil {
		t.Fatal(err)
	}
	for _, ws := range []knowledge.Workspace{workspace, other} {
		if _, err := p.Bind(ctx, ws.EntryID, "viking://resources/team"); err != nil {
			t.Fatal(err)
		}
	}
	ref := knowledge.SkillRef{Name: "review", Scope: "user"}
	settings, err := p.SetWorkspaceAgent(ctx, workspace, knowledge.AgentSettingsUpdate{
		Automatic: new(true), Skill: &ref, Enabled: true,
	})
	if err != nil || settings.Generation == "" {
		t.Fatalf("unexpected settings: %+v, %v", settings, err)
	}
	turn := knowledge.Turn{ThreadID: "codex-thread", TurnID: "first", Generation: settings.Generation,
		Title: "Fix gateway", Messages: []knowledge.Message{{Role: "user", Text: "Request"}, {Role: "assistant", Text: "Answer"}}}
	first, err := p.RecordTurn(ctx, workspace, turn)
	if err != nil || first.ID != sessionID || first.Archives != 1 {
		t.Fatalf("first turn: %+v, %v", first, err)
	}
	if _, err := p.RecordTurn(ctx, workspace, turn); err != nil {
		t.Fatal(err)
	}
	turn.TurnID = "second"
	second, err := p.RecordTurn(ctx, workspace, turn)
	if err != nil || second.ID != first.ID || second.Archives != 2 || appends.Load() != 2 {
		t.Fatalf("second turn: %+v, %v, appends: %d", second, err, appends.Load())
	}
	last, err := p.Session(ctx, workspace, first.ID, "")
	if err != nil || last.Status != "completed" || len(last.Messages) != 1 || last.Messages[0].Text != "archive_002" {
		t.Fatalf("latest archive: %+v, %v", last, err)
	}
	if items, err := p.Sessions(ctx, other); err != nil || len(items) != 0 {
		t.Fatalf("other workspace saw sessions: %+v, %v", items, err)
	}
	if _, err := p.Session(ctx, other, first.ID, ""); err == nil {
		t.Fatal("other workspace could read the session")
	}
	agent, err := p.Recall(ctx, workspace, "deploy")
	if err != nil || len(agent.Memories) != 1 || agent.Memories[0].URI != memoryURI || len(agent.Skills) != 1 {
		t.Fatalf("unexpected scoped context: %+v, %v", agent, err)
	}
	skill, err := p.SkillForAgent(ctx, workspace, ref, "")
	if err != nil || !strings.Contains(skill, "# Review") {
		t.Fatalf("skill read: %+v, %v", skill, err)
	}
	if _, err := p.SkillForAgent(ctx, other, ref, ""); err == nil {
		t.Fatal("an unselected Skill was readable by the Agent")
	}
}
