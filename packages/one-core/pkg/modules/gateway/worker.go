package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/rpc"
	"sync"
)

// Work operations are the transport contract between Gateway and One CLI.
type WorkOperation string

const (
	WorkStatus              WorkOperation = "status"
	WorkResolveIssue        WorkOperation = "resolve_issue"
	WorkAnswerIssue         WorkOperation = "answer_issue"
	WorkContinueIssue       WorkOperation = "continue_issue"
	WorkReviseIssue         WorkOperation = "revise_issue"
	WorkMonitorJobs         WorkOperation = "monitor_jobs"
	WorkMonitorTasks        WorkOperation = "monitor_tasks"
	WorkTaskSnapshot        WorkOperation = "task_snapshot"
	WorkCodexModels         WorkOperation = "codex_models"
	WorkCodexJobResult      WorkOperation = "codex_job_result"
	WorkList                WorkOperation = "list"
	WorkDispatch            WorkOperation = "dispatch"
	WorkDispatchWithOptions WorkOperation = "dispatch_with_options"
	WorkRepositories        WorkOperation = "repositories"
	WorkRepository          WorkOperation = "repository"
	WorkMapping             WorkOperation = "mapping"
	WorkSaveMapping         WorkOperation = "save_mapping"
)

const WorkMethod = "OneWorkflow.Call"

type WorkRequest struct {
	Operation         WorkOperation
	WorkerID          string
	Source            string
	Page              int
	TaskFilter        TaskFilter
	JobID             string
	ProjectID         string
	IssueID           string
	SubtaskOrder      []string
	RevisionCommentID string
	RepositoryID      string
	Mapping           ProjectInput
	Codex             CodexOptions
	AskUser           bool
	TrackProgress     bool
	Answer            ExecutionAnswer
	ProgressID        string
}

type WorkReply struct {
	Status int
	Body   json.RawMessage
}

func CallWorker(ctx context.Context, client *rpc.Client, input WorkRequest) (WorkReply, error) {
	var reply WorkReply
	pending := client.Go(WorkMethod, input, &reply, make(chan *rpc.Call, 1))
	select {
	case call := <-pending.Done:
		return reply, call.Error
	case <-ctx.Done():
		return WorkReply{}, ctx.Err()
	}
}

// WorkerStream adapts separate child stdio pipes to one RPC stream.
type WorkerStream struct {
	io.Reader
	io.Writer
	Closer io.Closer
	once   sync.Once
}

func (s *WorkerStream) Close() (err error) {
	s.once.Do(func() { err = s.Closer.Close() })
	return err
}
