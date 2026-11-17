package linear

import (
	"context"
	"slices"
	"strings"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

func (p *Plugin) Teams(ctx context.Context, connection string) ([]tracking.Named, error) {
	teams := []tracking.Named{}
	after := ""
	for {
		var data struct {
			Teams page[tracking.Named] `json:"teams"`
		}
		if err := p.graph(ctx, connection, `query OneLinearTeams($after: String) { teams(first: 100, after: $after) { nodes { id name } pageInfo { endCursor hasNextPage } } }`, map[string]any{"after": cursor(after)}, &data); err != nil {
			return nil, err
		}
		teams = append(teams, data.Teams.Nodes...)
		page := data.Teams.PageInfo
		if !page.HasNextPage {
			break
		}
		if page.EndCursor == "" || page.EndCursor == after {
			return nil, tracking.Fail("remote")
		}
		after = page.EndCursor
	}
	slices.SortFunc(teams, func(a, b tracking.Named) int { return strings.Compare(a.Name, b.Name) })
	return teams, nil
}

func (p *Plugin) CreateProject(ctx context.Context, id string, input tracking.ProjectInput) (tracking.Project, error) {
	var data struct {
		ProjectCreate struct {
			Success bool          `json:"success"`
			Project remoteProject `json:"project"`
		} `json:"projectCreate"`
	}
	variables := map[string]any{"input": map[string]any{
		"id": id, "name": input.Name, "teamIds": []string{input.TeamID},
	}}
	err := p.graph(ctx, input.ConnectionID, `mutation OneLinearCreateProject($input: ProjectCreateInput!) { projectCreate(input: $input) { success project { id name description url teams(first: 100) { nodes { id name } } } } }`, variables, &data)
	if err != nil {
		return tracking.Project{}, err
	}
	project := data.ProjectCreate.Project
	if !data.ProjectCreate.Success || project.ID != id {
		return tracking.Project{}, tracking.Fail("remote")
	}
	return tracking.Project{Named: project.Named, Description: project.Description, URL: project.URL, Teams: project.Teams.Nodes}, nil
}
