//go:build unix

package taskrun

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

const supportsPTY = true

type childProcess struct {
	cmd  *exec.Cmd
	file *os.File
	once sync.Once
}

func startPTY(task Task, width, height int) (*childProcess, error) {
	cmd := exec.Command(task.Argv[0], task.Argv[1:]...)
	cmd.Dir = task.Directory
	cmd.Env = os.Environ()
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
	if err != nil {
		return nil, err
	}
	return &childProcess{cmd: cmd, file: file}, nil
}
func (p *childProcess) Write(b []byte) (int, error) { return p.file.Write(b) }
func (p *childProcess) Resize(w, h int) error {
	return pty.Setsize(p.file, &pty.Winsize{Cols: uint16(w), Rows: uint16(h)})
}
func (p *childProcess) Close() error {
	var err error
	p.once.Do(func() { err = p.file.Close() })
	return err
}
func (p *childProcess) signal(sig syscall.Signal) { _ = syscall.Kill(-p.cmd.Process.Pid, sig) }
func (p *childProcess) run(ctx context.Context, out io.Writer) error {
	copied := make(chan struct{})
	go func() { defer close(copied); _, _ = io.Copy(out, p.file) }()
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		p.signal(syscall.SIGTERM)
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-done:
		case <-timer.C:
			p.signal(syscall.SIGKILL)
			<-done
		}
		timer.Stop()
		err = ctx.Err()
	}
	// Reap descendants even if the shell exits before its background jobs.
	p.signal(syscall.SIGKILL)
	select {
	case <-copied:
	case <-time.After(time.Second):
		_ = p.Close()
		<-copied
	}
	return err
}
func (s *Session) runRaw() ([]Result, error) {
	defer close(s.done)
	j := s.jobs[0]
	start := time.Now()
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w, h = 80, 24
	}
	p, err := startPTY(j.task, w, h)
	if err != nil {
		return nil, err
	}
	defer p.Close()
	old, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		p.signal(syscall.SIGKILL)
		_ = p.cmd.Wait()
		return nil, err
	}
	defer term.Restore(int(os.Stdin.Fd()), old)
	input, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		p.signal(syscall.SIGKILL)
		_ = p.cmd.Wait()
		return nil, err
	}
	inputDone := make(chan struct{})
	go func() { defer close(inputDone); _, _ = io.Copy(p, input) }()
	defer func() { input.Cancel(); _ = input.Close(); <-inputDone }()
	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	defer signal.Stop(resized)
	stopResize := make(chan struct{})
	defer close(stopResize)
	go func() {
		for {
			select {
			case <-resized:
				if w, h, e := term.GetSize(int(os.Stdout.Fd())); e == nil {
					_ = p.Resize(w, h)
				}
			case <-stopResize:
				return
			}
		}
	}()
	err = p.run(s.ctx, os.Stdout)
	if s.ctx.Err() != nil {
		err = context.Cause(s.ctx)
	}
	result := Result{Name: j.task.Name, Status: "succeeded", Duration: time.Since(start), ExitCode: exitCode(err), Err: err}
	if err != nil {
		result.Status = "failed"
	}
	if err != nil {
		return []Result{result}, statusError(err)
	}
	return []Result{result}, nil
}
