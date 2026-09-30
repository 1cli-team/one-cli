package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

const manualWorkerCommand = "__manual-update"
const manualTimeout = 10 * time.Minute

// Result describes a manual update. On Windows, pending means the verified
// download is ready and will be installed after the invoking command exits.
type Result struct {
	Schema         string `json:"schema"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	TargetPath     string `json:"target_path"`
	Status         string `json:"status"`
}

type manualReply struct {
	Result *Result `json:"result,omitempty"`
	Error  string  `json:"error,omitempty"`
}

// Upgrade explicitly checks the release feed, bypassing the daily throttle,
// CI detection, and output-mode gates. A copied worker shares the background
// updater's lock and verification, and can replace the Windows executable
// after this process releases it.
func Upgrade(ctx context.Context, current string) (*Result, error) {
	if buildChannel != "release" || !isStableRelease(current) {
		return nil, i18n.Errorf("upgrade.release_required", current, installCommand(runtime.GOOS))
	}
	target, err := os.Executable()
	if err == nil {
		target, err = filepath.EvalSymlinks(target)
	}
	if err != nil {
		return nil, err
	}
	result, err := upgradeUsingWorker(ctx, current, target)
	if err != nil {
		return nil, i18n.Errorf("upgrade.failed", target, err, installCommand(runtime.GOOS))
	}
	return result, nil
}

func upgradeUsingWorker(ctx context.Context, current, target string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, manualTimeout)
	defer cancel()
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	cmd, dir, err := prepareWorker(target, filepath.Dir(path), manualWorkerCommand)
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = os.RemoveAll(dir)
		return nil, err
	}
	defer stdout.Close()
	type response struct {
		reply manualReply
		err   error
	}
	replies := make(chan response, 1)
	go func() {
		var reply manualReply
		err := json.NewDecoder(io.LimitReader(stdout, 1<<20)).Decode(&reply)
		replies <- response{reply, err}
	}()
	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = os.RemoveAll(dir)
		return nil, ctx.Err()
	case response := <-replies:
		// A pending Windows worker must outlive its parent; waiting here
		// would deadlock because that worker waits for this process to exit.
		go func() { _ = cmd.Wait() }()
		if response.err != nil {
			return nil, response.err
		}
		if response.reply.Error != "" {
			return nil, errors.New(response.reply.Error)
		}
		r := response.reply.Result
		if r == nil || r.Schema != "one-cli/upgrade/v1" || r.CurrentVersion != current || r.TargetPath != target ||
			!isStableRelease(r.LatestVersion) || (r.Status != "current" && r.Status != "updated" && r.Status != "pending") {
			return nil, i18n.Errorf("upgrade.worker_invalid")
		}
		return r, nil
	}
}

func runManualWorker(version string, args []string, makeUpdater func() updater) bool {
	if len(args) == 0 || args[0] != manualWorkerCommand {
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
	dir, valid := validatedWorkerDir(worker, filepath.Dir(path))
	if !valid {
		return true
	}
	defer os.RemoveAll(dir)
	parent, err := strconv.Atoi(args[2])
	if err != nil || parent <= 0 || !filepath.IsAbs(args[1]) {
		return true
	}
	initWorkerLocale()
	sent := false
	send := func(result *Result, err error) error {
		reply := manualReply{Result: result}
		if err != nil {
			reply.Error = err.Error()
		}
		if err := json.NewEncoder(os.Stdout).Encode(reply); err != nil {
			return err
		}
		sent = true
		return nil
	}
	expected, err := executableDigest(worker)
	if err != nil {
		_ = send(nil, err)
		return true
	}
	wait, closeParent, err := parentExitWaiter(parent)
	if err != nil {
		_ = send(nil, err)
		return true
	}
	defer closeParent()
	ctx, cancel := context.WithTimeout(context.Background(), manualTimeout)
	defer cancel()
	u := makeUpdater()
	if u.goos == "windows" {
		u.beforeInstall = func(latest string) error {
			if err := send(&Result{Schema: "one-cli/upgrade/v1", CurrentVersion: version, LatestVersion: latest,
				TargetPath: args[1], Status: "pending"}, nil); err != nil {
				return err
			}
			return wait()
		}
	}
	result, err := upgradeLocked(ctx, u, args[1], version, expected)
	if !sent {
		_ = send(result, err)
	}
	return true
}

func upgradeLocked(ctx context.Context, u updater, target, current, expected string) (*Result, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	lock := flock.New(path + ".lock")
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, i18n.Errorf("upgrade.worker_invalid")
	}
	defer lock.Unlock()
	c := &Cache{LastChecked: time.Now().UTC(), CurrentVersion: current, TargetPath: target, Status: "checking"}
	if err := saveCache(c); err != nil {
		return nil, err
	}
	latest, installed, err := u.update(ctx, target, current, expected)
	c.LatestVersion, c.InstalledVersion = latest, installed
	c.Status = "current"
	if err != nil {
		c.Status, c.Error = "failed", err.Error()
	}
	if installed != "" {
		c.Status = "updated"
		c.NotificationPending = u.goos == "windows"
	}
	// A cache write failure must not turn an installed upgrade into a failure.
	_ = saveCache(c)
	if err != nil {
		return nil, err
	}
	return &Result{Schema: "one-cli/upgrade/v1", CurrentVersion: current, LatestVersion: latest,
		TargetPath: target, Status: c.Status}, nil
}
