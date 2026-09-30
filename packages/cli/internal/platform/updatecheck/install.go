package updatecheck

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

const releaseBaseURL = "https://github.com/1cli-team/one-cli/releases/download"
const maxArchiveSize = 256 << 20
const maxExecutableSize = 512 << 20

type updater struct {
	client                *http.Client
	latestURL, releaseURL string
	goos, goarch          string
	probe                 func(context.Context, string, string) error
	beforeInstall         func(string) error
}

func defaultUpdater() updater {
	return updater{
		client: &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || len(via) >= 10 {
				return i18n.Errorf("update.redirect_invalid")
			}
			return nil
		}},
		latestURL: latestEndpoint, releaseURL: releaseBaseURL,
		goos: runtime.GOOS, goarch: runtime.GOARCH,
		probe: probeExecutable,
	}
}

func (u updater) update(ctx context.Context, target, current, expected string) (latest, installed string, err error) {
	if !isStableRelease(current) {
		return "", "", i18n.Errorf("update.release_required")
	}
	if err = unchangedExecutable(target, expected); err != nil {
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", "", err
	}
	archiveName := "one-cli_" + u.goos + "_" + u.goarch
	switch {
	case (u.goos == "linux" || u.goos == "darwin") && (u.goarch == "amd64" || u.goarch == "arm64"):
		archiveName += ".tar.gz"
	case u.goos == "windows" && u.goarch == "amd64":
		archiveName += ".zip"
	default:
		err = i18n.Errorf("update.platform_unsupported", u.goos, u.goarch)
		return
	}
	latest, err = fetchLatestUsing(ctx, u.client, u.latestURL, current)
	if err != nil || !isNewer(latest, current) {
		return
	}
	base := u.releaseURL + "/" + latest + "/"
	checksum, err := u.checksum(ctx, base+"checksums.txt", archiveName)
	if err != nil {
		return latest, "", err
	}
	archive, err := os.CreateTemp(filepath.Dir(target), ".one-download-*")
	if err != nil {
		return latest, "", err
	}
	archivePath := archive.Name()
	defer os.Remove(archivePath)
	hash := sha256.New()
	err = u.download(ctx, base+archiveName, io.MultiWriter(archive, hash), maxArchiveSize)
	closeErr := archive.Close()
	if err != nil {
		return latest, "", err
	}
	if closeErr != nil {
		return latest, "", closeErr
	}
	if hex.EncodeToString(hash.Sum(nil)) != checksum {
		return latest, "", i18n.Errorf("update.checksum_mismatch", archiveName)
	}
	staged, err := os.CreateTemp(filepath.Dir(target), ".one-next-*")
	if err != nil {
		return latest, "", err
	}
	stagedPath := staged.Name()
	// Windows requires an .exe suffix to execute the verified candidate.
	if u.goos == "windows" {
		if err = staged.Close(); err != nil {
			os.Remove(stagedPath)
			return latest, "", err
		}
		os.Remove(stagedPath)
		stagedPath += ".exe"
		staged, err = os.OpenFile(stagedPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return latest, "", err
		}
	}
	defer os.Remove(stagedPath)
	err = extractExecutable(archivePath, u.goos, staged)
	if err == nil {
		err = staged.Chmod(info.Mode().Perm())
	}
	if err == nil {
		err = staged.Sync()
	}
	closeErr = staged.Close()
	if err != nil {
		return latest, "", err
	}
	if closeErr != nil {
		return latest, "", closeErr
	}
	if err = u.probe(ctx, stagedPath, latest); err != nil {
		return latest, "", err
	}
	if err = unchangedExecutable(target, expected); err != nil {
		return latest, "", err
	}
	if u.beforeInstall != nil {
		if err = u.beforeInstall(latest); err != nil {
			return latest, "", err
		}
	}
	if err = ctx.Err(); err != nil {
		return latest, "", err
	}
	if err = unchangedExecutable(target, expected); err != nil {
		return latest, "", err
	}
	if err = fsutil.ReplaceFile(stagedPath, target); err != nil {
		return latest, "", err
	}
	_ = fsutil.SyncDir(filepath.Dir(target))
	return latest, latest, nil
}

func (u updater) download(ctx context.Context, url string, dst io.Writer, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "one-cli/auto-update")
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return i18n.Errorf("update.http_failed", resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return i18n.Errorf("update.download_too_large")
	}
	n, err := io.Copy(dst, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return i18n.Errorf("update.download_too_large")
	}
	return nil
}

func (u updater) checksum(ctx context.Context, url, archive string) (string, error) {
	var buf strings.Builder
	if err := u.download(ctx, url, &buf, 1<<20); err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(strings.NewReader(buf.String()))
	result := ""
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != archive {
			continue
		}
		digest, err := hex.DecodeString(fields[0])
		if err != nil || len(digest) != sha256.Size || result != "" {
			return "", i18n.Errorf("update.checksum_invalid", archive)
		}
		result = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if result == "" {
		return "", i18n.Errorf("update.checksum_invalid", archive)
	}
	return result, nil
}

// Only copy the root executable. Archive paths, permissions, links, and other
// files never become filesystem paths or modify the installation directory.
func extractExecutable(archive, goos string, dst io.Writer) error {
	name := "one"
	found := false
	copyEntry := func(reader io.Reader, size int64) error {
		if found || size <= 0 || size > maxExecutableSize {
			return i18n.Errorf("update.archive_invalid")
		}
		found = true
		n, err := io.Copy(dst, io.LimitReader(reader, maxExecutableSize+1))
		if err != nil {
			return err
		}
		if n != size {
			return i18n.Errorf("update.archive_invalid")
		}
		return nil
	}
	if goos == "windows" {
		name += ".exe"
		archive, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer archive.Close()
		for _, file := range archive.File {
			if strings.TrimPrefix(file.Name, "./") != name {
				continue
			}
			if !file.Mode().IsRegular() || file.UncompressedSize64 > maxExecutableSize {
				return i18n.Errorf("update.archive_invalid")
			}
			reader, err := file.Open()
			if err != nil {
				return err
			}
			err = copyEntry(reader, int64(file.UncompressedSize64))
			closeErr := reader.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
	} else {
		file, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer file.Close()
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		reader := tar.NewReader(io.LimitReader(gzipReader, maxExecutableSize+1))
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if strings.TrimPrefix(header.Name, "./") != name {
				continue
			}
			if header.Typeflag != tar.TypeReg {
				return i18n.Errorf("update.archive_invalid")
			}
			if err = copyEntry(reader, header.Size); err != nil {
				return err
			}
		}
	}
	if !found {
		return i18n.Errorf("update.archive_invalid")
	}
	return nil
}

func executableName() string {
	if runtime.GOOS == "windows" {
		return "one.exe"
	}
	return "one"
}

func executableDigest(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", i18n.Errorf("update.target_changed", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func unchangedExecutable(path, expected string) error {
	actual, err := executableDigest(path)
	if err != nil {
		return err
	}
	if expected == "" || actual != expected {
		return i18n.Errorf("update.target_changed", path)
	}
	return nil
}

func copyExecutable(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func probeExecutable(ctx context.Context, path, version string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Env = append(os.Environ(), "CI=true")
	raw, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if normalizeTag(string(raw)) != version {
		return i18n.Errorf("update.version_mismatch", strings.TrimSpace(string(raw)), version)
	}
	return nil
}
