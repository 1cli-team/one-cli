package updatecheck

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// withTTY forces output mode to TTY for the duration of the test so the
// IsTTY-based gate doesn't reject the notification path. Resets to
// ModeAuto on cleanup — the package's exported zero default — since
// internal/platform/output doesn't surface a getter for the current mode.
func withTTY(t *testing.T) {
	t.Helper()
	originalChannel := buildChannel
	buildChannel = "release"
	t.Cleanup(func() { buildChannel = originalChannel })
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
	withTTY(t)
	clearCIEnv(t)
	output.SetMode(output.ModeJSON)
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
	if !shouldSkip("v0.8.0") {
		t.Errorf("expected skip when output mode is JSON (non-TTY)")
	}
}

var developmentVersions = []string{
	"", "dev", "unknown", "0.0.0", "v0.0.0", "0.0.0-dev",
	"0.0.0-dev-local.abc1234", "0.0.0-dev-local.abc1234.dirty",
	"1.2.3-local.abc1234", "1.2.3-local.abc1234.dirty",
	"v1.2.3-dev", "1.2.3-SNAPSHOT-abc1234", "v1.2.3-rc.1",
	"1.2.3+local.abc1234", "v1", "1.2", "01.2.3", "1.2.3.4",
}

func TestShouldSkip_DevVersion(t *testing.T) {
	withTTY(t)
	clearCIEnv(t)
	for _, version := range developmentVersions {
		if !shouldSkip(version) {
			t.Errorf("expected skip for development/unknown build %q", version)
		}
	}
}

func TestDevelopmentBuildsNeverRefreshOrNotify(t *testing.T) {
	withTTY(t)
	clearCIEnv(t)
	withIsolatedCache(t)
	cached := &Cache{LastChecked: time.Now().Add(-48 * time.Hour).UTC(), LatestVersion: "v99.0.0"}
	if err := saveCache(cached); err != nil {
		t.Fatal(err)
	}
	path, err := cachePath()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range developmentVersions {
		MaybeRefreshAsync(version)

		if got := captureStderr(t, func() { Notify(version) }); got != "" {
			t.Fatalf("development build %q printed an update: %q", version, got)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("development update check rewrote the cache")
	}
}

func TestShouldSkip_HappyPath(t *testing.T) {
	withTTY(t)
	clearCIEnv(t)
	for _, version := range []string{"v0.8.0", "0.8.0", "1.2.3", "v10.20.30"} {
		if shouldSkip(version) {
			t.Errorf("expected no skip for stable release %q", version)
		}
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
	for _, want := range []string{"v0.9.0", "v0.8.0", installCommand(runtime.GOOS), "⚠"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning missing %q\n  got: %s", want, got)
		}
	}
}

func TestInstallCommandMatchesPlatform(t *testing.T) {
	if got := installCommand("windows"); got != windowsInstallCommand {
		t.Fatalf("Windows install command = %q", got)
	}
	if got := installCommand("linux"); got != unixInstallCommand {
		t.Fatalf("Unix install command = %q", got)
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

func TestDevelopmentBuildWithReleaseVersionStillSkips(t *testing.T) {
	withTTY(t)
	clearCIEnv(t)
	buildChannel = "development"
	if !shouldSkip("1.2.3") {
		t.Fatal("version override enabled updates in a source build")
	}
}

func TestNotifyInstalledUpdateOnceInBothLanguages(t *testing.T) {
	withTTY(t)
	clearCIEnv(t)
	withIsolatedCache(t)
	target, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	for _, locale := range []string{"en-US", "zh-CN"} {
		if err := i18n.Init(locale); err != nil {
			t.Fatal(err)
		}
		c := &Cache{LastChecked: time.Now(), TargetPath: target, CurrentVersion: "1.0.0", LatestVersion: "v1.2.3", InstalledVersion: "v1.2.3", Status: "updated", NotificationPending: true}
		if err := saveCache(c); err != nil {
			t.Fatal(err)
		}
		got := captureStderr(t, func() { Notify("1.2.3") })
		if !strings.Contains(got, i18n.Tf("update.installed", "v1.2.3")) {
			t.Fatalf("%s notification: %q", locale, got)
		}
		if got := captureStderr(t, func() { Notify("1.2.3") }); got != "" {
			t.Fatalf("notification repeated: %q", got)
		}
	}
}
