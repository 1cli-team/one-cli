package issueworkflow

import (
	"slices"
	"testing"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

func TestReviewWithoutSeparateLinearState(t *testing.T) {
	team := tracking.Team{States: []tracking.State{
		{Named: tracking.Named{ID: "running", Name: "开发中"}, Type: "started"},
		{Named: tracking.Named{ID: "done", Name: "已完成"}, Type: "completed"},
	}}
	flow := defaultTeamFlow(team)
	if flow != (TeamFlow{Running: "running", Review: "running", Done: "done"}) {
		t.Fatalf("unexpected inferred workflow: %+v", flow)
	}

	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service, linear, _, _, prs := fixture(store)
	linear.issue.State.ID = flow.Running
	prs.pr = PullRequest{Number: 1, State: "open"}
	job := Job{
		ID: "job", ProjectID: projectID, Stage: toReview,
		Issue: linear.issue, PR: prs.pr,
		Project: Project{Teams: map[string]TeamFlow{linear.issue.Team.ID: flow}},
	}
	if err := service.deliver(t.Context(), &job); err != nil || job.Stage != review || len(linear.transitions) != 0 {
		t.Fatalf("review stage = %s, transitions = %v, error = %v", job.Stage, linear.transitions, err)
	}
	prs.pr.Merged = true
	for _, stage := range []stage{toDone, done} {
		if err := service.deliver(t.Context(), &job); err != nil || job.Stage != stage {
			t.Fatalf("stage = %s, want %s, error = %v", job.Stage, stage, err)
		}
	}
	if !slices.Equal(linear.transitions, []string{"done"}) {
		t.Fatalf("unexpected Linear transitions: %v", linear.transitions)
	}
}
