//go:build windows

package updatecheck

import (
	"errors"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func detachWorker(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
}

// Open the process handle before downloading so PID reuse cannot make us wait
// for an unrelated process after a slow transfer.
func parentExitWaiter(pid int) (func() error, func(), error) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return func() error { return nil }, func() {}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	wait := func() error {
		_, err := windows.WaitForSingleObject(handle, windows.INFINITE)
		return err
	}
	return wait, func() { _ = windows.CloseHandle(handle) }, nil
}
