package issueworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

// Handle is the command boundary for the worker. It is also used by the CLI
// when executing a workflow locally.
func (s *Service) Handle(ctx context.Context, req api.WorkRequest) api.WorkReply {
	reply := func(status int, data any) api.WorkReply {
		raw, err := json.Marshal(data)
		if err != nil {
			return api.WorkReply{Status: http.StatusInternalServerError}
		}
		return api.WorkReply{Status: status, Body: raw}
	}
	problem := func(code int, reason string, err error) api.WorkReply {
		if err != nil {
			s.logger.Warn("One issue workflow request", "operation", req.Operation, "reason", reason, "error", err)
		}
		return reply(code, map[string]string{"reason": reason})
	}
	dispatchProblem := func(code int, reason string, err error) api.WorkReply {
		var subtask subtaskProblem
		if errors.As(err, &subtask) {
			reason = string(subtask)
		}
		return problem(code, reason, err)
	}
	switch req.Operation {
	case api.WorkStatus:
		return reply(http.StatusOK, s.snapshot())
	case api.WorkCodexModels:
		if reader, ok := s.runner.(interface {
			CodexModels(context.Context) (api.CodexModelCatalog, error)
		}); ok {
			catalog, err := reader.CodexModels(ctx)
			if err != nil {
				return problem(http.StatusBadGateway, "codex_models_unavailable", err)
			}
			return reply(http.StatusOK, catalog)
		}
		return problem(http.StatusNotFound, "codex_models_unavailable", nil)
	case api.WorkCodexJobResult:
		if !uuidPattern.MatchString(req.JobID) {
			return problem(http.StatusBadRequest, "invalid_job_id", nil)
		}
		job, ok := s.store.findID(req.JobID)
		if !ok {
			return problem(http.StatusNotFound, "job_not_found", nil)
		}
		result, err := s.codexJobResult(ctx, job)
		if err != nil {
			return problem(http.StatusBadGateway, "codex_job_result_unavailable", err)
		}
		return reply(http.StatusOK, result)
	case api.WorkTaskSnapshot:
		return reply(http.StatusOK, s.taskSnapshot())
	case api.WorkMonitorJobs:
		if req.RevisionCommentID != "" {
			if !api.ValidIssueID(req.ProjectID) || !api.ValidIssueID(req.IssueID) || !api.ValidIssueID(req.RevisionCommentID) {
				return problem(http.StatusBadRequest, "invalid_revision", nil)
			}
			items := []api.JobSummary{}
			if job, ok := s.store.getRun(req.ProjectID, req.IssueID, req.RevisionCommentID); ok {
				items = append(items, job.monitorSummary())
			}
			return reply(http.StatusOK, api.JobPage{Items: items, Page: 1, PageCount: 1})
		}
		if req.IssueID != "" && !api.ValidIssueReference(req.IssueID) {
			return problem(http.StatusBadRequest, "invalid_issue_reference", nil)
		}
		return reply(http.StatusOK, s.monitorJobs(req.Page, req.IssueID))
	case api.WorkList:
		if uuidPattern.MatchString(req.ProjectID) {
			return reply(http.StatusOK, s.list(req.ProjectID))
		}
	case api.WorkResolveIssue:
		if !api.ValidSubtaskOrder(req.SubtaskOrder) {
			return problem(http.StatusBadRequest, "invalid_subtask_order", nil)
		}
		if req.ProjectID == "" && api.ValidIssueReference(req.IssueID) {
			resolved, code, err := s.findResolvedIssue(ctx, req.IssueID, req.SubtaskOrder)
			switch code {
			case http.StatusOK:
				return reply(code, resolved)
			case http.StatusNotFound:
				return problem(code, "issue_unmapped", err)
			case http.StatusConflict:
				return dispatchProblem(code, "issue_reference_ambiguous", err)
			default:
				return problem(code, "workflow_dispatch", err)
			}
		}
		if uuidPattern.MatchString(req.ProjectID) && api.ValidIssueReference(req.IssueID) {
			resolved, code, err := s.resolvedIssue(ctx, req.ProjectID, req.IssueID, req.SubtaskOrder)
			if err != nil {
				return dispatchProblem(code, "workflow_dispatch", err)
			}
			return reply(http.StatusOK, resolved)
		}
	case api.WorkDispatch, api.WorkDispatchWithOptions:
		if uuidPattern.MatchString(req.ProjectID) && api.ValidIssueReference(req.IssueID) {
			if !req.Codex.Valid() {
				return problem(http.StatusBadRequest, "invalid_codex_options", nil)
			}
			if !api.ValidSubtaskOrder(req.SubtaskOrder) {
				return problem(http.StatusBadRequest, "invalid_subtask_order", nil)
			}
			status, code, err := s.dispatch(ctx, req)
			if errors.Is(err, api.ErrCodexOptionsConflict) {
				return problem(code, "codex_options_conflict", nil)
			}
			if err != nil {
				return dispatchProblem(code, "workflow_dispatch", err)
			}
			return reply(code, map[string]api.Status{"status": status})
		}
	case api.WorkReviseIssue:
		if !api.ValidIssueID(req.ProjectID) || !api.ValidIssueID(req.IssueID) || !api.ValidIssueID(req.RevisionCommentID) || !req.Codex.Valid() {
			return problem(http.StatusBadRequest, "invalid_revision", nil)
		}
		if err := s.revise(ctx, req); err != nil {
			var revision revisionProblem
			if errors.As(err, &revision) {
				return problem(http.StatusConflict, string(revision), err)
			}
			return problem(http.StatusBadGateway, "workflow_dispatch", err)
		}
		return reply(http.StatusAccepted, struct{}{})
	case api.WorkAnswerIssue:
		if !uuidPattern.MatchString(req.ProjectID) || !uuidPattern.MatchString(req.IssueID) || !req.Answer.Valid() {
			return problem(http.StatusBadRequest, "invalid_execution_answer", nil)
		}
		code, err := s.store.answer(req.ProjectID, req.IssueID, req.RevisionCommentID, req.Answer)
		if err != nil {
			return problem(code, "execution_question_conflict", err)
		}
		s.notifyRun()
		return reply(code, struct{}{})
	case api.WorkContinueIssue:
		if !uuidPattern.MatchString(req.ProjectID) || !uuidPattern.MatchString(req.IssueID) || !api.ValidProgressID(req.ProgressID) {
			return problem(http.StatusBadRequest, "invalid_execution_continue", nil)
		}
		code, err := s.store.requestContinue(req.ProjectID, req.IssueID, req.RevisionCommentID, req.ProgressID)
		if err != nil {
			return problem(code, "execution_continue_conflict", err)
		}
		s.notifyRun()
		return reply(code, struct{}{})
	case api.WorkRepositories:
		if s.manager != nil {
			repos, err := s.repositories(ctx)
			if err != nil {
				return problem(http.StatusServiceUnavailable, "repository_unavailable", err)
			}
			return reply(http.StatusOK, repos)
		}
	case api.WorkRepository:
		if s.manager != nil && repositoryPattern.MatchString(req.RepositoryID) {
			repo, err := s.repositoryOption(ctx, req.RepositoryID)
			if err != nil || repo.ID == "" {
				return problem(http.StatusNotFound, "repository_unavailable", err)
			}
			choices, err := s.manager.Catalog.Choices(ctx, repo)
			if err != nil {
				return problem(http.StatusBadGateway, "repository_unavailable", err)
			}
			return reply(http.StatusOK, choices)
		}
	case api.WorkMapping:
		if s.manager != nil && uuidPattern.MatchString(req.ProjectID) {
			if p, ok := s.project(req.ProjectID); ok {
				return reply(http.StatusOK, projectMapping(p))
			}
			return reply(http.StatusOK, nil)
		}
	case api.WorkSaveMapping:
		if s.manager != nil && uuidPattern.MatchString(req.ProjectID) {
			mapping, reason, err := s.saveMapping(ctx, req.ProjectID, req.Mapping)
			if reason == "" {
				return reply(http.StatusOK, mapping)
			}
			status := http.StatusUnprocessableEntity
			if err != nil || reason == "config_write" {
				status = http.StatusBadGateway
			}
			return problem(status, reason, err)
		}
	}
	return problem(http.StatusBadRequest, "invalid_mapping", nil)
}

