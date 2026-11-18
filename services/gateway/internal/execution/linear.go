package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type workflowCall func(context.Context, api.WorkRequest) (api.WorkReply, error)

type LinearRouter struct {
	mu    sync.Mutex
	store *Store
	local workflowCall
}

func NewLinearRouter(store *Store, local workflowCall) *LinearRouter {
	return &LinearRouter{store: store, local: local}
}

func (r *LinearRouter) Forward(ctx context.Context, req api.WorkRequest) (api.WorkReply, error) {
	switch req.Operation {
	case api.WorkDispatch, api.WorkDispatchWithOptions:
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.dispatch(ctx, req)
	case api.WorkList:
		return r.list(ctx, req)
	case api.WorkMonitorTasks:
		return r.tasks(ctx, req.TaskFilter)
	default:
		return r.local(ctx, req)
	}
}

func (r *LinearRouter) dispatch(ctx context.Context, req api.WorkRequest) (api.WorkReply, error) {
	remote := req.WorkerID != "" && req.WorkerID != LocalWorkerID
	if (req.ProjectID != "" && !api.ValidIssueID(req.ProjectID)) || !api.ValidIssueReference(req.IssueID) {
		return linearProblem(http.StatusBadRequest, "invalid_issue_reference")
	}
	if job, ok := r.store.linearJob(req.ProjectID, req.IssueID); ok && len(req.SubtaskOrder) == 0 {
		return existingLinear(job, req)
	}
	if req.WorkerID == "" && !r.store.hasLinearProject(req.ProjectID) {
		return r.local(ctx, req)
	}
	reply, err := r.local(ctx, api.WorkRequest{
		Operation: api.WorkResolveIssue, ProjectID: req.ProjectID, IssueID: req.IssueID, SubtaskOrder: req.SubtaskOrder,
	})
	if err != nil || reply.Status != http.StatusOK {
		return reply, err
	}
	var resolved api.ResolvedIssue
	if err := json.Unmarshal(reply.Body, &resolved); err != nil {
		return api.WorkReply{}, err
	}
	req.ProjectID = resolved.Project.ID
	if job, ok := r.store.linearJob(req.ProjectID, resolved.ID); ok {
		if len(req.SubtaskOrder) != 0 && !slices.Equal(resolved.SubtaskOrder, job.Input.Linear.SubtaskOrder) {
			return linearProblem(http.StatusConflict, "subtask_order_conflict")
		}
		return existingLinear(job, req)
	}
	if req.WorkerID == "" {
		return r.local(ctx, req)
	}
	if remote && resolved.LocalStatus != "" {
		return linearProblem(http.StatusConflict, "issue_worker_conflict")
	}
	if resolved.LocalStatus == "" && !resolved.Dispatchable {
		return linearProblem(http.StatusConflict, "issue_ineligible")
	}
	source := req.Source
	if source == "" {
		source = "cli"
	}
	job, err := r.store.submit(api.ExecutionSubmission{
		WorkerID: req.WorkerID, RequestID: linearKey(req.ProjectID, resolved.ID), Source: source,
		Input: api.ExecutionInput{Kind: api.LinearWorkflow, Codex: req.Codex, Linear: &api.LinearExecution{
			Project: resolved.Project, IssueID: resolved.ID, Identifier: resolved.Identifier, Title: resolved.Title, URL: resolved.URL,
			SubtaskOrder: resolved.SubtaskOrder,
		}},
	})
	switch err {
	case nil:
		return linearReply(http.StatusAccepted, api.DispatchResult{Status: api.Processing, JobID: job.ID, ProjectID: req.ProjectID})
	case errWorker:
		return linearProblem(http.StatusNotFound, err.Error())
	case errWorkflow, errModel:
		return linearProblem(http.StatusConflict, err.Error())
	default:
		return api.WorkReply{}, err
	}
}

