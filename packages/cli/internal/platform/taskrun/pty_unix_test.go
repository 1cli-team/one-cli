//go:build unix

package taskrun

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPTYKeepsTTYColorPartialOutputAndInput(t *testing.T) {
	p, err := startPTY(Task{Argv: []string{"sh", "-c", `test -t 0 && test -t 1 && test -t 2 || exit 9; stty size; printf '\033[31mready\033[0m'; read value; printf '\ranswer:%s\n' "$value"`}}, 91, 27)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var out bytes.Buffer
	_, _ = p.Write([]byte("hello\n"))
	if err := p.run(ctx, &out); err != nil {
		t.Fatalf("%v %q", err, out.String())
	}
	for _, want := range []string{"27 91", "\x1b[31mready\x1b[0m", "answer:hello"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %q", want, out.String())
		}
	}
}
func TestPTYResizeAndTerminalResponses(t *testing.T) {
	s := &Session{width: 80, height: 24}
	j := &job{}
	s.initTerminal(j)
	defer func() { _ = j.terminal.InputPipe().(io.Closer).Close(); <-j.inputDone; _ = j.terminal.Close() }()
	if _, err := (jobOutput{s, j}).Write([]byte("\x1b[31mred\x1b[0m\rnew\x1b[K\n\x1b[6n")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(j.terminal.String(), "new") || strings.Contains(j.terminal.String(), "red") {
		t.Fatal(j.terminal.String())
	}
	j.terminal.Resize(100, 30)
	if j.terminal.Width() != 100 || j.terminal.Height() != 30 {
		t.Fatal("resize failed")
	}
}
func TestPTYCancellationCleansDescendants(t *testing.T) {
	dir := t.TempDir()
	p, err := startPTY(Task{Directory: dir, Argv: []string{"sh", "-c", "sleep 60 & echo $! > child.pid; wait"}}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.run(ctx, io.Discard) }()
	var pid int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		raw, _ := os.ReadFile(filepath.Join(dir, "child.pid"))
		pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
		if pid > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("child not started")
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	cancel()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("cancel hung")
	}
	if syscall.Kill(pid, 0) != syscall.ESRCH {
		raw, _ := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		end := strings.LastIndex(string(raw), ") ")
		if end < 0 || !strings.HasPrefix(string(raw)[end+2:], "Z ") {
			t.Fatalf("descendant %d alive", pid)
		}
	}
}

func TestPTYResizeReachesChild(t *testing.T) {
	p, err := startPTY(Task{Argv: []string{"sh", "-c", "read line; stty size"}}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err = p.Resize(102, 33); err != nil {
		t.Fatal(err)
	}
	_, _ = p.Write([]byte("continue\n"))
	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err = p.run(ctx, &out); err != nil || !strings.Contains(out.String(), "33 102") {
		t.Fatalf("%v %q", err, out.String())
	}
}
