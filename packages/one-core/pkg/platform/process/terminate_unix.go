//go:build !windows

package process

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type treeProcess struct {
	pid, parent int
	started     string
}

// TerminateTree lets task children forward signals and clean up external
// resources (e.g. simulator apps), then forcibly stops remaining processes.
// It is synchronous so Cmd.Wait cannot finish before descendant cleanup.
func TerminateTree(parent *os.Process, grace time.Duration) error {
	before := processTable()
	root, ok := before[parent.Pid]
	if !ok {
		return StopTree(parent)
	}
	descendants := descendantsOf(before, parent.Pid)
	// Children first: wrappers may exit as soon as they receive SIGTERM.
	for _, child := range descendants {
		_ = syscall.Kill(child.pid, syscall.SIGTERM)
	}
	err := parent.Signal(syscall.SIGTERM)
	descendants = append(descendants, root)
	deadline := time.Now().Add(grace)
	for len(descendants) > 0 && time.Now().Before(deadline) {
		live := descendants[:0]
		for _, p := range descendants {
			if syscall.Kill(p.pid, 0) != syscall.ESRCH {
				live = append(live, p)
			}
		}
		descendants = live
		if len(descendants) > 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	// Recheck identities after the grace period before escalating signals.
	after := processTable()
	// Snapshot new children before killing their parent and losing ancestry.
	if current, ok := after[root.pid]; ok && current.started == root.started {
		descendants = append(descendants, descendantsOf(after, root.pid)...)
	}
	for _, p := range descendants {
		if current, ok := after[p.pid]; ok && current.started == p.started {
			_ = syscall.Kill(p.pid, syscall.SIGKILL)
		}
	}
	_ = parent.Kill()
	return err
}

func descendantsOf(table map[int]treeProcess, parent int) []treeProcess {
	var result []treeProcess
	for _, child := range table {
		if child.parent == parent {
			result = append(result, descendantsOf(table, child.pid)...)
			result = append(result, child)
		}
	}
	return result
}

func processTable() map[int]treeProcess {
	cmd := exec.Command("ps", "-axo", "pid=,ppid=,lstart=")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	raw, _ := cmd.Output()
	result := map[int]treeProcess{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		pid, e1 := strconv.Atoi(fields[0])
		ppid, e2 := strconv.Atoi(fields[1])
		if e1 == nil && e2 == nil {
			result[pid] = treeProcess{pid, ppid, strings.Join(fields[2:], " ")}
		}
	}
	return result
}
