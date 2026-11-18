package execution

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"slices"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func (s *Store) syncWorkflows(workerID string, workflows []api.JobSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, workflow := range workflows {
		id := digest(linearKey(workflow.ProjectID, workflow.IssueID))[:32]
		job, found := s.jobs[id]
		if !found || job.WorkerID != workerID || job.Input.Linear == nil || job.Input.Linear.Revision != nil {
			continue
		}
		if job.Workflow != nil && (workflow.UpdatedAt.Before(job.Workflow.UpdatedAt) ||
			reflect.DeepEqual(*job.Workflow, workflow)) {
			continue
		}
		job.Workflow = &workflow
		if err := s.save(job); err != nil {
			return err
		}
	}
	return nil
}

func (l remoteLink) syncTaskSnapshots(ctx context.Context, call workflowCall) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		probe, cancel := context.WithTimeout(ctx, 5*time.Second)
		reply, err := call(probe, api.WorkRequest{Operation: api.WorkTaskSnapshot})
		cancel()
		if err == nil && reply.Status == http.StatusOK {
			var workflows []api.JobSummary
			if json.Unmarshal(reply.Body, &workflows) == nil {
				for batch := range slices.Chunk(workflows, 20) {
					if err := l.sendWorkflows(ctx, batch); err != nil {
						slog.Warn(i18n.T("gateway.worker_task_sync"), "error", err)
						break
					}
				}
			}
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
}

func (l remoteLink) sendWorkflows(ctx context.Context, batch []api.JobSummary) error {
	endpoint := l.endpoint("workflows")
	err := l.send(ctx, endpoint, batch, new(struct{}))
	if rejectsWorkerReport(err) && slices.ContainsFunc(batch, func(job api.JobSummary) bool {
		return job.EffectiveCodex != (api.CodexOptions{})
	}) {
		// Older Gateways reject unknown fields in worker requests.
		legacy := slices.Clone(batch)
		for i := range legacy {
			legacy[i].EffectiveCodex = api.CodexOptions{}
		}
		return l.send(ctx, endpoint, legacy, new(struct{}))
	}
	return err
}
