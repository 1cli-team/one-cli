package issueworkflow

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type subtaskProblem string

func (p subtaskProblem) Error() string { return i18n.T("gateway.execution_problem." + string(p)) }

const (
	invalidSubtaskOrder  subtaskProblem = "invalid_subtask_order"
	singleRepositoryOnly subtaskProblem = "subtasks_require_single_repository"
	parentUnavailable    subtaskProblem = "subtask_parent_unavailable"
)

func subtaskErrorStatus(err error) int {
	var problem subtaskProblem
	if errors.As(err, &problem) {
		return http.StatusConflict
	}
	return http.StatusBadGateway
}

type SubtaskRun struct {
	KnowledgeScope      string                 `json:"knowledgeScope,omitempty"`
	OpenVikingSkillTool bool                   `json:"openVikingSkillTool,omitempty"`
	OpenViking          *knowledge.SessionLink `json:"openViking,omitempty"`
	Issue               tracking.Issue         `json:"issue"`
	StartSHA            string                 `json:"startSha,omitempty"`
	CommitSHA           string                 `json:"commitSha,omitempty"`
	ThreadID            string                 `json:"threadId,omitempty"`
	Response            string                 `json:"response,omitempty"`
	Completed           bool                   `json:"completed,omitempty"`
}

func (s *Service) resolveDispatchIssue(ctx context.Context, projectID, ref string, order []string) (Project, tracking.Issue, []string, int, error) {
	project, issue, code, err := s.resolveIssue(ctx, projectID, ref)
	if err != nil || issue.Parent == nil {
		return project, issue, order, code, err
	}
	project, parent, code, err := s.resolveIssue(ctx, projectID, issue.Parent.ID)
	if code == http.StatusNotFound || err == nil && parent.Parent != nil {
		return project, parent, nil, http.StatusConflict, parentUnavailable
	}
	if err != nil {
		return project, parent, nil, code, err
	}
	if len(order) == 0 && !closedIssue(issue) {
		order = []string{issue.ID}
	}
	return project, parent, order, http.StatusOK, nil
}

func closedIssue(issue tracking.Issue) bool {
	return issue.State.Type == "completed" || issue.State.Type == "canceled"
}

func (s *Service) planSubtasks(ctx context.Context, project Project, parent tracking.Issue, order []string) ([]SubtaskRun, error) {
	if !parent.HasChildren && len(order) == 0 {
		return nil, nil
	}
	if len(project.repositorySettings()) != 1 {
		return nil, singleRepositoryOnly
	}
	reader, ok := s.linear.(interface {
		SubIssues(context.Context, tracking.Binding, string, string) (tracking.Issues, error)
	})
	if !ok || parent.Project == nil {
		return nil, parentUnavailable
	}
	binding := project.binding(parent.Project.ID, parent.Team.ID)
	after := ""
	var children []tracking.Issue
	seen := make(map[string]bool)
	for {
		page, err := reader.SubIssues(ctx, binding, parent.ID, after)
		if err != nil {
			var issueErr *tracking.Error
			if errors.As(err, &issueErr) && issueErr.Code == "scope" {
				return nil, parentUnavailable
			}
			return nil, err
		}
		for _, child := range page.Nodes {
			if child.Parent == nil || child.Parent.ID != parent.ID || !api.ValidIssueID(child.ID) {
				return nil, tracking.Fail("remote")
			}
			if child.HasChildren {
				return nil, parentUnavailable
			}
			if _, mapped := project.Teams[child.Team.ID]; !mapped && !closedIssue(child) {
				return nil, subtaskProblem("issue_ineligible")
			}
			if !seen[child.ID] {
				children = append(children, child)
				seen[child.ID] = true
			}
		}
		if !page.PageInfo.HasNextPage {
			break
		}
		if page.PageInfo.EndCursor == "" || page.PageInfo.EndCursor == after {
			return nil, tracking.Fail("remote")
		}
		after = page.PageInfo.EndCursor
	}
	slices.SortFunc(children, func(a, b tracking.Issue) int {
		aTeam, aNumber := splitIdentifier(a.Identifier)
		bTeam, bNumber := splitIdentifier(b.Identifier)
		if aTeam == bTeam && len(aNumber) != len(bNumber) {
			return len(aNumber) - len(bNumber)
		}
		return strings.Compare(a.Identifier, b.Identifier)
	})
	planned := make([]SubtaskRun, 0, len(children))
	clear(seen)
	for _, ref := range order {
		index := slices.IndexFunc(children, func(child tracking.Issue) bool {
			return strings.EqualFold(child.ID, ref) || strings.EqualFold(child.Identifier, ref)
		})
		if index < 0 || seen[children[index].ID] {
			return nil, invalidSubtaskOrder
		}
		child := children[index]
		seen[child.ID] = true
		if !closedIssue(child) {
			planned = append(planned, SubtaskRun{Issue: child})
		}
	}
	for _, child := range children {
		if !seen[child.ID] && !closedIssue(child) {
			planned = append(planned, SubtaskRun{Issue: child})
		}
	}
	return planned, nil
}

