package updatecheck

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/preferences"
)

// Only the release packager overrides this. Setting main.version for a local
// go/task/mise build is deliberately insufficient to enable self-updates.
var buildChannel = "development"

const refreshInterval = 24 * time.Hour
const workerCommand = "__self-update"
const workerPrefix = ".update-worker-"

// MaybeRefreshAsync starts an independent worker at most once a day. It never
// waits for network I/O, and the worker survives short commands such as --help.
func MaybeRefreshAsync(currentVersion string) {
	if shouldSkip(currentVersion) {
		return
	}
	target, err := os.Executable()
	if err != nil {
		return
	}
	startUpdate(currentVersion, target, launchWorker)
}

func freshFor(c *Cache, version, target string) bool {
	return c != nil && c.TargetPath == target &&
		(c.CurrentVersion == version || c.InstalledVersion != "" && c.InstalledVersion == normalizeTag(version)) &&
		time.Since(c.LastChecked) < refreshInterval
}

func startUpdate(version, target string, launch func(string, string) error) {
	path, err := cachePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	lock := flock.New(path + ".lock")
	locked, err := lock.TryLock()
	if err != nil || !locked {
		return
	}
	defer lock.Unlock()
	previous, _ := loadCache()
	if freshFor(previous, version, target) {
		return
	}
	c := &Cache{LastChecked: time.Now().UTC(), CurrentVersion: version, TargetPath: target, Status: "checking"}
	if previous != nil {
		c.LatestVersion = previous.LatestVersion
	}
	// Publish the attempt before starting the worker. A failed launch is also
	// rate-limited, so a read-only install never delays every command.
	if err := saveCache(c); err != nil {
		return
	}
	if err := launch(target, filepath.Dir(path)); err != nil {
		c.Status, c.Error = "failed", err.Error()
		_ = saveCache(c)
	}
}

func launchWorker(target, cacheDir string) error {
	cmd, dir, err := prepareWorker(target, cacheDir, workerCommand)
	if err != nil {
		return err
	}
	// Nil streams map to the null device, never to a user's terminal or pipe.
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func prepareWorker(target, cacheDir, command string) (*exec.Cmd, string, error) {
	cleanupWorkers(cacheDir)
	dir, err := os.MkdirTemp(cacheDir, workerPrefix)
	if err != nil {
		return nil, "", err
	}
	worker := filepath.Join(dir, executableName())
	// A separate executable lets Windows install after the invoking process
	// exits without the updater itself keeping the target binary locked.
	if err := copyExecutable(target, worker); err != nil {
		_ = os.RemoveAll(dir)
		return nil, "", err
	}
	cmd := exec.Command(worker, command, target, strconv.Itoa(os.Getpid()))
	detachWorker(cmd)
	return cmd, dir, nil
}

func cleanupWorkers(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), workerPrefix) {
			continue
		}
		info, err := entry.Info()
		if err == nil && time.Since(info.ModTime()) > refreshInterval {
			// On Windows, a running worker's executable cannot be deleted;
			// stale workers from previous runs can be reclaimed here.
			_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
		}
	}
}

// RunWorker consumes the private worker invocation before Cobra, prompts, or
// application services are initialized. Non-release builds always do nothing.
func RunWorker(version string, args []string) bool {
	if runManualWorker(version, args, defaultUpdater) {
		return true
	}
	return runWorker(version, args, defaultUpdater)
}

func runWorker(version string, args []string, makeUpdater func() updater) bool {
	if len(args) == 0 || args[0] != workerCommand {
		return false
	}
	if len(args) != 3 || buildChannel != "release" || !isStableRelease(version) {
		return true
	}
	worker, err := os.Executable()
	if err != nil {
		return true
	}
	path, err := cachePath()
	if err != nil {
		return true
	}
	workerDir, valid := validatedWorkerDir(worker, filepath.Dir(path))
	if !valid {
		return true
	}
	defer os.RemoveAll(workerDir)
	target := args[1]
	parent, err := strconv.Atoi(args[2])
	if err != nil || parent <= 0 || !filepath.IsAbs(target) {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	lock := flock.New(path + ".lock")
	locked, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil || !locked {
		return true
	}
	defer lock.Unlock()
	c, err := loadCache()
	if err != nil || c == nil || c.TargetPath != target || c.CurrentVersion != version || c.Status != "checking" {
		return true
	}
	// Only replace the exact executable that was copied by the parent. A user
	// rebuilding or reinstalling while we download always wins.
	initWorkerLocale()
	expected, err := executableDigest(worker)
	if err == nil {
		updater := makeUpdater()
		var closeParent func()
		var wait func() error
		wait, closeParent, err = parentExitWaiter(parent)
		if err != nil {
			c.Status, c.Error = "failed", err.Error()
			_ = saveCache(c)
			return true
		}
		defer closeParent()
		updater.beforeInstall = func(string) error { return wait() }
		var latest string
		latest, c.InstalledVersion, err = updater.update(ctx, target, version, expected)
		if latest != "" {
			c.LatestVersion = latest
		}
	}
	c.Status = "current"
	if err != nil {
		c.Status, c.Error = "failed", err.Error()
	}
	if c.InstalledVersion != "" {
		c.Status, c.NotificationPending = "updated", true
	}
	_ = saveCache(c)
	return true
}

func initWorkerLocale() {
	storedLocale := preferences.LocaleAuto
	if prefs, _ := preferences.Load(); prefs != nil {
		storedLocale = prefs.Locale
	}
	_ = i18n.Init(i18n.Resolve(storedLocale))
}

// validatedWorkerDir only accepts a copied worker directly inside the cache.
func validatedWorkerDir(worker, cacheDir string) (string, bool) {
	// Executable paths may retain /var aliases on macOS or short names on
	// Windows. Resolve the worker before checking its directory name, then
	// compare directory identities instead of platform-dependent spellings.
	workerDir, err := filepath.EvalSymlinks(filepath.Dir(worker))
	if err != nil || !strings.HasPrefix(filepath.Base(workerDir), workerPrefix) {
		return "", false
	}
	parentInfo, err := os.Stat(filepath.Dir(workerDir))
	if err != nil {
		return "", false
	}
	cacheInfo, err := os.Stat(cacheDir)
	if err != nil || !cacheInfo.IsDir() || !os.SameFile(parentInfo, cacheInfo) {
		return "", false
	}
	return workerDir, true
}
