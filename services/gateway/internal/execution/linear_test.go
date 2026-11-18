package execution

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

func TestLinearDispatchPreservesTargetAcrossAliasesAndRestart(t *testing.T) {
	ctx := t.Context()
	store, err := Open(t.TempDir(), "host")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project := api.LinearProject{ID: "11111111-1111-1111-1111-111111111111",
		Repositories: []api.RepositorySummary{{GitHub: "owner/repo"}}}
	issue := api.ResolvedIssue{Dispatchable: true, ID: "22222222-2222-2222-2222-222222222222", Identifier: "TEST-42", Project: project}
	worker := "w-" + strings.ToLower(rand.Text())
	if err := store.join(joinRequest{ID: worker, Name: "remote", Code: store.invite().Code}, rand.Text()+rand.Text(), false); err != nil {
		t.Fatal(err)
	}
	session := rand.Text()
	if err := store.heartbeat(worker, session, api.ExecutionWorker{LinearProjects: []api.LinearProject{project}}); err != nil {
		t.Fatal(err)
	}
	localCalls := 0
	router := NewLinearRouter(store, func(_ context.Context, req api.WorkRequest) (api.WorkReply, error) {
		if req.Operation == api.WorkResolveIssue {
			return linearReply(http.StatusOK, issue)
		}
		localCalls++
		return linearReply(http.StatusAccepted, api.DispatchResult{Status: api.Processing})
	})
	req := api.WorkRequest{Operation: api.WorkDispatchWithOptions, ProjectID: project.ID, IssueID: issue.Identifier,
		WorkerID: worker, Source: "desktop", Codex: api.CodexOptions{Effort: "low"}}
	reply, err := router.Forward(ctx, req)
	if err != nil || reply.Status != http.StatusAccepted {
		t.Fatalf("dispatch: %d %s %v", reply.Status, reply.Body, err)
	}
	var submitted api.DispatchResult
	if err := json.Unmarshal(reply.Body, &submitted); err != nil || submitted.JobID == "" {
		t.Fatalf("response: %+v %v", submitted, err)
	}
	req.IssueID, req.Source = issue.ID, "cli"
	reply, err = router.Forward(ctx, req)
	var repeated api.DispatchResult
	if err != nil || reply.Status != http.StatusOK || json.Unmarshal(reply.Body, &repeated) != nil ||
		submitted.JobID != repeated.JobID {
		t.Fatalf("repeated dispatch: %d %s %v", reply.Status, reply.Body, err)
	}
	req.WorkerID = LocalWorkerID
	reply, err = router.Forward(ctx, req)
	if err != nil || reply.Status != http.StatusConflict || localCalls != 0 {
		t.Fatalf("local reroute: %d %s %v, local calls: %d", reply.Status, reply.Body, err, localCalls)
	}
	claimed, err := store.claim(worker, session)
	if err != nil || claimed == nil || claimed.ID != submitted.JobID {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	newSession := rand.Text()
	if err := store.heartbeat(worker, newSession, api.ExecutionWorker{LinearProjects: []api.LinearProject{project}}); err != nil {
		t.Fatal(err)
	}
	resumed, err := store.claim(worker, newSession)
	if err != nil || resumed == nil || resumed.Attempt != claimed.Attempt || resumed.Session != newSession {
		t.Fatalf("resumption did not keep the Linear assignment: %+v %v", resumed, err)
	}
	settings := api.CodexOptions{Model: "resolved-model", Effort: "low"}
	progress := report{JobID: resumed.ID, Attempt: resumed.Attempt, Result: api.ExecutionResult{EffectiveCodex: settings}}
	if err := store.upload(worker, progress); err != nil {
		t.Fatal(err)
	}
	assertTaskSettings := func() {
		t.Helper()
		task := store.taskPage(api.TaskFilter{}, nil).Items[0]
		if task.Kind != api.LinearWorkflow || task.Codex != req.Codex || task.EffectiveCodex != settings {
			t.Fatalf("task request = %+v, recorded settings = %+v", task.Codex, task.EffectiveCodex)
		}
	}
	assertTaskSettings()
	if err := store.complete(worker, report{JobID: resumed.ID, Attempt: resumed.Attempt}); err != nil {
		t.Fatal(err)
	}
	assertTaskSettings()

	// A legacy local dispatch reserved before this review stays assigned locally.
	issue.ID, issue.Identifier, issue.LocalStatus = "33333333-3333-3333-3333-333333333333", "TEST-43", api.Submitted
	req.IssueID, req.WorkerID = issue.ID, worker
	reply, err = router.Forward(ctx, req)
	if err != nil || reply.Status != http.StatusConflict {
		t.Fatalf("moving an existing local issue: %d %s %v", reply.Status, reply.Body, err)
	}
}

func TestLinearExecutionUsesWorkerWorkflowAndReturnsPR(t *testing.T) {
	project := api.LinearProject{ID: "11111111-1111-1111-1111-111111111111",
		Repositories: []api.RepositorySummary{{GitHub: "owner/repo"}}}
	issue := &api.LinearExecution{IssueID: "22222222-2222-2222-2222-222222222222", Identifier: "TEST-42", Project: project}
	options := api.CodexOptions{Model: "fixture-model"}
	settings := api.CodexOptions{Model: "resolved-model", Effort: "medium"}
	pr := api.JobPullRequest{Number: 42, URL: "https://github.com/owner/repo/pull/42"}
	runner := Runner{workflow: func(_ context.Context, req api.WorkRequest) (api.WorkReply, error) {
		switch req.Operation {
		case api.WorkStatus:
			return linearReply(http.StatusOK, api.Snapshot{Mappings: []api.ProjectSummary{{ProjectID: project.ID, GitHub: "owner/repo"}}})
		case api.WorkDispatchWithOptions:
			if req.ProjectID != project.ID || req.IssueID != issue.IssueID || req.Codex != options || !req.ChinaMirror {
				t.Fatalf("worker dispatch: %+v", req)
			}
			return linearReply(http.StatusAccepted, api.DispatchResult{Status: api.Processing})
		case api.WorkMonitorJobs:
			return linearReply(http.StatusOK, api.JobPage{Items: []api.JobSummary{{
				ProjectID: project.ID, Identifier: issue.Identifier, Stage: "review", EffectiveCodex: settings,
				Repositories: []api.JobRepository{{PR: &pr}},
			}}})
		}
		t.Fatalf("unexpected operation %s", req.Operation)
		return api.WorkReply{}, nil
	}}
	var logs bytes.Buffer
	result := runner.runLinear(t.Context(), api.ExecutionInput{Kind: api.LinearWorkflow, Linear: issue, Codex: options}, true, &logs)
	if result.Error != "" || result.EffectiveCodex != settings || len(result.PullRequests) != 1 || result.PullRequests[0] != pr ||
		!strings.Contains(logs.String(), issue.Identifier) {
		t.Fatalf("result: %+v, logs: %s", result, &logs)
	}
}
