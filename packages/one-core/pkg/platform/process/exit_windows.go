//go:build windows

package process

import "os/exec"

func signalExitCode(_ *exec.ExitError) int { return 1 }
