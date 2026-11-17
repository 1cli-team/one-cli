package linear

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

type page[T any] struct {
	Nodes    []T               `json:"nodes"`
	PageInfo tracking.PageInfo `json:"pageInfo"`
}
type remoteProject struct {
	tracking.Named
	Description string               `json:"description"`
	URL         string               `json:"url"`
	Teams       page[tracking.Named] `json:"teams"`
}
type remoteIssue struct {
	ID          string                 `json:"id"`
	Identifier  string                 `json:"identifier"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	URL         string                 `json:"url"`
	UpdatedAt   string                 `json:"updatedAt"`
	Priority    int                    `json:"priority"`
	Team        tracking.Named         `json:"team"`
	Project     *tracking.Named        `json:"project"`
	State       tracking.State         `json:"state"`
	Assignee    *tracking.Named        `json:"assignee"`
	Labels      page[tracking.Named]   `json:"labels"`
	Comments    page[tracking.Comment] `json:"comments"`
}

const issueFields = `id identifier title description url updatedAt priority team { id name } project { id name } state { id name type position } assignee { id name } labels(first: 100) { nodes { id name } pageInfo { endCursor hasNextPage } }`

func (r remoteIssue) public() tracking.Issue {
	labels := r.Labels.Nodes
	if labels == nil {
		labels = []tracking.Named{}
	}
	comments := r.Comments.Nodes
	if comments == nil {
		comments = []tracking.Comment{}
	}
	return tracking.Issue{ID: r.ID, Identifier: r.Identifier, Title: r.Title, Description: r.Description, URL: r.URL, UpdatedAt: r.UpdatedAt, Priority: r.Priority, Team: r.Team, Project: r.Project, State: r.State, Assignee: r.Assignee, Labels: labels, Comments: comments, CommentsPageInfo: r.Comments.PageInfo}
}

func (p *Plugin) Projects(ctx context.Context, connection, after string) (tracking.Projects, error) {
	var data struct {
		Projects page[remoteProject] `json:"projects"`
	}
	err := p.graph(ctx, connection, `query OneLinearProjects($after: String) { projects(first: 50, after: $after, orderBy: updatedAt) { nodes { id name description url } pageInfo { endCursor hasNextPage } } }`, map[string]any{"after": cursor(after)}, &data)
	result := tracking.Projects{Nodes: []tracking.Project{}, PageInfo: data.Projects.PageInfo}
	for _, item := range data.Projects.Nodes {
		result.Nodes = append(result.Nodes, tracking.Project{Named: item.Named, Description: item.Description, URL: item.URL, Teams: []tracking.Named{}})
	}
	return result, err
}

func (p *Plugin) Project(ctx context.Context, connection, id string) (tracking.Project, error) {
	result := tracking.Project{Teams: []tracking.Named{}}
	after := ""
	for {
		var data struct {
			Project *remoteProject `json:"project"`
		}
		err := p.graph(ctx, connection, `query OneLinearProject($id: String!, $after: String) { project(id:$id) { id name description url teams(first:100, after:$after) { nodes { id name } pageInfo { endCursor hasNextPage } } } }`, map[string]any{"id": id, "after": cursor(after)}, &data)
		if err != nil {
			return result, err
		}
		if data.Project == nil {
			return result, tracking.Fail("not_found")
		}
		result.Named = data.Project.Named
		result.Description = data.Project.Description
		result.URL = data.Project.URL
		result.Teams = append(result.Teams, data.Project.Teams.Nodes...)
		if !data.Project.Teams.PageInfo.HasNextPage {
			break
		}
		if data.Project.Teams.PageInfo.EndCursor == "" || after == data.Project.Teams.PageInfo.EndCursor {
			return result, tracking.Fail("remote")
		}
		after = data.Project.Teams.PageInfo.EndCursor
	}
	return result, nil
}

func collectTeam[T any](ctx context.Context, p *Plugin, connection, id, field, selection string) ([]T, error) {
	result := []T{}
	after := ""
	for {
		var data struct {
			Team map[string]json.RawMessage `json:"team"`
		}
		query := fmt.Sprintf(`query OneLinearTeamItems($id:String!, $after:String) { team(id:$id) { %s(first:100, after:$after) { nodes { %s } pageInfo { endCursor hasNextPage } } } }`, field, selection)
		if err := p.graph(ctx, connection, query, map[string]any{"id": id, "after": cursor(after)}, &data); err != nil {
			return nil, err
		}
		var items page[T]
		if data.Team == nil {
			return nil, tracking.Fail("not_found")
		}
		if json.Unmarshal(data.Team[field], &items) != nil {
			return nil, tracking.Fail("remote")
		}
		result = append(result, items.Nodes...)
		if !items.PageInfo.HasNextPage {
			return result, nil
		}
		if items.PageInfo.EndCursor == "" || items.PageInfo.EndCursor == after {
			return nil, tracking.Fail("remote")
		}
		after = items.PageInfo.EndCursor
	}
}

func (p *Plugin) Team(ctx context.Context, connection, id string) (tracking.Team, error) {
	var data struct {
		Team *tracking.Team `json:"team"`
	}
	if err := p.graph(ctx, connection, `query OneLinearTeam($id:String!) { team(id:$id) { id name key } }`, map[string]any{"id": id}, &data); err != nil {
		return tracking.Team{}, err
	}
	if data.Team == nil {
		return tracking.Team{}, tracking.Fail("not_found")
	}
	result := *data.Team
	var err error
	result.States, err = collectTeam[tracking.State](ctx, p, connection, id, "states", "id name type position")
	if err != nil {
		return result, err
	}
	result.Members, err = collectTeam[tracking.Named](ctx, p, connection, id, "members", "id name")
	if err != nil {
		return result, err
	}
	labels, err := collectTeam[struct {
		tracking.Named
		IsGroup bool `json:"isGroup"`
	}](ctx, p, connection, id, "labels", "id name isGroup")
	if err != nil {
		return result, err
	}
	result.Labels = []tracking.Named{}
	for _, label := range labels {
		if !label.IsGroup {
			result.Labels = append(result.Labels, label.Named)
		}
	}
	sort.SliceStable(result.States, func(i, j int) bool { return result.States[i].Position < result.States[j].Position })
	return result, nil
}

func cursor(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (p *Plugin) Issues(ctx context.Context, binding tracking.Binding, filter tracking.IssueFilter) (tracking.Issues, error) {
	where := map[string]any{"project": map[string]any{"id": map[string]any{"eq": binding.Project.ID}}}
	if filter.Query != "" {
		where["title"] = map[string]any{"containsIgnoreCase": filter.Query}
	}
	for key, value := range map[string]string{"team": filter.TeamID, "state": filter.StateID, "assignee": filter.AssigneeID} {
		if value != "" {
			where[key] = map[string]any{"id": map[string]any{"eq": value}}
		}
	}
	if filter.Priority != nil {
		if *filter.Priority < 0 || *filter.Priority > 4 {
			return tracking.Issues{}, tracking.Fail("input")
		}
		where["priority"] = map[string]any{"eq": *filter.Priority}
	}
	variables := map[string]any{"filter": where, "after": cursor(filter.After)}
	keyBytes, _ := json.Marshal([]any{binding.ConnectionID, binding.OrganizationID, binding.Scope, variables})
	key := string(keyBytes)
	var data struct {
		Issues page[remoteIssue] `json:"issues"`
	}
	err := p.graph(ctx, binding.ConnectionID, `query OneLinearIssues($filter: IssueFilter, $after: String) { issues(first:50, after:$after, filter:$filter, orderBy:updatedAt) { nodes { `+issueFields+` } pageInfo { endCursor hasNextPage } } }`, variables, &data)
	if err != nil {
		var problem *tracking.Error
		if errors.As(err, &problem) && (problem.Code == "network" || problem.Code == "rate_limit") {
			p.mu.Lock()
			cached, ok := p.cache[key]
			p.mu.Unlock()
			if ok {
				cached.Stale = true
				return cached, nil
			}
		}
		return tracking.Issues{}, err
	}
	result := tracking.Issues{Nodes: []tracking.Issue{}, PageInfo: data.Issues.PageInfo, FetchedAt: time.Now().UTC().Format(time.RFC3339)}
	for _, item := range data.Issues.Nodes {
		if item.Project == nil || item.Project.ID != binding.Project.ID {
			return result, tracking.Fail("scope")
		}
		result.Nodes = append(result.Nodes, item.public())
	}
	p.mu.Lock()
	if len(p.cache) >= 256 {
		p.cache = map[string]tracking.Issues{}
	}
	p.cache[key] = result
	p.mu.Unlock()
	return result, nil
}

func (p *Plugin) Issue(ctx context.Context, binding tracking.Binding, id string) (tracking.Issue, error) {
	var data struct {
		Issue *remoteIssue `json:"issue"`
	}
	err := p.graph(ctx, binding.ConnectionID, `query OneLinearIssue($id:String!) { issue(id:$id) { `+issueFields+` comments(first:50) { nodes { id body createdAt user { id name } } pageInfo { endCursor hasNextPage } } } }`, map[string]any{"id": id}, &data)
	if err != nil {
		return tracking.Issue{}, err
	}
	if data.Issue == nil {
		return tracking.Issue{}, tracking.Fail("not_found")
	}
	if data.Issue.Project == nil || data.Issue.Project.ID != binding.Project.ID {
		return tracking.Issue{}, tracking.Fail("scope")
	}
	result := data.Issue.public()
	if data.Issue.Labels.PageInfo.HasNextPage {
		after := data.Issue.Labels.PageInfo.EndCursor
		for {
			var extra struct {
				Issue struct {
					Labels page[tracking.Named] `json:"labels"`
				} `json:"issue"`
			}
			if err := p.graph(ctx, binding.ConnectionID, `query OneLinearIssueLabels($id:String!,$after:String){issue(id:$id){labels(first:100,after:$after){nodes{id name} pageInfo{endCursor hasNextPage}}}}`, map[string]any{"id": id, "after": after}, &extra); err != nil {
				return result, err
			}
			result.Labels = append(result.Labels, extra.Issue.Labels.Nodes...)
			if !extra.Issue.Labels.PageInfo.HasNextPage {
				break
			}
			if extra.Issue.Labels.PageInfo.EndCursor == "" || extra.Issue.Labels.PageInfo.EndCursor == after {
				return result, tracking.Fail("remote")
			}
			after = extra.Issue.Labels.PageInfo.EndCursor
		}
	}
	return result, nil
}

func (p *Plugin) Comments(ctx context.Context, binding tracking.Binding, id, after string) ([]tracking.Comment, tracking.PageInfo, error) {
	if _, err := p.Issue(ctx, binding, id); err != nil {
		return nil, tracking.PageInfo{}, err
	}
	var data struct {
		Issue struct {
			Comments page[tracking.Comment] `json:"comments"`
		} `json:"issue"`
	}
	err := p.graph(ctx, binding.ConnectionID, `query OneLinearComments($id:String!,$after:String){issue(id:$id){comments(first:50,after:$after){nodes{id body createdAt user{id name}} pageInfo{endCursor hasNextPage}}}}`, map[string]any{"id": id, "after": cursor(after)}, &data)
	return data.Issue.Comments.Nodes, data.Issue.Comments.PageInfo, err
}

func (p *Plugin) validateInput(ctx context.Context, binding tracking.Binding, input tracking.IssueInput) (tracking.Team, error) {
	if strings.TrimSpace(input.Title) == "" || input.Priority < 0 || input.Priority > 4 {
		return tracking.Team{}, tracking.Fail("input")
	}
	project, err := p.Project(ctx, binding.ConnectionID, binding.Project.ID)
	if err != nil {
		return tracking.Team{}, err
	}
	found := false
	for _, team := range project.Teams {
		if team.ID == input.TeamID {
			found = true
		}
	}
	if !found {
		return tracking.Team{}, tracking.Fail("team")
	}
	team, err := p.Team(ctx, binding.ConnectionID, input.TeamID)
	if err != nil {
		return team, err
	}
	if input.StateID != "" && !slices.ContainsFunc(team.States, func(item tracking.State) bool { return item.ID == input.StateID }) {
		return team, tracking.Fail("input")
	}
	if input.AssigneeID != "" && !slices.ContainsFunc(team.Members, func(item tracking.Named) bool { return item.ID == input.AssigneeID }) {
		return team, tracking.Fail("input")
	}
	for _, id := range input.LabelIDs {
		if !slices.ContainsFunc(team.Labels, func(item tracking.Named) bool { return item.ID == id }) {
			return team, tracking.Fail("input")
		}
	}
	return team, nil
}

var operationPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[89abAB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$`)

