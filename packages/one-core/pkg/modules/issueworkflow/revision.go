package issueworkflow

import (
	"context"
	"errors"
	"slices"
	"strings"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type revisionProblem string

func (p revisionProblem) Error() string { return i18n.T("gateway.execution_problem." + string(p)) }

const (
	revisionRequiresPR    revisionProblem = "revision_requires_open_pr"
	revisionInProgress    revisionProblem = "revision_in_progress"
	revisionCommentScope  revisionProblem = "revision_comment_scope"
	revisionBranchChanged revisionProblem = "revision_branch_changed"
)

type Revision struct {
	CommentID string                  `json:"commentId"`
	Body      string                  `json:"body"`
	Issue     tracking.IssueReference `json:"issue"`
}

func (j Job) revisionCommentID() string {
	if j.Revision == nil {
		return ""
	}
	return j.Revision.CommentID
}

func (j Job) revisionCheckout() (Job, error) {
	if len(j.RepositoryRuns) == 0 {
		if j.PR.Number == 0 {
			return Job{}, revisionRequiresPR
		}
		return j, nil
	}
	index := -1
	for i, repo := range j.RepositoryRuns {
		if repo.PR.Number == 0 {
			continue
		}
		if index >= 0 {
			return Job{}, revisionRequiresPR
		}
		index = i
	}
	if index < 0 {
		return Job{}, revisionRequiresPR
	}
	copy := j.repositoryJob(index)
	copy.PR = j.RepositoryRuns[index].PR
	return copy, nil
}

func (s *Service) revise(ctx context.Context, req api.WorkRequest) error {
	if _, ok := s.store.getRun(req.ProjectID, req.IssueID, req.RevisionCommentID); ok {
		return nil
	}
	parent, ok := s.store.get(req.ProjectID, req.IssueID)
	if !ok || parent.Stage != review {
		return revisionRequiresPR
	}
	job, err := parent.revisionCheckout()
	if err != nil {
		return err
	}
	pr, err := s.openRevisionPR(ctx, job)
	if err != nil {
		return err
	}
	reader, ok := s.linear.(interface {
		CommentByID(context.Context, tracking.Binding, string) (tracking.Comment, string, error)
	})
	if !ok {
		return revisionCommentScope
	}
	binding := parent.Project.binding(parent.ProjectID, parent.Issue.Team.ID)
	comment, issueID, err := reader.CommentByID(ctx, binding, req.RevisionCommentID)
	if err != nil {
		var problem *tracking.Error
		if errors.As(err, &problem) && (problem.Code == "scope" || problem.Code == "not_found") {
			return revisionCommentScope
		}
		return err
	}
	if !strings.EqualFold(comment.ID, req.RevisionCommentID) || strings.TrimSpace(comment.Body) == "" {
		return revisionCommentScope
	}
	issue := parent.Issue
	threadID := job.ThreadID
	if issue.ID != issueID {
		index := slices.IndexFunc(parent.Subtasks, func(task SubtaskRun) bool { return task.Issue.ID == issueID && task.Completed })
		if index < 0 {
			return revisionCommentScope
		}
		issue, threadID = parent.Subtasks[index].Issue, parent.Subtasks[index].ThreadID
	}
	revision := Job{
		ProjectID: parent.ProjectID, Project: job.Project, Issue: parent.Issue,
		Worktree: job.Worktree, CheckDir: job.CheckDir, Branch: job.Branch, PR: pr, ThreadID: threadID,
		Codex: req.Codex, AskUser: req.AskUser, TrackProgress: req.TrackProgress,
		Revision: &Revision{
			CommentID: req.RevisionCommentID, Body: comment.Body,
			Issue: tracking.IssueReference{ID: issue.ID, Identifier: issue.Identifier, Title: issue.Title},
		},
	}
	if _, err := s.store.enqueue(revision); err != nil {
		return err
	}
	s.notifyRun()
	return nil
}

func (s *Service) openRevisionPR(ctx context.Context, job Job) (PullRequest, error) {
	pr, err := s.pr.Read(ctx, job.Project.ref(), job.PR.Number)
	if err != nil {
		return pr, err
	}
	if pr.Number != job.PR.Number || pr.Merged || pr.State != "open" {
		return pr, revisionRequiresPR
	}
	return pr, nil
}

type revisionCheckout interface {
	PrepareRevision(context.Context, Job) (string, error)
	CommitRevision(context.Context, Job) (string, error)
	PushRevision(context.Context, Job) error
}

func (s *Service) prepareRevision(ctx context.Context, job *Job) error {
	checkout, ok := s.git.(revisionCheckout)
	if !ok {
		return revisionBranchChanged
	}
	if _, err := s.openRevisionPR(ctx, *job); err != nil {
		return err
	}
	base, err := checkout.PrepareRevision(ctx, *job)
	if err != nil {
		return err
	}
	job.BaseSHA = base
	return s.store.save(*job)
}

func (s *Service) publishRevision(ctx context.Context, job *Job) error {
	if job.Stage != publishing {
		return nil
	}
	checkout, ok := s.git.(revisionCheckout)
	if !ok {
		return s.saveRevisionFailure(job, revisionBranchChanged)
	}
	pr, err := s.openRevisionPR(ctx, *job)
	if err != nil {
		if errors.Is(err, revisionRequiresPR) {
			return s.saveRevisionFailure(job, err)
		}
		return err
	}
	job.PR = pr
	if job.HeadSHA == "" {
		head, err := checkout.CommitRevision(ctx, *job)
		if err != nil {
			return s.saveRevisionFailure(job, err)
		}
		if head == "" {
			return s.saveStage(job, review)
		}
		job.HeadSHA = head
		if err := s.store.save(*job); err != nil {
			return err
		}
		if _, err := s.openRevisionPR(ctx, *job); err != nil {
			if errors.Is(err, revisionRequiresPR) {
				return s.saveRevisionFailure(job, err)
			}
			return err
		}
	}
	if err := checkout.PushRevision(ctx, *job); err != nil {
		if errors.Is(err, revisionBranchChanged) {
			return s.saveRevisionFailure(job, err)
		}
		return err
	}
	return s.saveStage(job, review)
}

func (s *Service) saveRevisionFailure(job *Job, err error) error {
	job.Stage, job.LastError = failed, err.Error()
	return s.store.save(*job)
}