func existingLinear(job api.Execution, req api.WorkRequest) (api.WorkReply, error) {
	if req.WorkerID != "" && req.WorkerID != job.WorkerID {
		return linearProblem(http.StatusConflict, "issue_worker_conflict")
	}
	if req.Codex != (api.CodexOptions{}) && req.Codex != job.Input.Codex {
		return linearProblem(http.StatusConflict, "codex_options_conflict")
	}
	return linearReply(http.StatusOK, api.DispatchResult{Status: linearStatus(job.State), JobID: job.ID, ProjectID: req.ProjectID})
}

func (r *LinearRouter) list(ctx context.Context, req api.WorkRequest) (api.WorkReply, error) {
	reply, err := r.local(ctx, req)
	targeted := r.store.linearStatuses(req.ProjectID)
	if len(targeted) == 0 {
		return reply, err
	}
	jobs := api.Jobs{Enabled: true, Issues: map[string]api.Status{}}
	if err == nil && reply.Status == http.StatusOK {
		if err := json.Unmarshal(reply.Body, &jobs); err != nil {
			return api.WorkReply{}, err
		}
		if jobs.Issues == nil {
			jobs.Issues = map[string]api.Status{}
		}
	}
	for id, status := range targeted {
		jobs.Issues[id] = status
	}
	return linearReply(http.StatusOK, jobs)
}

func linearKey(projectID, issueID string) string {
	return "linear/" + strings.ToLower(projectID) + "/" + strings.ToLower(issueID)
}

func (s *Store) linearJob(projectID, issueID string) (api.Execution, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if issue := job.Input.Linear; issue != nil && issue.Revision == nil && strings.EqualFold(issue.Project.ID, projectID) &&
			(strings.EqualFold(issue.IssueID, issueID) || strings.EqualFold(issue.Identifier, issueID)) {
			return s.view(job), true
		}
	}
	return api.Execution{}, false
}

func (s *Store) hasLinearProject(projectID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if issue := job.Input.Linear; issue != nil && issue.Revision == nil && strings.EqualFold(issue.Project.ID, projectID) {
			return true
		}
	}
	return false
}

func (s *Store) linearStatuses(projectID string) map[string]api.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	statuses := map[string]api.Status{}
	for _, job := range s.jobs {
		if issue := job.Input.Linear; issue != nil && issue.Revision == nil && strings.EqualFold(issue.Project.ID, projectID) {
			statuses[issue.IssueID] = linearStatus(job.State)
		}
	}
	return statuses
}

func linearStatus(state string) api.Status {
	switch state {
	case api.ExecutionSucceeded:
		return api.Submitted
	case api.ExecutionFailed:
		return api.Failed
	default:
		return api.Processing
	}
}

func linearReply(status int, payload any) (api.WorkReply, error) {
	body, err := json.Marshal(payload)
	return api.WorkReply{Status: status, Body: body}, err
}

func linearProblem(status int, reason string) (api.WorkReply, error) {
	return linearReply(status, map[string]string{"reason": reason})
}

func projectMatches(a, b api.LinearProject) bool {
	if a.ID != b.ID || len(a.Repositories) == 0 || len(a.Repositories) != len(b.Repositories) {
		return false
	}
	return slices.EqualFunc(a.Repositories, b.Repositories, func(left, right api.RepositorySummary) bool {
		return left.Base == right.Base && summaryRef(left).Ref().Matches(summaryRef(right).Ref())
	})
}

func summaryRef(repo api.RepositorySummary) api.RepositoryOption {
	return api.RepositoryOption{Repository: repo.Repository, GitHub: repo.GitHub}
}

func linearProjects(ctx context.Context, call workflowCall) []api.LinearProject {
	if call == nil {
		return nil
	}
	reply, err := call(ctx, api.WorkRequest{Operation: api.WorkStatus})
	if err != nil || reply.Status != http.StatusOK {
		return nil
	}
	var snapshot api.Snapshot
	if json.Unmarshal(reply.Body, &snapshot) != nil {
		return nil
	}
	projects := make([]api.LinearProject, 0, len(snapshot.Mappings))
	for _, item := range snapshot.Mappings {
		repos := item.Repositories
		if len(repos) == 0 {
			repos = []api.RepositorySummary{{Repository: item.Repository, GitHub: item.GitHub, Base: item.Base}}
		}
		projects = append(projects, api.LinearProject{ID: item.ProjectID, Repositories: repos})
	}
	return projects
}

