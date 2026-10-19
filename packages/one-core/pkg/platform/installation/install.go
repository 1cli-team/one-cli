// Package installation owns terminal integration, independent of the desktop bundle.
package installation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

type Status struct {
	Target         string `json:"target"`
	Version        string `json:"version"`
	BundledVersion string `json:"bundledVersion"`
	Action         string `json:"action"`
	Digest         string `json:"digest"`
	Installed      bool   `json:"installed"`
	PathReady      bool   `json:"pathReady"`
	ResolvedPath   string `json:"resolvedPath"`
	PathHint       string `json:"pathHint"`
	ConfigPath     string `json:"configPath"`
}

// IsBundled resolves symlinks and checks the sidecar name/layout, not an env flag.
// A standalone one-api sibling is normal CLI packaging, not a Desktop marker.
func IsBundled(path string) bool {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		real = path
	}
	base := strings.TrimSuffix(filepath.Base(real), ".exe")
	_, bundleErr := os.Stat(filepath.Join(filepath.Dir(real), "bundle-manifest.json"))
	return bundleErr == nil || base == "one-api" || base == "one-backend" || strings.HasPrefix(base, "one-backend-") || strings.Contains(filepath.ToSlash(real), ".app/Contents/MacOS/")
}

func DefaultTarget() (string, error) {
	home, err := userdirs.Home()
	if err != nil {
		return "", err
	}
	dir := os.Getenv("ONE_INSTALL_DIR")
	name := "one"
	if runtime.GOOS == "windows" {
		name += ".exe"
		if dir == "" {
			dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "one", "bin")
		}
	} else if dir == "" {
		dir = filepath.Join(home, ".local", "bin")
	}
	return filepath.Abs(filepath.Join(dir, name))
}

func Digest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// LockTarget is also used by one upgrade; resolve the parent so symlink aliases
// cannot acquire independent locks for the same installation.
func LockTarget(ctx context.Context, target string) (func(), error) {
	dir, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, "."+filepath.Base(target)+".install.lock"))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, i18n.Errorf("desktop.install_locked")
	}
	return func() { _ = lock.Unlock() }, nil
}

var stable = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func compare(a, b string) int {
	aa, bb := stable.FindStringSubmatch(a), stable.FindStringSubmatch(b)
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(aa[i])
		y, _ := strconv.Atoi(bb[i])
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

func Inspect(ctx context.Context, target, version string) (Status, error) {
	s := Status{Target: target, BundledVersion: version, Action: "install"}
	s.ConfigPath, _ = preferences.Path()
	if !filepath.IsAbs(target) || IsBundled(target) {
		return s, i18n.Errorf("desktop.install_target")
	}
	resolved, _ := exec.LookPath("one")
	if resolved != "" {
		s.ResolvedPath, _ = filepath.EvalSymlinks(resolved)
	}
	if info, err := os.Lstat(target); err == nil {
		s.Action = "preserve"
		if !info.Mode().IsRegular() {
			return s, nil
		}
		s.Digest, err = Digest(target)
		if err != nil {
			return s, err
		}
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		data, err := exec.CommandContext(probeCtx, target, "--version").Output()
		if err == nil {
			s.Version = strings.TrimSpace(string(data))
			s.Installed = true
		}
		if stable.MatchString(s.Version) && stable.MatchString(version) {
			switch compare(s.Version, version) {
			case -1:
				s.Action = "upgrade"
			case 0:
				s.Action = "keep"
			case 1:
				s.Action = "newer"
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return s, err
	}
	real, _ := filepath.EvalSymlinks(target)
	s.PathReady = s.Installed && real != "" && real == s.ResolvedPath
	s.PathHint = i18n.Tf("desktop.path_hint", filepath.Dir(target))
	return s, nil
}

// Install only applies the reviewed digest. A stale preview never replaces a
// newer file. Unknown, linked and newer installations are preserved.
func Install(ctx context.Context, source, target, version, expected string) (Status, error) {
	return install(ctx, source, target, version, expected, false)
}

// InstallRelease is the verified archive installer's explicit force policy.
// Desktop keeps the conservative preview/digest policy exposed by Install.
func InstallRelease(ctx context.Context, source, target, version string, force bool) (Status, error) {
	current, err := Inspect(ctx, target, version)
	if err != nil {
		return current, err
	}
	return install(ctx, source, target, version, current.Digest, force)
}
func install(ctx context.Context, source, target, version, expected string, force bool) (Status, error) {
	if !filepath.IsAbs(target) || IsBundled(target) {
		return Status{}, i18n.Errorf("desktop.install_target")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Status{}, err
	}
	unlock, err := LockTarget(ctx, target)
	if err != nil {
		return Status{}, err
	}
	defer unlock()
	s, err := Inspect(ctx, target, version)
	if err != nil {
		return s, err
	}
	if s.Digest != expected {
		return s, i18n.Errorf("desktop.install_changed")
	}
	if s.Action != "install" && s.Action != "upgrade" && !force {
		return s, nil
	}
	sourceDigest, err := Digest(source)
	if err != nil {
		return s, err
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	stage, err := os.CreateTemp(filepath.Dir(target), ".one-install-*"+suffix)
	if err != nil {
		return s, err
	}
	staged := stage.Name()
	defer os.Remove(staged)
	in, err := os.Open(source)
	if err != nil {
		stage.Close()
		return s, err
	}
	_, err = io.Copy(stage, in)
	in.Close()
	if err == nil {
		err = stage.Chmod(0o755)
	}
	if err == nil {
		err = stage.Sync()
	}
	closeErr := stage.Close()
	if err != nil {
		return s, err
	}
	if closeErr != nil {
		return s, closeErr
	}
	hash, err := Digest(staged)
	if err != nil {
		return s, err
	}
	if hash != sourceDigest {
		return s, i18n.Errorf("desktop.install_checksum")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	data, err := exec.CommandContext(probeCtx, staged, "--version").Output()
	if err != nil {
		return s, err
	}
	if strings.TrimSpace(string(data)) != version {
		return s, i18n.Errorf("desktop.install_checksum")
	}
	helperOutput, err := exec.CommandContext(probeCtx, Companion(source), "--version").Output()
	if err != nil {
		return s, err
	}
	if strings.TrimSpace(string(helperOutput)) != version {
		return s, i18n.Errorf("desktop.install_checksum")
	}
	// Keep a byte-for-byte recovery copy; do not move away the current executable.
	if s.Digest != "" {
		if err = copyFile(target, target+".previous"); err != nil {
			return s, err
		}
	}
	if err = ctx.Err(); err != nil {
		return s, err
	}
	if err = ReplacePair(staged, Companion(source), target, version); err != nil {
		return s, err
	}
	_ = fsutil.SyncDir(filepath.Dir(target))
	return Inspect(ctx, target, version)
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
