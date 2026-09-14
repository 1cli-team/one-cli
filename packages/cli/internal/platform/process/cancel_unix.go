//go:build !windows

package process

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// CancelProcessTree prevents cancelled preparation from leaving Go compilers,
// package managers or lifecycle scripts running after the supervisor exits.
func CancelProcessTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 5 * time.Second
}
