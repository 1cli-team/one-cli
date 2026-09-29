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
		// mise exec can put the command in another process group. Snapshot
		// descendants before terminating mise, then also kill the original
		// group to cover children spawned during the snapshot.
		err := StopTree(cmd.Process)
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return err
	}
	cmd.WaitDelay = 5 * time.Second
}