func (p *Plugin) beginOperation(ctx context.Context, binding tracking.Binding, id, kind, issueID string, payload any) (bool, error) {
	if !operationPattern.MatchString(id) {
		return false, tracking.Fail("operation")
	}
	raw, _ := json.Marshal(payload)
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	fresh := false
	err := p.transaction(ctx, func(doc *document) (bool, error) {
		if previous, ok := doc.Operations[id]; ok {
			if previous.ConnectionID != binding.ConnectionID || previous.ProjectID != binding.Project.ID || previous.Kind != kind || previous.IssueID != issueID || previous.Digest != digest {
				return false, tracking.Fail("operation")
			}
			return false, nil
		}
		doc.Operations[id] = operationRecord{Operation: tracking.Operation{ID: id, Kind: kind, Status: "unknown", ResourceID: id}, ConnectionID: binding.ConnectionID, ProjectID: binding.Project.ID, IssueID: issueID, Digest: digest}
		fresh = true
		return true, nil
	})
	return fresh, err
}

func unknownOperation(id string) error {
	return &tracking.Error{Code: "outcome_unknown", OperationID: id}
}

func (p *Plugin) finishOperation(ctx context.Context, id string) error {
	err := p.transaction(ctx, func(doc *document) (bool, error) {
		item, ok := doc.Operations[id]
		if !ok {
			return false, tracking.Fail("operation")
		}
		item.Status = "confirmed"
		doc.Operations[id] = item
		return true, nil
	})
	if err != nil {
		return unknownOperation(id)
	}
	p.clearCache()
	return nil
}