func (r Runner) runLinear(ctx context.Context, input api.ExecutionInput, logs io.Writer) runOutcome {
	if r.workflow == nil {
		return runOutcome{ExecutionResult: failed(i18n.T("gateway.worker_workflow_unavailable"))}
	}
	target := input.Linear
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		probe, cancel := context.WithTimeout(ctx, 3*time.Second)
		projects := linearProjects(probe, r.workflow)
		cancel()
		if projects != nil {
			if target.Revision == nil && !slices.ContainsFunc(projects, func(p api.LinearProject) bool { return projectMatches(p, target.Project) }) {
				return runOutcome{ExecutionResult: failed(i18n.T("gateway.worker_workflow_unavailable"))}
			}
			break
		}
		select {
		case <-ctx.Done():
			return runOutcome{ExecutionResult: failed(ctx.Err().Error())}
		case <-ticker.C:
		}
	}
	req := api.WorkRequest{
		Operation: api.WorkDispatchWithOptions, ProjectID: target.Project.ID, IssueID: target.IssueID,
		Codex: input.Codex, AskUser: r.exchangeQuestion != nil,
		TrackProgress: r.trackProgress,
		SubtaskOrder:  target.SubtaskOrder,
	}
	if target.Revision != nil {
		req.Operation, req.RevisionCommentID = api.WorkReviseIssue, target.Revision.CommentID
	}
	reply, err := r.workflow(ctx, req)
	if err != nil || (reply.Status != http.StatusAccepted && reply.Status != http.StatusOK) {
		message := i18n.T("gateway.worker_workflow_dispatch")
		var problem struct{ Reason string }
		if err == nil && json.Unmarshal(reply.Body, &problem) == nil {
			switch problem.Reason {
			case "invalid_subtask_order", "subtasks_require_single_repository", "subtask_parent_unavailable", "issue_ineligible",
				"invalid_revision", "revision_requires_open_pr", "revision_in_progress", "revision_comment_scope":
				message = i18n.T("gateway.execution_problem." + problem.Reason)
			}
		}
		return runOutcome{ExecutionResult: failed(message)}
	}
	var lastStage, resumedQuestion string
	var lastSubtasks []api.SubtaskResult
	for {
		req.Operation = api.WorkMonitorJobs
		reply, err := r.workflow(ctx, req)
		if err == nil && reply.Status == http.StatusOK {
			var page api.JobPage
			if json.Unmarshal(reply.Body, &page) == nil {
				for _, job := range page.Items {
					if job.ProjectID != target.Project.ID || job.RevisionCommentID != req.RevisionCommentID {
						continue
					}
					if job.Stage != lastStage {
						_, _ = fmt.Fprintln(logs, i18n.Tf("gateway.worker_workflow_stage", job.Identifier, i18n.T("gateway.worker_stage."+job.Stage)))
						lastStage = job.Stage
					}
					if r.reportSubtasks != nil && !slices.Equal(job.Subtasks, lastSubtasks) {
						if err := r.reportSubtasks(ctx, job.Subtasks); err != nil {
							slog.Warn(i18n.T("gateway.worker_contact"), "error", err)
						} else {
							lastSubtasks = job.Subtasks
						}
					}
					if r.exchangeQuestion != nil && job.Question != nil && job.Question.ID != resumedQuestion &&
						(job.Stage == api.ExecutionWaitingForAnswer || job.Question.Answer != "") {
						if (len(target.SubtaskOrder) != 0 || target.Revision != nil) && job.Stage == api.ExecutionWaitingForAnswer &&
							job.Question.ID != r.resumeQuestionID {
							if _, err := r.exchangeQuestion(ctx, *job.Question, true); err != nil {
								slog.Warn(i18n.T("gateway.worker_question_sync"), "error", err)
								break
							}
							return runOutcome{waitingForUser: true}
						}
						if err := r.syncQuestion(ctx, req, job); err != nil {
							slog.Warn(i18n.T("gateway.worker_question_sync"), "error", err)
							break
						}
						if job.Stage != api.ExecutionWaitingForAnswer {
							resumedQuestion = job.Question.ID
						}
					}
					switch job.Stage {
					case api.ExecutionWaitingToContinue:
						if job.Progress == nil {
							return runOutcome{ExecutionResult: failed(i18n.T("gateway.worker_progress_missing"))}
						}
						if job.Progress.ID != r.resumeProgressID {
							return runOutcome{ExecutionResult: api.ExecutionResult{Subtasks: job.Subtasks}, progress: job.Progress}
						}
						if !job.Progress.ContinueRequested {
							if err := r.continueWorkflow(ctx, req, job.Progress.ID); err != nil {
								slog.Warn(i18n.T("gateway.worker_progress_sync"), "error", err)
							}
						}
					case "failed":
						result := failed(i18n.T("gateway.worker_workflow_dispatch"))
						if job.LastError != "" {
							result = failed(job.LastError)
						}
						result.Subtasks = job.Subtasks
						result.OpenViking = job.OpenViking
						result.RevisionIssueID = job.RevisionIssueID
						return runOutcome{ExecutionResult: result}
					case "review", "done":
						result := api.ExecutionResult{Subtasks: job.Subtasks, CommitSHA: job.CommitSHA, RevisionIssueID: job.RevisionIssueID, OpenViking: job.OpenViking}
						if job.CanReadCodexResult {
							result.Response = readLinearCodexResult(ctx, r.workflow, job.ID)
						}
						for _, repo := range job.Repositories {
							if repo.PR != nil {
								result.PullRequests = append(result.PullRequests, *repo.PR)
							}
						}
						return runOutcome{ExecutionResult: result}
					}
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			return runOutcome{ExecutionResult: failed(ctx.Err().Error())}
		case <-ticker.C:
		}
	}
}

func (r Runner) syncQuestion(ctx context.Context, req api.WorkRequest, job api.JobSummary) error {
	waiting := job.Stage == api.ExecutionWaitingForAnswer
	question, err := r.exchangeQuestion(ctx, *job.Question, waiting)
	if err != nil || !waiting || job.Question.Answer != "" || question.Answer == "" {
		return err
	}
	req.Operation = api.WorkAnswerIssue
	req.Answer = api.ExecutionAnswer{QuestionID: question.ID, Answer: question.Answer}
	reply, err := r.workflow(ctx, req)
	if err != nil {
		return err
	}
	if reply.Status != http.StatusOK {
		return i18n.Errorf("gateway.worker_question_answer_failed", reply.Status)
	}
	return nil
}

func (r Runner) continueWorkflow(ctx context.Context, req api.WorkRequest, progressID string) error {
	req.Operation, req.ProgressID = api.WorkContinueIssue, progressID
	reply, err := r.workflow(ctx, req)
	if err != nil {
		return err
	}
	if reply.Status != http.StatusOK {
		return i18n.Errorf("gateway.worker_continue_failed", reply.Status)
	}
	return nil
}

func readLinearCodexResult(ctx context.Context, workflow workflowCall, jobID string) string {
	ctx, cancel := context.WithTimeout(ctx, 14*time.Second)
	defer cancel()
	reply, err := workflow(ctx, api.WorkRequest{Operation: api.WorkCodexJobResult, JobID: jobID})
	if err != nil || reply.Status != http.StatusOK {
		return ""
	}
	var result api.CodexJobResult
	if json.Unmarshal(reply.Body, &result) != nil {
		return ""
	}
	return result.Response
}
