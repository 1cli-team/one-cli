package process

import (
	"os"
	"os/exec"
	"strconv"
	"time"
)

func CancelProcessTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 5 * time.Second
}
