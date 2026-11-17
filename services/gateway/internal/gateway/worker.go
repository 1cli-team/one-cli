package gateway

import (
	"context"
	"log/slog"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
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

// Supervise keeps the legacy local RPC available independently of the HTTP listener.
type Supervisor struct {
	mu      sync.RWMutex
	current *Worker
}

func (s *Supervisor) Call(ctx context.Context, req api.WorkRequest) (api.WorkReply, error) {
	s.mu.RLock()
	w := s.current
	s.mu.RUnlock()
	if w == nil {
		return api.WorkReply{}, i18n.Errorf("gateway.worker_unavailable")
	}
	return w.Call(ctx, req)
}

func (s *Supervisor) Run(ctx context.Context, one, config string) {
	for ctx.Err() == nil {
		if err := s.run(ctx, one, config); err != nil && ctx.Err() == nil {
			slog.Warn(i18n.T("gateway.worker_unavailable"), "error", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
		}
	}
}

func (s *Supervisor) run(ctx context.Context, one, config string) error {
	codex, err := exec.LookPath("codex")
	if err != nil {
		return err
	}
	socket, closeSocket, err := OpenCodexSocket(codex)
	if err != nil {
		return err
	}
	defer closeSocket()
	w, err := StartWorker(one, config, socket)
	if err != nil {
		return err
	}
	defer w.Close()
	ready, cancel := context.WithTimeout(ctx, 15*time.Second)
	_, err = w.Call(ready, api.WorkRequest{Operation: api.WorkStatus})
	cancel()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.current = w
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.current = nil
		s.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return nil
	case <-w.Done:
		return w.Err()
	}
}
