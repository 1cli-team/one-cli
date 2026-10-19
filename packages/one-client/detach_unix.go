//go:build !windows

package oneclient

import "os/exec"
import "syscall"

func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
