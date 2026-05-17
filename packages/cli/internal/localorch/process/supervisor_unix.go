//go:build unix

package processorch

// supervisor_unix.go implements the built-in Procfile runner for Unix
// platforms (darwin, linux, *bsd). Each entry is exec'd as `sh -c <cmd>`
// in a fresh process group (Setpgid=true) so npm/node/pnpm grand-
// children can be SIGTERM'd as a group during shutdown.
//
// Shutdown sequence mirrors foreman/hivemind defaults:
//   1. First child to exit OR external SIGINT/SIGTERM triggers shutdown.
//   2. SIGTERM is sent to every running child's process group.
//   3. After BuiltinOpts.GracePeriod, SIGKILL cleans up stragglers.
//   4. runBuiltin returns the first non-zero exit error encountered, or
//      nil if every child exited cleanly.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// isStdoutTTY returns true when stdout is a real terminal AND the
// active output mode is human-friendly (not JSON/YAML). Used to gate
// ANSI coloring in the supervisor's prefixed output.
func isStdoutTTY() bool { return output.IsTTY() }

// defaultGracePeriod is the SIGTERM → SIGKILL window when callers
// don't override it.
const defaultGracePeriod = 5 * time.Second

// runBuiltin runs every entry as a child process in parallel. Blocks
// until all children exit AND all output streams have drained. See
// package doc for shutdown semantics.
func runBuiltin(ctx context.Context, projectRoot string, entries []ProcEntry, opts BuiltinOpts) error {
	if len(entries) == 0 {
		return nil
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.GracePeriod <= 0 {
		opts.GracePeriod = defaultGracePeriod
	}

	// Single mutex on the output writer — guarantees prefixed lines from
	// concurrent children don't tear.
	var writeMu sync.Mutex
	writeLine := func(prefix, line string) {
		writeMu.Lock()
		defer writeMu.Unlock()
		fmt.Fprintf(opts.Out, "%s | %s\n", prefix, line)
	}

	width := maxNameLen(entries)
	// Only colorise when the writer is the real stdout AND that file is
	// a TTY. Piping `one dev | tee log` should produce uncoloured ANSI-
	// free output for clean log files.
	useColor := opts.Out == os.Stdout && isStdoutTTY()

	type running struct {
		entry ProcEntry
		cmd   *exec.Cmd
		out   *prefixLineWriter
		err   *prefixLineWriter
		// done closes when this child's Wait returns.
		done chan error
	}

	var (
		procs   []*running
		startMu sync.Mutex
	)

	// Helper to broadcast SIGTERM/SIGKILL to every still-running child's
	// process group. Safe to call repeatedly.
	terminate := func(sig syscall.Signal) {
		startMu.Lock()
		defer startMu.Unlock()
		for _, p := range procs {
			if p.cmd.Process == nil {
				continue
			}
			// Setpgid=true → child's pgid equals its pid. Kill the
			// negative value targets the entire group, so any grand-
			// children spawned by npm/node/pnpm die too.
			_ = syscall.Kill(-p.cmd.Process.Pid, sig)
		}
	}
	waitStarted := func() {
		startMu.Lock()
		started := append([]*running(nil), procs...)
		startMu.Unlock()
		for _, p := range started {
			<-p.done
		}
	}

	// Start every entry. If any Start fails, tear down the ones that
	// already started.
	for _, e := range entries {
		cmd := exec.Command("sh", "-c", e.Cmd)
		cmd.Dir = projectRoot
		cmd.Env = os.Environ()
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		prefix := decoratePrefix(padName(e.Name, width), len(procs), useColor)
		stdout := newPrefixLineWriter(prefix, writeLine)
		stderr := newPrefixLineWriter(prefix, writeLine)
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		if err := cmd.Start(); err != nil {
			terminate(syscall.SIGTERM)
			waitStarted()
			return fmt.Errorf("启动 %s 失败: %w", e.Name, err)
		}

		r := &running{entry: e, cmd: cmd, out: stdout, err: stderr, done: make(chan error, 1)}

		startMu.Lock()
		procs = append(procs, r)
		startMu.Unlock()

		go func(rr *running) {
			err := rr.cmd.Wait()
			rr.out.Flush()
			rr.err.Flush()
			rr.done <- err
		}(r)
	}

	// Trap external signals — convert them into a ctx cancel.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Merge every per-proc done channel into one. The first event that
	// arrives — child exit, external signal, or ctx cancel — flips us
	// into shutdown mode.
	type exitEvent struct {
		proc *running
		err  error
	}
	exitCh := make(chan exitEvent, len(procs))
	for _, p := range procs {
		go func(pp *running) {
			exitCh <- exitEvent{proc: pp, err: <-pp.done}
		}(p)
	}

	var firstErr error
	exited := map[*running]bool{}

	select {
	case <-ctx.Done():
		firstErr = ctx.Err()
	case sig := <-sigCh:
		// Wrap into an error with signal-aware exit code semantics. The
		// caller (cmdgate / Start) maps to process exit code; for
		// SIGINT that's 130, SIGTERM 143.
		firstErr = &signalError{sig: sig.(syscall.Signal)}
	case ev := <-exitCh:
		exited[ev.proc] = true
		firstErr = ev.err
	}

	// Shutdown: SIGTERM everyone, wait grace, SIGKILL holdouts.
	terminate(syscall.SIGTERM)

	graceTimer := time.NewTimer(opts.GracePeriod)
	defer graceTimer.Stop()

	remaining := len(procs) - len(exited)
	for remaining > 0 {
		select {
		case ev := <-exitCh:
			if !exited[ev.proc] {
				exited[ev.proc] = true
				remaining--
			}
		case <-graceTimer.C:
			terminate(syscall.SIGKILL)
			// Drain whatever's left — SIGKILL is guaranteed.
			for remaining > 0 {
				ev := <-exitCh
				if !exited[ev.proc] {
					exited[ev.proc] = true
					remaining--
				}
			}
		}
	}

	return firstErr
}

// prefixLineWriter collects arbitrary Write chunks into log lines, prefixes
// each complete line, and flushes the final unterminated line after Wait.
// It avoids bufio.Scanner's token limit and lets exec.Cmd own pipe draining.
type prefixLineWriter struct {
	prefix    string
	writeLine func(string, string)
	pending   []byte
}

func newPrefixLineWriter(prefix string, writeLine func(string, string)) *prefixLineWriter {
	return &prefixLineWriter{prefix: prefix, writeLine: writeLine}
}

func (w *prefixLineWriter) Write(p []byte) (int, error) {
	start := 0
	for i, b := range p {
		if b != '\n' {
			continue
		}
		w.pending = append(w.pending, p[start:i]...)
		w.emit()
		start = i + 1
	}
	if start < len(p) {
		w.pending = append(w.pending, p[start:]...)
	}
	return len(p), nil
}

func (w *prefixLineWriter) Flush() {
	if len(w.pending) == 0 {
		return
	}
	w.emit()
}

func (w *prefixLineWriter) emit() {
	line := strings.TrimSuffix(string(w.pending), "\r")
	w.writeLine(w.prefix, line)
	w.pending = w.pending[:0]
}

// signalError is returned by runBuiltin when shutdown was triggered by
// an external SIGINT/SIGTERM. The caller can ExitCode() to decide the
// process exit code.
type signalError struct{ sig syscall.Signal }

func (e *signalError) Error() string {
	return fmt.Sprintf("interrupted by signal: %s", e.sig)
}

// ExitCode maps the signal to the conventional shell exit code
// (128 + signal number).
func (e *signalError) ExitCode() int {
	return 128 + int(e.sig)
}

// IsSignal reports whether err is a signalError (so callers can avoid
// double-logging it as a generic failure).
func IsSignal(err error) bool {
	var se *signalError
	return errors.As(err, &se)
}
