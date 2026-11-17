package execution

import (
	"errors"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

var errContinueConflict = errors.New("execution_continue_conflict")

// Called with the store lock held after validating the worker and attempt.
func (s *Store) pause(job assignment, update report) error {
	progress := *update.Progress
	current := job.Progress
	if !progress.Valid() || progress.ContinueRequested || job.Input.Kind != api.LinearWorkflow {
		return errContinueConflict
	}
	if current != nil && current.ID == progress.ID {
		if current.Summary != progress.Summary || current.Round != progress.Round {
			return errContinueConflict
		}
		// A lost acknowledgment may be retried after the user requeues the job.
		return nil
	}
	if !activeAssignment(job.State) || (current == nil && progress.Round != 1) ||
		(current != nil && (!current.ContinueRequested || progress.Round != current.Round+1)) ||
		job.PendingQuestion() != nil {
		return errContinueConflict
	}
	if err := s.saveLogs(job.ID, update.Logs); err != nil {
		return err
	}
	job.Progress = &progress
	if update.Result.Subtasks != nil {
		job.Subtasks = update.Result.Subtasks
	}
	job.LogsTruncated = job.LogsTruncated || update.Result.LogsTruncated
	job.State, job.UpdatedAt = api.ExecutionWaitingToContinue, time.Now().UTC()
	return s.save(job)
}

func (s *Store) requestContinue(id string, input api.ContinueExecutionInput) (api.ExecutionProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return api.ExecutionProgress{}, errNotFound
	}
	if job.Progress == nil || job.Progress.ID != input.ProgressID ||
		(job.State != api.ExecutionWaitingToContinue && !job.Progress.ContinueRequested) {
		return api.ExecutionProgress{}, errContinueConflict
	}
	progress := *job.Progress
	if progress.ContinueRequested {
		return progress, nil
	}
	progress.ContinueRequested = true
	job.Progress, job.UpdatedAt = &progress, time.Now().UTC()
	job.State = api.ExecutionQueued
	return progress, s.save(job)
}
