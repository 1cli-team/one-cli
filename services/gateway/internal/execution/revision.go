package execution

import (
	"errors"
	"strings"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

var (
	errRevisionPR   = errors.New("revision_requires_open_pr")
	errRevisionBusy = errors.New("revision_in_progress")
)

func (job assignment) canRevise() bool {
	if job.Input.Linear == nil || job.State != api.ExecutionSucceeded || job.Result == nil || len(job.Result.PullRequests) != 1 {
		return false
	}
	pr := job.Result.PullRequests[0]
	return pr.Number > 0 && !pr.Merged && pr.State == "open"
}

func (s *Store) revise(id string, input api.RevisionInput) (api.Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return api.Execution{}, errNotFound
	}
	if linear := job.Input.Linear; linear != nil && linear.Revision != nil {
		job, ok = s.jobs[linear.Revision.OriginID]
		if !ok {
			return api.Execution{}, errNotFound
		}
	}
	root := job.Input.Linear
	if root == nil || job.State != api.ExecutionSucceeded || job.Result == nil || len(job.Result.PullRequests) != 1 {
		return api.Execution{}, errRevisionPR
	}
	comment := strings.ToLower(input.CommentID)
	requestID := "revision/" + job.ID + "/" + comment
	revisionID := digest(requestID)[:32]
	if existing, ok := s.jobs[revisionID]; ok {
		if previous := existing.Input.Linear; previous == nil || previous.Revision == nil ||
			previous.Revision.OriginID != job.ID || previous.Revision.CommentID != comment {
			return api.Execution{}, errConflict
		}
		return s.view(existing), nil
	}
	if !job.canRevise() {
		return api.Execution{}, errRevisionPR
	}
	if worker := s.workers[job.WorkerID]; s.downloads.ChinaMirror && online(worker) && !worker.SupportsChinaMirror {
		return api.Execution{}, errWorkerMirror
	}
	for _, other := range s.jobs {
		if other.Input.Linear != nil && other.Input.Linear.Revision != nil &&
			other.Input.Linear.Revision.OriginID == job.ID &&
			other.State != api.ExecutionSucceeded && other.State != api.ExecutionFailed {
			return api.Execution{}, errRevisionBusy
		}
	}
	source := input.Source
	if source == "" {
		source = "cli"
	}
	linear := *root
	linear.Revision = &api.LinearRevision{OriginID: job.ID, CommentID: comment}
	now := time.Now().UTC()
	next := assignment{Execution: api.Execution{
		ID: revisionID, State: api.ExecutionQueued, CreatedAt: now, UpdatedAt: now,
		ChinaMirror: s.downloads.ChinaMirror,
		ExecutionSubmission: api.ExecutionSubmission{WorkerID: job.WorkerID, Source: source, RequestID: requestID,
			Input: api.ExecutionInput{Kind: api.LinearWorkflow, Linear: &linear, Codex: job.Input.Codex}},
	}}
	return s.view(next), s.save(next)
}
