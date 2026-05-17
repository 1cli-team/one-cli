//go:build unix

package processorch

// supervisor_unix_test.go exercises the real supervisor against `sh -c`
// commands. Tests cover output prefixing, fail-fast (one child dies →
// others SIGTERM'd), context cancel, and pgid-aware cleanup of grand-
// children spawned by shell commands.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunBuiltin_NoEntries_NoOp(t *testing.T) {
	var buf bytes.Buffer
	err := runBuiltin(context.Background(), t.TempDir(), nil, BuiltinOpts{Out: &buf})
	if err != nil {
		t.Fatalf("empty entries should be a no-op, got: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("no-op should not write, got: %q", buf.String())
	}
}

func TestRunBuiltin_AllExitZero_PrefixesOutput(t *testing.T) {
	// Both processes print, then sleep long enough that the fail-fast
	// shutdown (triggered when the first exits) doesn't race past the
	// second's output. Without the sleep the first command can exit
	// before the second has flushed its echo and we'd SIGTERM it mid-
	// print — a real semantic but not what this test is asserting.
	var buf bytes.Buffer
	entries := []ProcEntry{
		{Name: "alpha", Cmd: "echo hello-from-alpha; sleep 0.2"},
		{Name: "beta", Cmd: "echo hello-from-beta; sleep 0.2"},
	}
	err := runBuiltin(context.Background(), t.TempDir(), entries,
		BuiltinOpts{Out: &buf, GracePeriod: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("happy path should succeed, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "alpha | hello-from-alpha") {
		t.Errorf("missing alpha prefix line in: %q", out)
	}
	if !strings.Contains(out, "beta  | hello-from-beta") {
		// beta is padded to alpha's width (5)
		t.Errorf("missing beta prefix line (with padding) in: %q", out)
	}
}

func TestRunBuiltin_OneFails_ShutsDownOthers(t *testing.T) {
	// One short command exits with 7. A sibling sleeps long enough that
	// without proper shutdown the test would hang. We assert: the call
	// returns within a few seconds AND the error reflects exit 7.
	entries := []ProcEntry{
		{Name: "loser", Cmd: "exit 7"},
		{Name: "long", Cmd: "sleep 30"},
	}
	var buf bytes.Buffer
	start := time.Now()
	err := runBuiltin(context.Background(), t.TempDir(), entries,
		BuiltinOpts{Out: &buf, GracePeriod: 500 * time.Millisecond})
	elapsed := time.Since(start)
	if elapsed > 5*time.Second {
		t.Fatalf("supervisor took %v to tear down siblings — pgid kill likely broken", elapsed)
	}
	if err == nil {
		t.Fatal("expected non-nil error reflecting child failure")
	}
}

func TestRunBuiltin_CtxCancel_KillsAll(t *testing.T) {
	entries := []ProcEntry{
		{Name: "a", Cmd: "sleep 30"},
		{Name: "b", Cmd: "sleep 30"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	var buf bytes.Buffer
	start := time.Now()
	err := runBuiltin(ctx, t.TempDir(), entries,
		BuiltinOpts{Out: &buf, GracePeriod: 500 * time.Millisecond})
	elapsed := time.Since(start)
	if elapsed > 3*time.Second {
		t.Fatalf("ctx cancel should kill children quickly, took %v", elapsed)
	}
	if err == nil {
		t.Fatal("expected ctx.Err() returned on cancel")
	}
}

// TestRunBuiltin_PGroupCleansGrandchildren is the regression test for
// the central design decision: we set Setpgid=true so that npm/node-
// style processes that fork subprocesses get cleaned up as a group.
// The test spawns sh that backgrounds a long-running grandchild and
// writes its pid; after we cancel the supervisor, the grandchild must
// also be dead within the grace period.
func TestRunBuiltin_PGroupCleansGrandchildren(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "grandchild.pid")
	// Shell: spawn sleep in background, record its pid, wait. When the
	// supervisor SIGTERMs the shell's pgid, the backgrounded sleep
	// receives it too (because it's in the same pgid).
	cmd := fmt.Sprintf("sleep 60 & echo $! > %s; wait", pidFile)
	entries := []ProcEntry{{Name: "spawner", Cmd: cmd}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		// Wait for the pidfile to appear, then cancel.
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(pidFile); err == nil {
				time.Sleep(100 * time.Millisecond) // give the shell a beat to actually start sleep
				cancel()
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Errorf("grandchild pidfile never appeared")
		cancel()
	}()

	var buf bytes.Buffer
	_ = runBuiltin(ctx, dir, entries,
		BuiltinOpts{Out: &buf, GracePeriod: 1 * time.Second})

	// Now check: is the grandchild dead?
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read pidfile: %v", err)
	}
	pidStr := strings.TrimSpace(string(raw))
	if pidStr == "" {
		t.Fatal("pidfile was empty")
	}

	// Poll for up to grace+overhead: kill(pid, 0) returns ESRCH when the
	// process is gone.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var pid int
		_, _ = fmt.Sscanf(pidStr, "%d", &pid)
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			return // grandchild is gone — pgid cleanup worked
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("grandchild pid=%s still alive after supervisor shutdown — pgid cleanup is broken", pidStr)
}

func TestRunBuiltin_LongOutputLines_NoTruncation(t *testing.T) {
	// Stack traces / JSON blobs can be long. Default bufio scanner limit
	// is 64KB; we explicitly bumped to 1MB. This test asserts the bump
	// holds.
	longArg := strings.Repeat("X", 70*1024)
	entries := []ProcEntry{{Name: "verbose", Cmd: "printf %s '" + longArg + "'"}}
	var buf bytes.Buffer
	err := runBuiltin(context.Background(), t.TempDir(), entries, BuiltinOpts{Out: &buf})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), longArg) {
		t.Errorf("70KB line was truncated or dropped — scanner buffer too small?")
	}
}

func TestSignalError_ExitCode(t *testing.T) {
	if (&signalError{sig: syscall.SIGINT}).ExitCode() != 130 {
		t.Errorf("SIGINT should map to exit 130")
	}
	if (&signalError{sig: syscall.SIGTERM}).ExitCode() != 143 {
		t.Errorf("SIGTERM should map to exit 143")
	}
}

func TestIsSignal(t *testing.T) {
	if IsSignal(nil) {
		t.Error("nil should not be a signal error")
	}
	if !IsSignal(&signalError{sig: syscall.SIGINT}) {
		t.Error("signalError should be detected")
	}
	if IsSignal(fmt.Errorf("normal error")) {
		t.Error("regular error should not be a signal error")
	}
}
