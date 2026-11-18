package issueworkflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	workspacecore "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
)

type knowledgeRegistry struct{ value workspacecore.Registry }

func (r *knowledgeRegistry) Load(context.Context) (workspacecore.Registry, error) {
	return r.value, nil
}

func (r *knowledgeRegistry) Update(_ context.Context, update func(*workspacecore.Registry) error) error {
	return update(&r.value)
}

type knowledgeRecorder struct {
	knowledge.Backend
	turns []knowledge.Turn
	skill []knowledge.SkillRef
}

func (*knowledgeRecorder) WorkspaceAgent(context.Context, knowledge.Workspace) (knowledge.AgentSettings, error) {
	return knowledge.AgentSettings{Automatic: true, Generation: "enabled-generation", Skills: []knowledge.SkillRef{{Name: "review", Scope: "user"}}}, nil
}

func (*knowledgeRecorder) Recall(_ context.Context, _ knowledge.Workspace, query string) (knowledge.AgentContext, error) {
	if query != "" {
		return knowledge.AgentContext{Memories: []knowledge.Content{{URI: "viking://~/example", Text: "Recall the deployment constraint."}}}, nil
	}
	return knowledge.AgentContext{}, nil
}

func (r *knowledgeRecorder) SkillForAgent(_ context.Context, _ knowledge.Workspace, skill knowledge.SkillRef, _ string) (string, error) {
	r.skill = append(r.skill, skill)
	return "Review the checkout.", nil
}

func (r *knowledgeRecorder) RecordTurn(_ context.Context, _ knowledge.Workspace, turn knowledge.Turn) (knowledge.Session, error) {
	r.turns = append(r.turns, turn)
	return knowledge.Session{ID: "same-session", TaskID: "extraction", Status: "pending"}, nil
}

func TestCodexRecordsContinuedTurnsAndOnlyReadsSelectedSkills(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "one.manifest.toml"), []byte("version = 2\n[workspace]\nid = \"fixture\"\nname = \"Fixture\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := workspaceapp.NewRegistryService(&knowledgeRegistry{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Observe(ctx, root, "test"); err != nil {
		t.Fatal(err)
	}
	var requests bytes.Buffer
	init := `{"id":1,"result":{}}
{"id":2,"result":{"exitCode":0}}
{"id":3,"result":{"thread":{"id":"thread-a"}}}
`
	replies := init + `{"id":4,"result":{"turn":{"id":"turn-a"}}}
{"id":"blocked","method":"item/tool/call","params":{"threadId":"thread-a","turnId":"turn-a","tool":"one_openviking_read_skill","arguments":{"scope":"shared","name":"review"}}}
{"id":"allowed","method":"item/tool/call","params":{"threadId":"thread-a","turnId":"turn-a","tool":"one_openviking_read_skill","arguments":{"scope":"user","name":"review"}}}
{"method":"item/completed","params":{"threadId":"thread-a","turnId":"turn-a","item":{"type":"agentMessage","text":"First answer","phase":"final_answer"}}}
{"method":"turn/completed","params":{"threadId":"thread-a","turn":{"id":"turn-a","status":"completed"}}}
`
	recorder := &knowledgeRecorder{}
	codex := Codex{
		Executable: os.Args[0], StateDir: t.TempDir(), Knowledge: WorkspaceKnowledge{Backend: recorder, Registry: registry},
		Launch: func(context.Context, api.CodexStart) (api.CodexStream, error) {
			return api.CodexStream{Input: &requests, Output: strings.NewReader(replies), Close: func() {}}, nil
		},
	}
	job := Job{Worktree: root, Project: Project{Root: root}, Issue: issuetracking.Issue{Title: "Fix issue", Description: "Task description."}}
	checkpoint := func(Job) error { return nil }
	if err := codex.Implement(ctx, &job, root, checkpoint); err != nil {
		t.Fatal(err)
	}
	if len(recorder.skill) != 1 || recorder.skill[0].Scope != "user" || !strings.Contains(requests.String(), "Recall the deployment constraint.") {
		t.Fatalf("did not restrict skill access or recall workspace memory: %v", recorder.skill)
	}
	if !strings.Contains(requests.String(), "Review the checkout.") || !strings.Contains(requests.String(), "not enabled") {
		t.Fatal("unexpected dynamic tool replies")
	}
	if job.OpenViking == nil || job.OpenViking.ID != "same-session" || len(recorder.turns) != 1 {
		t.Fatalf("unexpected first session: %+v", job.OpenViking)
	}
	requests.Reset()
	replies = init + `{"id":4,"result":{"turn":{"id":"turn-b"}}}
{"method":"item/completed","params":{"threadId":"thread-a","turnId":"turn-b","item":{"type":"agentMessage","text":"Continued answer","phase":"final_answer"}}}
{"method":"turn/completed","params":{"threadId":"thread-a","turn":{"id":"turn-b","status":"completed"}}}
`
	job.Stage = waitingToContinue
	job.Progress = &api.ExecutionProgress{ContinueRequested: true, Summary: "Continue on the same branch"}
	if err := codex.Implement(ctx, &job, root, checkpoint); err != nil {
		t.Fatal(err)
	}
	if len(recorder.turns) != 2 || recorder.turns[1].ThreadID != recorder.turns[0].ThreadID ||
		recorder.turns[1].TurnID == recorder.turns[0].TurnID || !strings.Contains(requests.String(), "thread/resume") {
		t.Fatalf("continuation did not retain the Codex thread: %+v", recorder.turns)
	}
}
