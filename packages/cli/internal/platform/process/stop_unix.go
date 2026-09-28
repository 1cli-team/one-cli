//go:build !windows

package process

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// StopTree also stops descendants which created their own process groups (mise
// does this for each task). Snapshot while the parent still owns those children.
func StopTree(parent *os.Process) error {
	raw, err := exec.Command("ps", "-axo", "pid=,ppid=").Output()
	children := map[int][]int{}
	if err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			pid, e1 := strconv.Atoi(fields[0])
			ppid, e2 := strconv.Atoi(fields[1])
			if e1 == nil && e2 == nil {
				children[ppid] = append(children[ppid], pid)
			}
		}
	}
	var descendants []int
	var collect func(int)
	collect = func(pid int) {
		for _, child := range children[pid] {
			descendants = append(descendants, child)
			collect(child)
		}
	}
	collect(parent.Pid)
	// Stop parents before children so the scheduler cannot spawn more tasks.
	result := parent.Kill()
	for _, pid := range descendants {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	return result
}
