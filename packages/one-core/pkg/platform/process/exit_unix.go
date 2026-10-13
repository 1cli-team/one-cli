//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

func signalExitCode(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return 1
}
