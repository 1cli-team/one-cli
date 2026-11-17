package issueworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

const projectID = "11111111-1111-1111-1111-111111111111"
const issueID = "22222222-2222-2222-2222-222222222222"

type fakeLinear struct {
	issue       tracking.Issue
	transitions []string
	failDone    bool
}

func (f *fakeLinear) Issue(context.Context, tracking.Binding, string) (tracking.Issue, error) {
	return f.issue, nil
}

func (*fakeLinear) Team(context.Context, string, string) (tracking.Team, error) {
	return tracking.Team{States: []tracking.State{
		{Named: tracking.Named{ID: "running"}, Type: "started"},
		{Named: tracking.Named{ID: "review"}, Type: "started"},
		{Named: tracking.Named{ID: "done"}, Type: "completed"},
	}}, nil
}

func (f *fakeLinear) TransitionIssue(_ context.Context, _ tracking.Binding, _, from, to string) error {
	if f.failDone && to == "done" {
		return errors.New("temporary Linear failure")
	}
	if f.issue.State.ID != from {
		return errors.New("unexpected previous state")
	}
	f.issue.State.ID = to
	f.transitions = append(f.transitions, to)
	return nil
}

type fakeRunner struct {
	turns, recoveries, checks int
	checkError                error
}

func (r *fakeRunner) Implement(_ context.Context, job *Job, _ string, save func(Job) error) error {
	r.turns++
	job.ThreadID, job.TurnID, job.Stage = "thread", "turn", running
	return save(*job)
}
func (r *fakeRunner) Recover(_ context.Context, _ Job) error { r.recoveries++; return nil }
func (r *fakeRunner) Verify(context.Context, Job, string) error {
	r.checks++
	return r.checkError
}

type fakeGit struct{ pushes int }

func (*fakeGit) Prepare(_ context.Context, job *Job, save func(Job) error) error {
	job.BaseSHA, job.Worktree, job.Branch = "base", "worktree", "one/linear/one-1"
	return save(*job)
}
func (*fakeGit) CommonDir(context.Context, Job) (string, error) { return "/repo/.git", nil }
func (*fakeGit) Commit(context.Context, Job) (string, error)    { return "head", nil }
func (g *fakeGit) Push(context.Context, Job) error              { g.pushes++; return nil }

type fakePRs struct {
	pr        PullRequest
	opens     int
	loseReply bool
}

func (p *fakePRs) Find(context.Context, gitmodule.RepositoryRef, string, string) (PullRequest, error) {
	return p.pr, nil
}
func (p *fakePRs) Read(context.Context, gitmodule.RepositoryRef, int) (PullRequest, error) {
	return p.pr, nil
}
func (p *fakePRs) Create(context.Context, gitmodule.RepositoryRef, codehost.PullRequestInput) (PullRequest, error) {
	p.opens++
	p.pr = PullRequest{Number: 42, State: "open"}
	if p.loseReply {
		return PullRequest{}, errors.New("lost GitHub response")
	}
	return p.pr, nil
}

