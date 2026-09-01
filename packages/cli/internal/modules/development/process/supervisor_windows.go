//go:build windows

package processorch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runBuiltin executes each manifest command with the native Windows command
// processor. All children share a Job Object configured with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, so stopping one dev session also stops
// descendants launched by npm, pnpm, node, or go.
func runBuiltin(ctx context.Context, projectRoot string, entries []ProcEntry, opts BuiltinOpts) error {
	if len(entries) == 0 {
		return nil
	}
	normalizeBuiltinOpts(&opts)

	job, err := newKillOnCloseJob()
	if err != nil {
		return fmt.Errorf("创建 Windows 进程 Job Object 失败: %w", err)
	}
	defer windows.CloseHandle(job)

	var writeMu sync.Mutex
	writeLine := func(prefix, line string) {
		writePrefixedLine(&writeMu, opts.Out, prefix, line)
	}
	width := maxNameLen(entries)
	useColor := opts.Out == os.Stdout && isStdoutTTY()

	type running struct {
		entry ProcEntry
		cmd   *exec.Cmd
		out   *prefixLineWriter
		err   *prefixLineWriter
		done  chan error
	}
	procs := make([]*running, 0, len(entries))

	terminate := func() {
		_ = windows.TerminateJobObject(job, 1)
	}
	waitStarted := func() {
		for _, proc := range procs {
			<-proc.done
		}
	}

	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	for _, entry := range entries {
		cmd := exec.Command(comspec, "/d", "/s", "/c", entry.Cmd)
		cmd.Dir = projectRoot
		cmd.Env = os.Environ()
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}

		prefix := decoratePrefix(padName(entry.Name, width), len(procs), useColor)
		stdout := newPrefixLineWriter(prefix, writeLine)
		stderr := newPrefixLineWriter(prefix, writeLine)
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		if err := cmd.Start(); err != nil {
			terminate()
			waitStarted()
			return fmt.Errorf("启动 %s 失败: %w", entry.Name, err)
		}
		if err := assignProcessToJob(job, cmd.Process.Pid); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			terminate()
			waitStarted()
			return fmt.Errorf("将 %s 加入 Windows Job Object 失败: %w", entry.Name, err)
		}

		proc := &running{entry: entry, cmd: cmd, out: stdout, err: stderr, done: make(chan error, 1)}
		procs = append(procs, proc)
		go func(p *running) {
			err := p.cmd.Wait()
			p.out.Flush()
			p.err.Flush()
			p.done <- err
		}(proc)
	}

	type exitEvent struct {
		proc *running
		err  error
	}
	exitCh := make(chan exitEvent, len(procs))
	for _, proc := range procs {
		go func(p *running) {
			exitCh <- exitEvent{proc: p, err: <-p.done}
		}(proc)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	var firstErr error
	exited := make(map[*running]bool, len(procs))
	select {
	case <-ctx.Done():
		firstErr = ctx.Err()
	case sig := <-sigCh:
		firstErr = &windowsSignalError{sig: sig}
	case event := <-exitCh:
		exited[event.proc] = true
		firstErr = event.err
	}

	terminate()
	remaining := len(procs) - len(exited)
	for remaining > 0 {
		event := <-exitCh
		if !exited[event.proc] {
			exited[event.proc] = true
			remaining--
		}
	}
	return firstErr
}

func newKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func assignProcessToJob(job windows.Handle, pid int) error {
	handle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(pid),
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return windows.AssignProcessToJobObject(job, handle)
}

type windowsSignalError struct{ sig os.Signal }

func (e *windowsSignalError) Error() string { return fmt.Sprintf("interrupted by signal: %s", e.sig) }
func (e *windowsSignalError) ExitCode() int { return 130 }

func IsSignal(err error) bool {
	var signalErr *windowsSignalError
	return errors.As(err, &signalErr)
}
