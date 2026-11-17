package issueworkflow

import (
	"context"
	"encoding/json"
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
	switch req.Operation {
	case api.WorkStatus:
		return reply(http.StatusOK, s.snapshot())
	case api.WorkMonitorJobs:
		if req.IssueID != "" && !api.ValidIssueReference(req.IssueID) {
			return problem(http.StatusBadRequest, "invalid_issue_reference", nil)
		}
		return reply(http.StatusOK, s.monitorJobs(req.Page, req.IssueID))
	case api.WorkList:
		if uuidPattern.MatchString(req.ProjectID) {
			return reply(http.StatusOK, s.list(req.ProjectID))
		}
	case api.WorkDispatch:
		if uuidPattern.MatchString(req.ProjectID) && api.ValidIssueReference(req.IssueID) {
			status, code, err := s.dispatch(ctx, req.ProjectID, req.IssueID)
			if err != nil {
				return problem(code, "workflow_dispatch", err)
			}
			return reply(code, map[string]api.Status{"status": status})
		}
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
		case queued, running, verifying:
			s.execute(ctx, job)
			continue
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
