//go:build !windows

package updatecheck

import (
	"os/exec"
	"syscall"
)

func detachWorker(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

// Unix permits atomically replacing an executable while older processes run.
func parentExitWaiter(int) (func() error, func(), error) {
	return func() error { return nil }, func() {}, nil
}
