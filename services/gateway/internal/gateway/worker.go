package gateway

import (
	"context"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

type Worker struct {
	client *rpc.Client
	cmd    *exec.Cmd
	Done   chan struct{}
	err    error
}

func StartWorker(executable, config, socket string) (*Worker, error) {
	cmd := exec.Command(executable, "__issue-worker", config, socket)
	cmd.Env = slices.DeleteFunc(os.Environ(), func(value string) bool {
		key, _, _ := strings.Cut(value, "=")
		return strings.EqualFold(key, "ONE_GATEWAY_TOKEN")
	})
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		_ = in.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	worker := &Worker{
		client: jsonrpc.NewClient(&api.WorkerStream{Reader: out, Writer: in, Closer: in}),
		cmd:    cmd, Done: make(chan struct{}),
	}
	go func() {
		worker.err = cmd.Wait()
		close(worker.Done)
	}()
	return worker, nil
}

func (w *Worker) Call(ctx context.Context, req api.WorkRequest) (api.WorkReply, error) {
	return api.CallWorker(ctx, w.client, req)
}

func (w *Worker) Err() error {
	<-w.Done
	return w.err
}

func (w *Worker) Close() {
	_ = w.client.Close()
	select {
	case <-w.Done:
	case <-time.After(5 * time.Second):
		_ = w.cmd.Process.Kill()
		<-w.Done
	}
}
