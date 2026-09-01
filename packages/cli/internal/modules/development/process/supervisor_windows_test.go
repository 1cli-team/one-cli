//go:build windows

package processorch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunBuiltinWindows_NoEntries_NoOp(t *testing.T) {
	var out bytes.Buffer
	if err := runBuiltin(context.Background(), t.TempDir(), nil, BuiltinOpts{Out: &out}); err != nil {
		t.Fatalf("empty entries should be a no-op: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("empty entries wrote output: %q", out.String())
	}
}

func TestRunBuiltinWindows_PrefixesOutput(t *testing.T) {
	var out bytes.Buffer
	err := runBuiltin(context.Background(), t.TempDir(), []ProcEntry{
		{Name: "api", Cmd: "echo hello-from-windows"},
	}, BuiltinOpts{Out: &out})
	if err != nil {
		t.Fatalf("runBuiltin: %v", err)
	}
	if !strings.Contains(out.String(), "api | hello-from-windows") {
		t.Fatalf("missing prefixed output: %q", out.String())
	}
}

func TestRunBuiltinWindows_ChildFailureStopsSiblings(t *testing.T) {
	start := time.Now()
	err := runBuiltin(context.Background(), t.TempDir(), []ProcEntry{
		{Name: "failure", Cmd: "exit /b 7"},
		{Name: "long", Cmd: "ping -n 30 127.0.0.1 >nul"},
	}, BuiltinOpts{Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected child failure")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Windows Job Object did not stop sibling promptly: %v", elapsed)
	}
}

func TestRunBuiltinWindows_ContextCancelStopsProcessTree(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := runBuiltin(ctx, t.TempDir(), []ProcEntry{
		{Name: "long", Cmd: "ping -n 30 127.0.0.1 >nul"},
	}, BuiltinOpts{Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected context cancellation")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("context cancellation did not stop process tree promptly: %v", elapsed)
	}
}

func TestWindowsSignalError(t *testing.T) {
	err := &windowsSignalError{sig: os.Interrupt}
	if err.ExitCode() != 130 {
		t.Fatalf("interrupt exit code = %d, want 130", err.ExitCode())
	}
	if !IsSignal(err) || IsSignal(fmt.Errorf("ordinary")) {
		t.Fatal("IsSignal classification is incorrect")
	}
}