func issueValues(input tracking.IssueInput) map[string]any {
	labels := input.LabelIDs
	if labels == nil {
		labels = []string{}
	}
	return map[string]any{"title": strings.TrimSpace(input.Title), "description": input.Description, "priority": input.Priority, "assigneeId": cursor(input.AssigneeID), "labelIds": labels}
}

func (p *Plugin) CreateIssue(ctx context.Context, binding tracking.Binding, input tracking.IssueInput) (tracking.Issue, error) {
	if _, err := p.validateInput(ctx, binding, input); err != nil {
		return tracking.Issue{}, err
	}
	fresh, err := p.beginOperation(ctx, binding, input.OperationID, "issue", "", input)
	if err != nil {
		return tracking.Issue{}, err
	}
	if !fresh {
		item, err := p.Issue(ctx, binding, input.OperationID)
		if err != nil {
			return item, unknownOperation(input.OperationID)
		}
		return item, p.finishOperation(ctx, input.OperationID)
	}
	values := issueValues(input)
	values["id"] = input.OperationID
	values["projectId"] = binding.Project.ID
	values["teamId"] = input.TeamID
	if input.StateID != "" {
		values["stateId"] = input.StateID
	}
	var data struct {
		IssueCreate struct {
			Success bool         `json:"success"`
			Issue   *remoteIssue `json:"issue"`
		} `json:"issueCreate"`
	}
	err = p.graph(ctx, binding.ConnectionID, `mutation OneLinearCreateIssue($input:IssueCreateInput!){issueCreate(input:$input){success issue{`+issueFields+`}}}`, map[string]any{"input": values}, &data)
	if err != nil || !data.IssueCreate.Success || data.IssueCreate.Issue == nil || data.IssueCreate.Issue.ID != input.OperationID {
		return tracking.Issue{}, unknownOperation(input.OperationID)
	}
	return data.IssueCreate.Issue.public(), p.finishOperation(ctx, input.OperationID)
}

