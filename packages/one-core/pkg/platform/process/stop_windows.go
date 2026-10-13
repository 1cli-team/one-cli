package process

import (
	"os"
	"os/exec"
	"strconv"
)

func StopTree(parent *os.Process) error {
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(parent.Pid)).Run()
	return parent.Kill()
}
