package updatecheck

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/installation"
)

// EnsureCompanion repairs upgrades made by historical single-binary updaters.
// It downloads the exact same release archive, verifies its published checksum,
// and installs only the matching server in the user's private data directory.
// It never follows Latest or rewrites the CLI that is already running.
func EnsureCompanion(ctx context.Context, root, version string) (string, error) {
	return defaultUpdater().ensureCompanion(ctx, root, normalizeTag(version))
}
func (u updater) ensureCompanion(ctx context.Context, root, version string) (string, error) {
	if !isStableRelease(version) || !filepath.IsAbs(root) {
		return "", i18n.Errorf("update.release_required")
	}
	name := "one-api"
	if u.goos == "windows" {
		name += ".exe"
	}
	target := filepath.Join(root, "data", "companions", strings.TrimPrefix(version, "v"), u.goos+"-"+u.goarch, name)
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return "", err
	}
	unlock, err := installation.LockTarget(ctx, target)
	if err != nil {
		return "", err
	}
	defer unlock()
	if _, err = os.Stat(target); err == nil {
		if err = u.probe(ctx, target, version); err != nil {
			return "", err
		}
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	ext := ".tar.gz"
	if u.goos == "windows" {
		ext = ".zip"
	}
	if !((u.goos == "darwin" || u.goos == "linux") && (u.goarch == "amd64" || u.goarch == "arm64") || u.goos == "windows" && u.goarch == "amd64") {
		return "", i18n.Errorf("update.platform_unsupported", u.goos, u.goarch)
	}
	archiveName := "one-cli_" + u.goos + "_" + u.goarch + ext
	base := u.releaseURL + "/" + version + "/"
	checksum, err := u.checksum(ctx, base+"checksums.txt", archiveName)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(filepath.Dir(target), ".bootstrap-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	file, err := os.Create(filepath.Join(dir, "archive"))
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	err = u.download(ctx, base+archiveName, io.MultiWriter(file, hash), maxArchiveSize)
	closeErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if hex.EncodeToString(hash.Sum(nil)) != checksum {
		return "", i18n.Errorf("update.checksum_mismatch", archiveName)
	}
	staged := filepath.Join(dir, name)
	candidate, err := os.OpenFile(staged, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0700)
	if err != nil {
		return "", err
	}
	err = extractNamedExecutable(file.Name(), u.goos, "one-api", candidate)
	if err == nil {
		err = candidate.Sync()
	}
	closeErr = candidate.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err = u.probe(ctx, staged, version); err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = fsutil.ReplaceFile(staged, target); err != nil {
		return "", err
	}
	return target, nil
}