type workRPC struct {
	ctx     context.Context
	service *Service
}

func (w workRPC) Call(req api.WorkRequest, reply *api.WorkReply) error {
	ctx, cancel := context.WithTimeout(w.ctx, 35*time.Second)
	defer cancel()
	*reply = w.service.Handle(ctx, req)
	return nil
}

func (s *Service) ServeWorker(ctx context.Context, stream *api.WorkerStream) error {
	server := rpc.NewServer()
	if err := server.RegisterName("OneWorkflow", workRPC{ctx: ctx, service: s}); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = stream.Close() })
	defer stop()
	server.ServeCodec(jsonrpc.NewServerCodec(stream))
	return nil
}

type RunResult struct {
	PullRequest
	PullRequests []PullRequest `json:"pullRequests,omitempty"`
}

func (j Job) result() RunResult {
	if len(j.RepositoryRuns) == 0 {
		return RunResult{PullRequest: j.PR}
	}
	result := RunResult{}
	for _, repo := range j.RepositoryRuns {
		if repo.PR.Number > 0 {
			result.PullRequests = append(result.PullRequests, repo.PR)
		}
	}
	if len(result.PullRequests) != 0 {
		result.PullRequest = result.PullRequests[0]
	}
	return result
}

// RunIssue executes or resumes one mapped issue in a local CLI invocation.
func (s *Service) RunIssue(ctx context.Context, project, issue string, onReview func(PullRequest)) (RunResult, error) {
	if !uuidPattern.MatchString(issue) {
		return RunResult{}, i18n.Errorf("linear.cli.workflow_dispatch", http.StatusBadRequest)
	}
	reply := s.Handle(ctx, api.WorkRequest{Operation: api.WorkDispatch, ProjectID: project, IssueID: issue})
	if reply.Status != http.StatusOK && reply.Status != http.StatusAccepted {
		return RunResult{}, i18n.Errorf("linear.cli.workflow_dispatch", reply.Status)
	}
	if _, ok := s.store.get(project, issue); !ok {
		_, parent, _, _, err := s.resolveDispatchIssue(ctx, project, issue, nil)
		if err != nil {
			return RunResult{}, err
		}
		issue = parent.ID
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		job, ok := s.store.get(project, issue)
		if !ok {
			return RunResult{}, i18n.Errorf("linear.cli.workflow_failed", issue)
		}
		if job.Stage == review && onReview != nil {
			result := job.result()
			if len(result.PullRequests) == 0 {
				onReview(result.PullRequest)
			} else {
				for _, pr := range result.PullRequests {
					onReview(pr)
				}
			}
			onReview = nil
		}
		switch job.Stage {
		case done:
			return job.result(), nil
		case failed:
			return RunResult{}, i18n.Errorf("linear.cli.workflow_failed", job.LastError)
		case queued, running, verifying, preparingSubtask, committingSubtask:
			s.execute(ctx, job)
			continue
		case waitingForAnswer:
			if job.readyToResume() {
				s.execute(ctx, job)
				continue
			}
			return RunResult{}, i18n.Errorf("gateway.cli.workflow_waiting_for_answer")
		case waitingToContinue:
			if job.readyToResume() {
				s.execute(ctx, job)
				continue
			}
			return RunResult{}, i18n.Errorf("gateway.cli.workflow_waiting_to_continue")
		default:
			previous := job.Stage
			if err := s.deliver(ctx, &job); err != nil && ctx.Err() == nil {
				s.recordError(job, err, false)
			}
			current, _ := s.store.get(project, issue)
			if current.Stage != previous {
				continue
			}
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
	return RunResult{}, ctx.Err()
}
