package execution

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type workerLink interface {
	heartbeat(context.Context, string, api.ExecutionWorker) error
	claim(context.Context, string) (*assignment, error)
	complete(context.Context, report) error
	upload(context.Context, report) error
	exchangeQuestion(context.Context, questionReport) (api.ExecutionQuestion, error)
}

type localLink struct{ store *Store }

func (l localLink) heartbeat(_ context.Context, session string, inventory api.ExecutionWorker) error {
	inventory.SupportsChinaMirror = true
	return l.store.heartbeat(LocalWorkerID, session, inventory)
}

func (l localLink) claim(_ context.Context, session string) (*assignment, error) {
	return l.store.claim(LocalWorkerID, session)
}

func (l localLink) complete(_ context.Context, result report) error {
	return l.store.complete(LocalWorkerID, result)
}

func (l localLink) upload(_ context.Context, progress report) error {
	return l.store.upload(LocalWorkerID, progress)
}

func (l localLink) exchangeQuestion(_ context.Context, input questionReport) (api.ExecutionQuestion, error) {
	return l.store.exchangeQuestion(LocalWorkerID, input)
}

func (s *Store) upload(workerID string, progress report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[progress.JobID]
	if !ok || job.WorkerID != workerID || job.Attempt != progress.Attempt || progress.Attempt == "" ||
		!activeAssignment(job.State) {
		return errNotFound
	}
	if err := s.saveLogs(job.ID, progress.Logs); err != nil {
		return err
	}
	updated := job.recordCodexSettings(progress.Result.EffectiveCodex)
	if progress.Result.Subtasks != nil && !slices.Equal(job.Subtasks, progress.Result.Subtasks) {
		job.Subtasks = progress.Result.Subtasks
		updated = true
	}
	if updated {
		return s.save(job)
	}
	return nil
}

func (s *Store) RunLocal(ctx context.Context, runner Runner) error {
	return runWorker(ctx, localLink{store: s}, runner)
}

func runWorker(ctx context.Context, link workerLink, runner Runner) error {
	if err := os.MkdirAll(runner.root, 0700); err != nil {
		return err
	}
	journal := filepath.Join(runner.root, "pending-result.json")
	var pending *report
	if err := readJSON(journal, &pending); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	session := rand.Text()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var inventory api.ExecutionWorker
	var refresh, modelRefresh time.Time
	var active *assignment
	var output *limitedLog
	var finished <-chan runOutcome
	for ctx.Err() == nil {
		if time.Since(refresh) >= 10*time.Second {
			catalogCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			value, err := runner.inventory(catalogCtx)
			if err == nil {
				value.Models = inventory.Models
				value.LinearProjects = linearProjects(catalogCtx, runner.workflow)
				inventory = value
			} else {
				slog.Warn(i18n.T("gateway.worker_catalog"), "error", err)
			}
			cancel()
			refresh = time.Now()
		}
		if time.Since(modelRefresh) >= time.Minute {
			modelCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			inventory.Models = runner.models(modelCtx)
			cancel()
			modelRefresh = time.Now()
		}
		if err := link.heartbeat(ctx, session, inventory); err != nil {
			slog.Warn(i18n.T("gateway.worker_contact"), "error", err)
		} else if pending != nil {
			if err := deliverResult(ctx, link, journal, *pending); err == nil {
				active, pending, output = nil, nil, nil
			} else {
				slog.Warn(i18n.T("gateway.worker_contact"), "error", err)
			}
		} else if active != nil {
			data, _ := output.snapshot()
			if len(data) > 0 {
				if err := link.upload(ctx, report{JobID: active.ID, Attempt: active.Attempt, Logs: data}); err != nil {
					slog.Warn(i18n.T("gateway.worker_contact"), "error", err)
				}
			}
		} else {
			job, err := link.claim(ctx, session)
			if err != nil {
				slog.Warn(i18n.T("gateway.worker_contact"), "error", err)
			} else if job != nil {
				active, output = job, &limitedLog{truncated: job.LogsTruncated}
				_, _ = output.Write(job.PreviousLogs)
				ch := make(chan runOutcome, 1)
				finished = ch
				go func(job assignment, logs *limitedLog) {
					taskRunner := runner
					if job.Input.Kind == api.LinearWorkflow || job.Input.Kind == api.CodexTask {
						taskRunner.reportProgress = func(ctx context.Context, result api.ExecutionResult) error {
							return link.upload(ctx, report{JobID: job.ID, Attempt: job.Attempt, Result: result})
						}
					}
					if job.CanAsk {
						taskRunner.exchangeQuestion = func(ctx context.Context, question api.ExecutionQuestion, waiting bool) (api.ExecutionQuestion, error) {
							question.Answer = ""
							return link.exchangeQuestion(ctx, questionReport{
								JobID: job.ID, Attempt: job.Attempt, Session: job.Session,
								Question: question, Waiting: waiting,
							})
						}
					}
					ch <- taskRunner.execute(ctx, job, logs)
				}(*job, output)
			}
		}
		select {
		case <-ctx.Done():
			if finished != nil {
				result := <-finished
				if result.progress != nil || active.Input.Kind != api.LinearWorkflow {
					pending = finish(active, output, result)
				}
			}
			if pending != nil {
				return writeJSON(journal, pending)
			}
			return nil
		case result := <-finished:
			if ctx.Err() != nil && active.Input.Kind == api.LinearWorkflow && result.progress == nil {
				return nil
			}
			if result.waitingForUser {
				active, output, finished = nil, nil, nil
				continue
			}
			pending = finish(active, output, result)
			if err := writeJSON(journal, pending); err != nil {
				slog.Warn(i18n.T("gateway.worker_contact"), "error", err)
			}
			finished = nil
		case <-ticker.C:
		}
	}
	return nil
}

func finish(job *assignment, output *limitedLog, result runOutcome) *report {
	logs, truncated := output.snapshot()
	result.LogsTruncated = truncated
	return &report{JobID: job.ID, Attempt: job.Attempt, Result: result.ExecutionResult, Logs: logs, Progress: result.progress}
}

func deliverResult(ctx context.Context, link workerLink, journal string, result report) error {
	if err := writeJSON(journal, result); err != nil {
		return err
	}
	if err := link.complete(ctx, result); err != nil {
		return err
	}
	return os.Remove(journal)
}
