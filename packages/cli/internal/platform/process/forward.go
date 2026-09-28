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
				if child.Cancel != nil {
					_ = child.Cancel()
				} else {
					_ = child.Process.Kill()
				}
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

// ExitCode preserves signal exits as well as explicit process exit statuses.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var status *ExitStatus
	if errors.As(err, &status) {
		return status.Code
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if code := exit.ExitCode(); code >= 0 {
			return code
		}
		return signalExitCode(exit)
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	return 1
}
