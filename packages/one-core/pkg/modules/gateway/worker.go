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
	WorkMonitorJobs         WorkOperation = "monitor_jobs"
	WorkCodexModels         WorkOperation = "codex_models"
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
	Operation    WorkOperation
	Page         int
	ProjectID    string
	IssueID      string
	RepositoryID string
	Mapping      ProjectInput
	Codex        CodexOptions
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
