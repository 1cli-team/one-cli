package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// ExitStatus preserves application exit codes without terminating the CLI before cleanup.
type ExitStatus struct{ Code int }

func (e *ExitStatus) Error() string { return "child process exited unsuccessfully" }

func RunForwarded(ctx context.Context, child *exec.Cmd) error {
	if err := child.Start(); err != nil {
		return err
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case sig := <-signals:
				_ = child.Process.Signal(sig)
			case <-ctx.Done():
				_ = child.Process.Kill()
				return
			case <-done:
				return
			}
		}
	}()
	err := child.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code := exit.ExitCode()
		if code < 0 {
			code = signalExitCode(exit)
		}
		return &ExitStatus{Code: code}
	}
	return err
}
