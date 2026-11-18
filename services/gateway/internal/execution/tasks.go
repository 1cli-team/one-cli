package execution

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

type taskEntry struct {
	task     api.Task
	workflow *api.JobSummary
	original *api.Execution
}

func (r *LinearRouter) tasks(ctx context.Context, filter api.TaskFilter) (api.WorkReply, error) {
	if r.local == nil {
		result := r.store.taskPage(filter, nil)
		result.Partial = true
		return linearReply(http.StatusOK, result)
	}
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var local []api.JobSummary
	reply, err := r.local(probe, api.WorkRequest{Operation: api.WorkTaskSnapshot})
	partial := err != nil || reply.Status != http.StatusOK || json.Unmarshal(reply.Body, &local) != nil
	if partial {
		local = nil
	}
	result := r.store.taskPage(filter, local)
	result.Partial = partial
	return linearReply(http.StatusOK, result)
}

func (s *Store) taskPage(filter api.TaskFilter, local []api.JobSummary) api.TaskPage {
	s.mu.Lock()
	entries := make(map[string]*taskEntry)
	workerNames := make(map[string]string, len(s.workers))
	for id, worker := range s.workers {
		workerNames[id] = worker.Name
	}
	entryFor := func(key string) *taskEntry {
		entry := entries[key]
		if entry == nil {
			entry = &taskEntry{task: api.Task{ID: key}}
			entries[key] = entry
		}
		return entry
	}
	for i := range local {
		job := &local[i]
		entryFor(linearKey(job.ProjectID, job.IssueID)).workflow = job
	}
	for _, assignment := range s.jobs {
		job := s.view(assignment)
		linear := job.Input.Linear
		key := job.ID
		if linear != nil {
			key = linearKey(linear.Project.ID, linear.IssueID)
		}
		entry := entryFor(key)
		run := api.TaskRun{
			ID: job.ID, WorkerID: job.WorkerID, WorkerName: cmp.Or(workerNames[job.WorkerID], job.WorkerID),
			Source: job.Source, State: job.State, CreatedAt: job.CreatedAt, Revision: linear != nil && linear.Revision != nil,
		}
		if job.State == api.ExecutionWaitingForAnswer && len(job.Questions) != 0 {
			run.Question = &job.Questions[len(job.Questions)-1]
		}
		if job.State == api.ExecutionWaitingToContinue {
			run.Progress = job.Progress
		}
		entry.task.Runs = append(entry.task.Runs, run)
		entry.task.UpdatedAt = later(entry.task.UpdatedAt, job.UpdatedAt)
		if !run.Revision {
			entry.original = &job
			if assignment.Workflow != nil && (entry.workflow == nil || job.WorkerID != LocalWorkerID) {
				entry.workflow = assignment.Workflow
			}
		}
	}
	s.mu.Unlock()

	tasks := make([]api.Task, 0, len(entries))
	for _, entry := range entries {
		task := entry.finish(workerNames[LocalWorkerID])
		if matchesTask(task, filter) {
			tasks = append(tasks, task)
		}
	}
	slices.SortFunc(tasks, func(a, b api.Task) int {
		return cmp.Or(b.UpdatedAt.Compare(a.UpdatedAt), cmp.Compare(a.ID, b.ID))
	})
	const pageSize = 20
	pages := max(1, (len(tasks)+pageSize-1)/pageSize)
	page := min(max(1, filter.Page), pages)
	start := (page - 1) * pageSize
	return api.TaskPage{Items: tasks[start:min(start+pageSize, len(tasks))], Page: page, PageCount: pages}
}

func (e taskEntry) finish(localName string) api.Task {
	task := e.task
	root := e.original
	if root != nil {
		if issue := root.Input.Linear; issue != nil {
			task.Identifier, task.Title, task.URL = issue.Identifier, issue.Title, issue.URL
		} else {
			task.Title, task.Repository = root.Input.Task, &root.Input.Repository
		}
		if root.Result != nil {
			task.PullRequests, task.Subtasks = root.Result.PullRequests, root.Result.Subtasks
		}
		if len(root.Subtasks) > 0 {
			task.Subtasks = root.Subtasks
		}
	}
	if flow := e.workflow; flow != nil {
		task.Identifier, task.Title, task.URL = flow.Identifier, flow.Title, flow.URL
		task.ProjectName, task.Description, task.LastError = flow.ProjectName, flow.Description, flow.LastError
		task.Repositories = flow.Repositories
		if len(flow.Subtasks) != 0 {
			task.Subtasks = flow.Subtasks
		}
		task.UpdatedAt = later(task.UpdatedAt, later(flow.UpdatedAt, flow.CreatedAt))
		if len(task.Runs) == 0 {
			task.WorkerName, task.Codex = localName, flow.Codex
			task.OpenViking = flow.OpenViking
			if flow.CanReadCodexResult {
				task.LocalResultJobID = flow.ID
			}
		}
	}
	slices.SortFunc(task.Runs, func(a, b api.TaskRun) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	if len(task.Runs) > 0 {
		task.WorkerName = task.Runs[0].WorkerName
	}
	if len(task.Subtasks) > 0 {
		task.Subtasks = slices.Clone(task.Subtasks)
		for i := range task.Subtasks {
			task.Subtasks[i].Response = ""
		}
	}
	task.Stage = taskStage(task, e.workflow)
	return task
}

func taskStage(task api.Task, workflow *api.JobSummary) string {
	if workflow != nil && (workflow.Stage == "done" || workflow.Stage == "to_done") {
		return workflow.Stage
	}
	if len(task.Runs) > 0 {
		run := task.Runs[0]
		switch run.State {
		case api.ExecutionWaitingForAnswer, api.ExecutionWaitingToContinue, api.ExecutionUnknown:
			return run.State
		case api.ExecutionQueued, api.ExecutionRunning:
			if run.Revision {
				return "revising"
			}
			if run.State == api.ExecutionQueued || workflow == nil || workflow.Stage == "review" {
				return run.State
			}
		case api.ExecutionFailed:
			if !run.Revision && len(task.PullRequests) == 0 &&
				!slices.ContainsFunc(task.Repositories, func(repo api.JobRepository) bool { return repo.PR != nil }) {
				return run.State
			}
		}
	}
	if workflow != nil {
		return workflow.Stage
	}
	if len(task.PullRequests) > 0 {
		if !slices.ContainsFunc(task.PullRequests, func(pr api.JobPullRequest) bool { return !pr.Merged }) {
			return "to_done"
		}
		return "review"
	}
	if len(task.Runs) > 0 {
		return task.Runs[0].State
	}
	return api.ExecutionQueued
}

func matchesTask(task api.Task, filter api.TaskFilter) bool {
	if filter.Stage != "" && task.Stage != filter.Stage {
		return false
	}
	if filter.WorkerID == "" && filter.Source == "" {
		return true
	}
	if len(task.Runs) == 0 {
		return filter.WorkerID == LocalWorkerID && filter.Source == ""
	}
	return slices.ContainsFunc(task.Runs, func(run api.TaskRun) bool {
		return (filter.WorkerID == "" || filter.WorkerID == run.WorkerID) &&
			(filter.Source == "" || filter.Source == run.Source)
	})
}

func later(left, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}
