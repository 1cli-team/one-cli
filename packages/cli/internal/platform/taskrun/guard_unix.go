//go:build !windows

package taskrun

import "os/exec"

func guardProcess(cmd *exec.Cmd) (func(), error) { return func() { _ = cmd.Cancel() }, nil }
