package updatecheck

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// withTTY forces output mode to TTY for the duration of the test so the
// IsTTY-based gate doesn't reject the notification path. Resets to
// ModeAuto on cleanup — the package's exported zero default — since
// internal/output doesn't surface a getter for the current mode.
func withTTY(t *testing.T) {
	t.Helper()
	output.SetMode(output.ModeTTY)
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
}

// clearCIEnv unsets CI-detection env vars that may be inherited from the
// test runner (especially relevant when running these tests in CI itself).
func clearCIEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "BUILDKITE"} {
		t.Setenv(k, "")
	}
}

func TestShouldSkip_CI(t *testing.T) {
	withTTY(t)
	clearCIEnv(t)
	t.Setenv("CI", "true")
	if !shouldSkip("v0.8.0") {
		t.Errorf("expected skip when CI is set")
	}
}

func TestShouldSkip_NonTTY(t *testing.T) {
	clearCIEnv(t)
	output.SetMode(output.ModeJSON)
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
	if !shouldSkip("v0.8.0") {
		t.Errorf("expected skip when output mode is JSON (non-TTY)")
	}
}

func TestShouldSkip_DevVersion(t *testing.T) {
	withTTY(t)
	clearCIEnv(t)
	if !shouldSkip("0.0.0-dev") {
		t.Errorf("expected skip for dev build")
	}
	if !shouldSkip("") {
		t.Errorf("expected skip for empty version")
	}
}

func TestShouldSkip_HappyPath(t *testing.T) {
	withTTY(t)
	clearCIEnv(t)
	if shouldSkip("v0.8.0") {
		t.Errorf("expected no skip for plain TTY interactive run")
	}
}

func TestPrintWarning_ContainsBothLines(t *testing.T) {
	// Capture stderr by piping through a tmpfile.
	tmp, err := os.CreateTemp(t.TempDir(), "stderr-*.txt")
	if err != nil {
		t.Fatalf("tmp: %v", err)
	}
	defer tmp.Close()
	printWarning(tmp, "v0.9.0", "v0.8.0")
	_, _ = tmp.Seek(0, 0)
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(tmp); err != nil {
		t.Fatalf("read: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"v0.9.0", "v0.8.0", installCommand, "⚠"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning missing %q\n  got: %s", want, got)
		}
	}
}

// Notify is the integration: skip rules + cache read + format. Wires up
// the cache directly so we don't go anywhere near the network.
func TestNotify_PrintsWhenNewerCached(t *testing.T) {
	withTTY(t)
	clearCIEnv(t)
	withIsolatedCache(t)
	// Seed cache with a strictly newer version.
	if err := saveCache(&Cache{
		LastChecked:   time.Now().UTC(),
		LatestVersion: "v0.9.0",
	}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	// Capture stderr by replacing os.Stderr around the call.
	got := captureStderr(t, func() { Notify("v0.8.0") })
	if !strings.Contains(got, "v0.9.0") {
		t.Errorf("expected notification on stderr, got %q", got)
	}
}

func TestNotify_QuietWhenSameOrOlder(t *testing.T) {
	withTTY(t)
	clearCIEnv(t)
	withIsolatedCache(t)
	if err := saveCache(&Cache{
		LastChecked:   time.Now().UTC(),
		LatestVersion: "v0.8.0",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got := captureStderr(t, func() { Notify("v0.8.0") })
	if got != "" {
		t.Errorf("expected silence when versions match, got %q", got)
	}
}

// captureStderr swaps os.Stderr for a pipe, runs fn, and returns the
// captured output. Restores the original on exit.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = orig })

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()

	fn()
	_ = w.Close()
	return <-done
}
