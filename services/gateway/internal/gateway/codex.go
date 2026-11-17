package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

// OpenCodexSocket gives the local One worker access to Gateway's Codex App
// Server transport. The socket is private to this service invocation.
func OpenCodexSocket(executable string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "one-codex-")
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, "app.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		var sessions sync.WaitGroup
		defer sessions.Wait()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			sessions.Go(func() { relayCodex(ctx, executable, conn) })
		}
	}()
	return path, func() {
		cancel()
		_ = listener.Close()
		<-done
		_ = os.RemoveAll(dir)
	}, nil
}

func relayCodex(ctx context.Context, executable string, conn net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReaderSize(conn, 64<<10)
	line, err := reader.ReadSlice('\n')
	var start api.CodexStart
	if err != nil || json.Unmarshal(line, &start) != nil || !filepath.IsAbs(start.Directory) {
		return
	}
	stream, err := api.StartCodex(ctx, executable, start)
	if err != nil {
		_ = json.NewEncoder(conn).Encode(api.CodexReady{Error: err.Error()})
		return
	}
	defer stream.Close()
	if err := json.NewEncoder(conn).Encode(api.CodexReady{}); err != nil {
		return
	}
	_ = conn.SetDeadline(time.Time{})
	forwardDone := make(chan struct{})
	go func() {
		defer close(forwardDone)
		_, _ = io.Copy(stream.Input, reader)
		cancel()
	}()
	_, _ = io.Copy(conn, stream.Output)
	_ = conn.Close()
	cancel()
	<-forwardDone
}