func (p *Plugin) UpdateIssue(ctx context.Context, binding tracking.Binding, id string, input tracking.IssueInput) (tracking.Issue, error) {
	current, err := p.Issue(ctx, binding, id)
	if err != nil {
		return current, err
	}
	if input.ExpectedUpdatedAt == "" || current.UpdatedAt != input.ExpectedUpdatedAt {
		return current, tracking.Fail("conflict")
	}
	if input.TeamID != current.Team.ID {
		return current, tracking.Fail("team")
	}
	if _, err := p.validateInput(ctx, binding, input); err != nil {
		return current, err
	}
	values := issueValues(input)
	if input.StateID != "" {
		values["stateId"] = input.StateID
	}
	var data struct {
		IssueUpdate struct {
			Success bool         `json:"success"`
			Issue   *remoteIssue `json:"issue"`
		} `json:"issueUpdate"`
	}
	err = p.graph(ctx, binding.ConnectionID, `mutation OneLinearUpdateIssue($id:String!,$input:IssueUpdateInput!){issueUpdate(id:$id,input:$input){success issue{`+issueFields+`}}}`, map[string]any{"id": id, "input": values}, &data)
	if err != nil {
		return current, err
	}
	if !data.IssueUpdate.Success || data.IssueUpdate.Issue == nil {
		return current, tracking.Fail("remote")
	}
	p.clearCache()
	return data.IssueUpdate.Issue.public(), nil
}