func fixture(store *Store) (*Service, *fakeLinear, *fakeRunner, *fakeGit, *fakePRs) {
	linear := &fakeLinear{issue: tracking.Issue{
		ID: issueID, Identifier: "ONE-1", Team: tracking.Named{ID: "team"},
		State: tracking.State{Named: tracking.Named{ID: "backlog"}},
	}}
	runner, git, prs := new(fakeRunner), new(fakeGit), new(fakePRs)
	cfg := Config{Projects: map[string]Project{projectID: {Teams: map[string]TeamFlow{
		"team": {Running: "running", Review: "review", Done: "done"},
	}}}}
	service := New(cfg, store, linear, runner, git, prs, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return service, linear, runner, git, prs
}

func TestDispatchAndReconcileAfterLostPRResponse(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service, linear, runner, git, prs := fixture(store)
	prs.loseReply = true
	dispatch := api.WorkRequest{Operation: api.WorkDispatch, ProjectID: projectID, IssueID: issueID}
	verifyWorkRequest(t, service, dispatch, http.StatusAccepted)
	verifyWorkRequest(t, service, dispatch, http.StatusOK)
	var result api.Jobs
	list := verifyWorkRequest(t, service, api.WorkRequest{Operation: api.WorkList, ProjectID: projectID}, http.StatusOK)
	if err := json.Unmarshal(list, &result); err != nil || result.Issues[issueID] != api.Processing {
		t.Fatalf("job status = %v, %v", result, err)
	}
	checkStatus := func(want api.QueueCounts) {
		t.Helper()
		var snapshot api.Snapshot
		output := verifyWorkRequest(t, service, api.WorkRequest{Operation: api.WorkStatus}, http.StatusOK)
		if err := json.Unmarshal(output, &snapshot); err != nil ||
			snapshot.Schema != api.SnapshotSchema || snapshot.Projects != 1 ||
			snapshot.StartedAt.IsZero() || snapshot.Jobs != want {
			t.Fatalf("Gateway status = %+v, %v", snapshot, err)
		}
	}
	checkStatus(api.QueueCounts{Queued: 1})
	job, _ := store.get(projectID, issueID)
	ctx := t.Context()
	if err := service.run(ctx, &job); err != nil || job.Stage != publishing {
		t.Fatalf("execute = %s, %v", job.Stage, err)
	}
	checkStatus(api.QueueCounts{Active: 1})
	if err := service.deliver(ctx, &job); err == nil || job.Stage != opening {
		t.Fatalf("publish = %s, %v", job.Stage, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	service.store = store
	job, _ = store.get(projectID, issueID)
	deliver := func(want stage) {
		t.Helper()
		if err := service.deliver(ctx, &job); err != nil || job.Stage != want {
			t.Fatalf("deliver = %s, %v, want %s", job.Stage, err, want)
		}
	}
	deliver(toReview)
	deliver(review)
	checkStatus(api.QueueCounts{Review: 1})
	deliver(review)
	prs.pr.State, prs.pr.Merged = "closed", true
	deliver(toDone)
	linear.failDone = true
	if err := service.deliver(ctx, &job); err == nil || job.Stage != toDone {
		t.Fatalf("failed writeback = %s, %v", job.Stage, err)
	}
	linear.failDone = false
	deliver(done)
	checkStatus(api.QueueCounts{Completed: 1})
	if runner.turns != 1 || runner.checks != 0 || git.pushes != 1 || prs.opens != 1 ||
		!slices.Equal(linear.transitions, []string{"running", "review", "done"}) {
		t.Fatalf("phase effects: turns=%d checks=%d pushes=%d opens=%d Linear=%v", runner.turns, runner.checks, git.pushes, prs.opens, linear.transitions)
	}
}

func TestVerificationFailureStopsPublication(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service, _, runner, git, prs := fixture(store)
	project := service.projects[projectID]
	project.Check = []string{"true"}
	service.projects[projectID] = project
	runner.checkError = errors.New("check failed")
	if _, _, err := service.dispatch(t.Context(), projectID, issueID); err != nil {
		t.Fatal(err)
	}
	job, _ := store.get(projectID, issueID)
	service.execute(t.Context(), job)
	job, _ = store.get(projectID, issueID)
	if job.Stage != failed || git.pushes != 0 || prs.opens != 0 {
		t.Fatalf("failed verification advanced job: %#v", job)
	}
}

func TestPreviouslyStartedTurnIsReadBackBeforeVerification(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service, _, runner, _, _ := fixture(store)
	job := Job{Stage: running, ProjectID: projectID, ID: issueID, Issue: tracking.Issue{ID: issueID}}
	if err := service.run(t.Context(), &job); err != nil {
		t.Fatal(err)
	}
	if job.Stage != publishing || runner.turns != 0 || runner.recoveries != 1 {
		t.Fatalf("restart started a new turn: stage=%s turns=%d recoveries=%d", job.Stage, runner.turns, runner.recoveries)
	}
}

func verifyWorkRequest(t *testing.T, service *Service, input api.WorkRequest, want int) []byte {
	t.Helper()
	reply := service.Handle(t.Context(), input)
	if reply.Status != want {
		t.Fatalf("request %s = %d, want %d (%s)", input.Operation, reply.Status, want, reply.Body)
	}
	return reply.Body
}
