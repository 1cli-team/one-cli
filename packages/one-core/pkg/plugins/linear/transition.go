package linear

import (
	"context"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

// TransitionIssue applies a state-only update so concurrent edits to other
// fields are preserved. The caller supplies the previously observed state.
func (p *Plugin) TransitionIssue(ctx context.Context, binding tracking.Binding, id, from, to string) error {
	current, err := p.Issue(ctx, binding, id)
	if err != nil {
		return err
	}
	if current.Team.ID != binding.TeamID {
		return tracking.Fail("team")
	}
	if current.State.ID == to {
		return nil
	}
	if current.State.ID != from || to == "" {
		return tracking.Fail("conflict")
	}
	var result struct {
		IssueUpdate struct {
			Success bool `json:"success"`
		} `json:"issueUpdate"`
	}
	err = p.graph(ctx, binding.ConnectionID,
		`mutation OneLinearTransitionIssue($id:String!,$input:IssueUpdateInput!){issueUpdate(id:$id,input:$input){success}}`,
		map[string]any{"id": current.ID, "input": map[string]string{"stateId": to}}, &result)
	if err != nil {
		return err
	}
	if !result.IssueUpdate.Success {
		return tracking.Fail("remote")
	}
	p.clearCache()
	current, err = p.Issue(ctx, binding, id)
	if err != nil {
		return err
	}
	if current.State.ID != to || current.Team.ID != binding.TeamID {
		return tracking.Fail("conflict")
	}
	return nil
}
