package updatecheck

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestUpdateLaunchIsThrottledAndLocked(t *testing.T) {
	withIsolatedCache(t)
	target := filepath.Join(t.TempDir(), "one")
	calls := 0
	launch := func(got, dir string) error {
		if got != target || dir == "" {
			t.Fatalf("launch arguments: %s %s", got, dir)
		}
		calls++
		return nil
	}
	startUpdate("1.0.0", target, launch)
	startUpdate("1.0.0", target, launch)
	if calls != 1 {
		t.Fatalf("launches = %d", calls)
	}
	c, err := loadCache()
	if err != nil || c.Status != "checking" {
		t.Fatalf("cache = %#v, %v", c, err)
	}
	c.LastChecked = time.Now().Add(-48 * time.Hour)
	if err := saveCache(c); err != nil {
		t.Fatal(err)
	}
	path, _ := cachePath()
	lock := flock.New(path + ".lock")
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	startUpdate("1.0.0", target, launch)
	if err := lock.Unlock(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("concurrent updater was launched")
	}
	startUpdate("1.0.0", target, launch)
	if calls != 2 {
		t.Fatal("stale update did not retry")
	}
}

func TestFailedLaunchPreservesInstalledProgramAndRateLimits(t *testing.T) {
	withIsolatedCache(t)
	target := filepath.Join(t.TempDir(), "one")
	if err := os.WriteFile(target, []byte("installed"), 0o700); err != nil {
		t.Fatal(err)
	}
	calls := 0
	launch := func(string, string) error { calls++; return errors.New("access denied") }
	startUpdate("1.0.0", target, launch)
	startUpdate("1.0.0", target, launch)
	c, err := loadCache()
	if err != nil || c.Status != "failed" || c.Error != "access denied" || calls != 1 {
		t.Fatalf("cache=%#v calls=%d err=%v", c, calls, err)
	}
	raw, err := os.ReadFile(target)
	if err != nil || string(raw) != "installed" {
		t.Fatalf("target changed: %q, %v", raw, err)
	}
}

func TestWorkerEntryRejectsDevelopmentBuildsAndInvalidCalls(t *testing.T) {
	withIsolatedCache(t)
	if RunWorker("1.0.0", []string{"--help"}) {
		t.Fatal("ordinary invocation consumed")
	}
	before := buildChannel
	t.Cleanup(func() { buildChannel = before })
	buildChannel = "development"
	if !RunWorker("1.0.0", []string{workerCommand, "/unused", "1"}) {
		t.Fatal("worker invocation not consumed")
	}
	buildChannel = "release"
	if !RunWorker("1.0.0-local.abc1234", []string{workerCommand, "/unused", "1"}) {
		t.Fatal("local invocation not consumed")
	}
	c, err := loadCache()
	if err != nil || c != nil {
		t.Fatalf("disabled worker changed cache: %#v, %v", c, err)
	}
}

func TestDetachedWorkerSurvivesLauncher(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "finished")
	launcher := exec.Command(os.Args[0], "-test.run=^TestDetachedWorkerHelper$")
	launcher.Env = append(os.Environ(), "ONE_UPDATE_TEST_ROLE=launcher", "ONE_UPDATE_TEST_MARKER="+marker)
	if raw, err := launcher.CombinedOutput(); err != nil {
		t.Fatalf("launcher: %v %s", err, raw)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(marker)
		if err == nil {
			if string(raw) != "finished" {
				t.Fatalf("worker result: %q", raw)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("detached updater did not survive its launching command")
}

func TestDetachedWorkerHelper(t *testing.T) {
	switch os.Getenv("ONE_UPDATE_TEST_ROLE") {
	case "launcher":
		worker := exec.Command(os.Args[0], "-test.run=^TestDetachedWorkerHelper$")
		worker.Env = append(os.Environ(), "ONE_UPDATE_TEST_ROLE=worker", "ONE_UPDATE_TEST_PARENT="+strconv.Itoa(os.Getpid()))
		detachWorker(worker)
		if err := worker.Start(); err != nil {
			os.Exit(2)
		}
		_ = worker.Process.Release()
		os.Exit(0)
	case "worker":
		parent, _ := strconv.Atoi(os.Getenv("ONE_UPDATE_TEST_PARENT"))
		wait, closeParent, err := parentExitWaiter(parent)
		if err != nil {
			os.Exit(3)
		}
		if err := wait(); err != nil {
			closeParent()
			os.Exit(4)
		}
		closeParent()
		time.Sleep(100 * time.Millisecond)
		if err := os.WriteFile(os.Getenv("ONE_UPDATE_TEST_MARKER"), []byte("finished"), 0o600); err != nil {
			os.Exit(5)
		}
		os.Exit(0)
	}
}

func TestWorkerDirectoryValidation(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "cache")
	workerDir := filepath.Join(cacheDir, workerPrefix+"test")
	otherDir := filepath.Join(root, "other", workerPrefix+"test")
	unprefixedDir := filepath.Join(cacheDir, "ordinary")
	for _, dir := range []string{workerDir, otherDir, unprefixedDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	check := func(t *testing.T, workerDir, cacheDir string, want bool) {
		t.Helper()
		resolved, valid := validatedWorkerDir(filepath.Join(workerDir, executableName()), cacheDir)
		if valid != want {
			t.Fatalf("worker %q in cache %q: valid = %v, want %v", workerDir, cacheDir, valid, want)
		}
		if valid {
			wantDir, err := filepath.EvalSymlinks(workerDir)
			if err != nil || resolved != wantDir {
				t.Fatalf("worker directory = %q, want %q: %v", resolved, wantDir, err)
			}
		}
	}
	check(t, workerDir, cacheDir, true)
	check(t, otherDir, cacheDir, false)
	check(t, unprefixedDir, cacheDir, false)
	check(t, workerDir, filepath.Join(root, "missing"), false)
	t.Run("path aliases", func(t *testing.T) {
		alias := filepath.Join(root, "alias")
		if err := os.Symlink(cacheDir, alias); err != nil {
			t.Skipf("directory symlinks unavailable: %v", err)
		}
		check(t, filepath.Join(alias, filepath.Base(workerDir)), cacheDir, true)
		check(t, workerDir, alias, true)
		check(t, filepath.Join(alias, filepath.Base(workerDir)), alias, true)
		// A worker-shaped symlink into another directory must remain invalid.
		escaped := filepath.Join(cacheDir, workerPrefix+"outside")
		if err := os.Symlink(otherDir, escaped); err != nil {
			t.Fatal(err)
		}
		check(t, escaped, cacheDir, false)
	})
}