func splitIdentifier(identifier string) (string, string) {
	index := strings.LastIndexByte(identifier, '-')
	return identifier[:index+1], identifier[index+1:]
}

func (j Job) activeSubtask() int {
	return slices.IndexFunc(j.Subtasks, func(task SubtaskRun) bool { return !task.Completed })
}

func (s *Service) runSubtask(ctx context.Context, job *Job) error {
	checkout, ok := s.git.(interface {
		ValidateSubtaskStart(context.Context, Job, string) error
		CommitSubtask(context.Context, Job, SubtaskRun) (string, error)
	})
	if !ok {
		return i18n.Errorf("gateway.workflow_subtask_checkout")
	}
	index := job.activeSubtask()
	if index < 0 {
		return i18n.Errorf("gateway.workflow_subtask_checkout")
	}
	task := &job.Subtasks[index]
	commonDir, err := s.git.CommonDir(ctx, *job)
	if err != nil {
		return err
	}
	if job.Stage == queued || job.Stage == preparingSubtask {
		base := job.HeadSHA
		if base == "" {
			base = job.BaseSHA
		}
		if err := checkout.ValidateSubtaskStart(ctx, *job, base); err != nil {
			return err
		}
		task.StartSHA = base
		if err := s.saveStage(job, preparingSubtask); err != nil {
			return err
		}
	}
	switch job.Stage {
	case preparingSubtask, waitingForAnswer, waitingToContinue:
		err = s.runner.Implement(ctx, job, commonDir, s.store.save)
	case running:
		err = s.runner.Recover(ctx, job, s.store.save)
	}
	if err != nil || job.Stage == waitingForAnswer || job.Stage == waitingToContinue {
		return err
	}
	if job.Stage != committingSubtask {
		if len(job.Project.Check) != 0 {
			if err := s.saveStage(job, verifying); err != nil {
				return err
			}
			if err := s.runner.Verify(ctx, *job, commonDir); err != nil {
				return s.checkFailed(ctx, job, err)
			}
		}
		if err := s.saveStage(job, committingSubtask); err != nil {
			return err
		}
	}
	sha, err := checkout.CommitSubtask(ctx, *job, *task)
	if err != nil {
		return err
	}
	task.CommitSHA, task.ThreadID, task.Response, task.Completed = sha, job.ThreadID, job.CodexResponse, true
	task.KnowledgeScope, task.OpenVikingSkillTool, task.OpenViking = job.KnowledgeScope, job.OpenVikingSkillTool, job.OpenViking
	if sha != "" {
		job.HeadSHA = sha
	}
	job.ThreadID, job.TurnID, job.CodexResponse = "", "", ""
	switch {
	case index+1 < len(job.Subtasks):
		if err := s.saveStage(job, preparingSubtask); err != nil {
			return err
		}
		s.notifyRun()
		return nil
	case job.HeadSHA == "":
		return s.saveStage(job, toDone)
	default:
		return s.saveStage(job, publishing)
	}
}

func (s *Service) completeSubtasks(ctx context.Context, job *Job) error {
	for _, task := range job.Subtasks {
		binding := job.Project.binding(job.ProjectID, task.Issue.Team.ID)
		child, err := s.linear.Issue(ctx, binding, task.Issue.ID)
		if err != nil {
			return err
		}
		if closedIssue(child) || child.Parent == nil || child.Parent.ID != job.Issue.ID ||
			child.Team.ID != task.Issue.Team.ID {
			continue
		}
		flow := job.Project.Teams[child.Team.ID]
		if err := s.linear.TransitionIssue(ctx, binding, child.ID, child.State.ID, flow.Done); err != nil {
			return err
		}
	}
	binding := job.Project.binding(job.ProjectID, job.Issue.Team.ID)
	parent, err := s.linear.Issue(ctx, binding, job.Issue.ID)
	if err != nil {
		return err
	}
	if !closedIssue(parent) {
		flow := job.Project.Teams[job.Issue.Team.ID]
		from := flow.Review
		if parent.State.ID == flow.Running {
			from = flow.Running
		}
		if err := s.linear.TransitionIssue(ctx, binding, parent.ID, from, flow.Done); err != nil {
			return err
		}
	}
	return s.saveStage(job, done)
}

func (j Job) subtaskResults() []api.SubtaskResult {
	if len(j.Subtasks) == 0 {
		return nil
	}
	active := j.activeSubtask()
	results := make([]api.SubtaskResult, 0, len(j.Subtasks))
	for i, task := range j.Subtasks {
		state := string(queued)
		switch {
		case j.Stage == done:
			state = string(done)
		case task.Completed && task.CommitSHA != "":
			state = "committed"
		case task.Completed:
			state = "no_changes"
		case i == active:
			state = string(j.Stage)
		}
		results = append(results, api.SubtaskResult{
			Identifier: task.Issue.Identifier, Title: task.Issue.Title, Stage: state,
			CommitSHA: task.CommitSHA, Response: monitorText(task.Response),
		})
	}
	return results
}