func (p *Plugin) readComment(ctx context.Context, binding tracking.Binding, id string) (tracking.Comment, error) {
	var data struct {
		Comment *struct {
			tracking.Comment
			Issue struct {
				ID      string          `json:"id"`
				Project *tracking.Named `json:"project"`
			} `json:"issue"`
		} `json:"comment"`
	}
	err := p.graph(ctx, binding.ConnectionID, `query OneLinearComment($id:String!){comment(id:$id){id body createdAt user{id name} issue{id project{id name}}}}`, map[string]any{"id": id}, &data)
	if err != nil {
		return tracking.Comment{}, err
	}
	if data.Comment == nil {
		return tracking.Comment{}, tracking.Fail("not_found")
	}
	if data.Comment.Issue.Project == nil || data.Comment.Issue.Project.ID != binding.Project.ID {
		return tracking.Comment{}, tracking.Fail("scope")
	}
	return data.Comment.Comment, nil
}

func (p *Plugin) Comment(ctx context.Context, binding tracking.Binding, issueID, body, operationID string) (tracking.Comment, error) {
	if strings.TrimSpace(body) == "" {
		return tracking.Comment{}, tracking.Fail("input")
	}
	if _, err := p.Issue(ctx, binding, issueID); err != nil {
		return tracking.Comment{}, err
	}
	fresh, err := p.beginOperation(ctx, binding, operationID, "comment", issueID, body)
	if err != nil {
		return tracking.Comment{}, err
	}
	if !fresh {
		comment, err := p.readComment(ctx, binding, operationID)
		if err != nil {
			return comment, unknownOperation(operationID)
		}
		return comment, p.finishOperation(ctx, operationID)
	}
	var data struct {
		CommentCreate struct {
			Success bool              `json:"success"`
			Comment *tracking.Comment `json:"comment"`
		} `json:"commentCreate"`
	}
	err = p.graph(ctx, binding.ConnectionID, `mutation OneLinearCreateComment($input:CommentCreateInput!){commentCreate(input:$input){success comment{id body createdAt user{id name}}}}`, map[string]any{"input": map[string]any{"id": operationID, "issueId": issueID, "body": body}}, &data)
	if err != nil || !data.CommentCreate.Success || data.CommentCreate.Comment == nil || data.CommentCreate.Comment.ID != operationID {
		return tracking.Comment{}, unknownOperation(operationID)
	}
	return *data.CommentCreate.Comment, p.finishOperation(ctx, operationID)
}

func (p *Plugin) Operation(ctx context.Context, binding tracking.Binding, id string) (tracking.Operation, error) {
	var operation operationRecord
	err := p.transaction(ctx, func(doc *document) (bool, error) {
		item, ok := doc.Operations[id]
		if !ok || item.ConnectionID != binding.ConnectionID || item.ProjectID != binding.Project.ID {
			return false, tracking.Fail("operation")
		}
		operation = item
		return false, nil
	})
	if err != nil {
		return operation.Operation, err
	}
	if operation.Status == "confirmed" {
		return operation.Operation, nil
	}
	if operation.Kind == "issue" {
		_, err = p.Issue(ctx, binding, operation.ResourceID)
	} else {
		_, err = p.readComment(ctx, binding, operation.ResourceID)
	}
	if err != nil {
		return operation.Operation, nil
	}
	if err = p.finishOperation(ctx, id); err != nil {
		return operation.Operation, err
	}
	operation.Status = "confirmed"
	return operation.Operation, nil
}

func (p *Plugin) PendingOperations(ctx context.Context, binding tracking.Binding) ([]tracking.Operation, error) {
	result := []tracking.Operation{}
	err := p.transaction(ctx, func(doc *document) (bool, error) {
		for _, operation := range doc.Operations {
			if operation.ConnectionID == binding.ConnectionID && operation.ProjectID == binding.Project.ID && operation.Status == "unknown" {
				result = append(result, operation.Operation)
			}
		}
		return false, nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, err
}
