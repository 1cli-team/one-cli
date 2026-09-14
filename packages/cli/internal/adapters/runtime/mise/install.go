package mise

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/userdirs"
)

const maxArchiveSize = 256 << 20
const maxBinarySize = 512 << 20

// installer owns only One's private versioned runtime cache. It never changes
// PATH, shell configuration, an existing mise installation, or config trust.
type installer struct {
	root    string
	archive []byte
	out     io.Writer
}

func defaultInstaller() (installer, error) {
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		home, err := userdirs.Home()
		if err != nil {
			return installer{}, err
		}
		cache = filepath.Join(home, ".cache")
	}
	if !filepath.IsAbs(cache) {
		return installer{}, fmt.Errorf("mise runtime cache must have an absolute path")
	}
	return installer{
		root:    filepath.Join(cache, "one", "runtimes", "mise"),
		archive: bundledArchive, out: os.Stderr,
	}, nil
}

func (i installer) ensure(ctx context.Context, a releaseAsset) (string, error) {
	dir := filepath.Join(i.root, managedVersion, a.Platform)
	target := filepath.Join(dir, a.BinaryName())
	if err := verifyExecutable(target, a.BinarySHA256); err == nil {
		return target, nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(i.archive) == 0 {
		return "", fmt.Errorf("this One build has no bundled mise for the current platform")
	}
	if len(i.archive) > maxArchiveSize || fmt.Sprintf("%x", sha256.Sum256(i.archive)) != a.ArchiveSHA256 {
		return "", fmt.Errorf("bundled mise archive SHA256 mismatch")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	lock := flock.New(filepath.Join(dir, ".install.lock"))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return "", err
	}
	if !locked {
		return "", fmt.Errorf("could not lock mise runtime installation")
	}
	defer lock.Unlock()
	// Another process (for example the other half of `one dev`) may have
	// finished installing while we waited for the cross-process lock.
	if err := verifyExecutable(target, a.BinarySHA256); err == nil {
		return target, nil
	}
	if i.out != nil {
		fmt.Fprintf(i.out, "[one] Preparing bundled mise %s (%s); extracting to the One runtime cache.\n", managedVersion, a.Platform)
	}

	binary, err := os.CreateTemp(dir, ".extract-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(binary.Name())
	err = extractBinary(ctx, i.archive, a, binary)
	if err == nil {
		err = binary.Sync()
	}
	closeErr := binary.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err := verifyFile(binary.Name(), a.BinarySHA256, maxBinarySize); err != nil {
		return "", fmt.Errorf("mise executable verification failed: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.Chmod(binary.Name(), 0o755); err != nil {
		return "", err
	}
	if err := fsutil.ReplaceFile(binary.Name(), target); err != nil {
		return "", err
	}
	return target, fsutil.SyncDir(dir)
}

func verifyExecutable(path, expected string) error {
	if err := verifyFile(path, expected, maxBinarySize); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("cached mise is not executable")
	}
	return nil
}

func verifyFile(path, expected string, limit int64) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return fmt.Errorf("invalid cached file: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	if err := copyLimited(hash, f, limit); err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != expected {
		return fmt.Errorf("SHA256 mismatch for %s", filepath.Base(path))
	}
	return nil
}

func copyLimited(dest io.Writer, source io.Reader, limit int64) error {
	n, err := io.Copy(dest, io.LimitReader(source, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return fmt.Errorf("mise archive or executable exceeds size limit")
	}
	return nil
}

// Extract only the expected executable into a caller-owned temporary file.
// Archive paths are never joined to a destination or extracted as directories.
func extractBinary(ctx context.Context, archive []byte, a releaseAsset, dest io.Writer) error {
	dest = contextWriter{ctx: ctx, dest: dest}
	match := func(name string) bool {
		name = strings.TrimPrefix(name, "./")
		return name == "mise/bin/"+a.BinaryName() || name == a.BinaryName()
	}
	if a.Format == "zip" {
		z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return err
		}
		for _, entry := range z.File {
			if !match(entry.Name) || !entry.Mode().IsRegular() {
				continue
			}
			r, err := entry.Open()
			if err != nil {
				return err
			}
			defer r.Close()
			return copyLimited(dest, r, maxBinarySize)
		}
	} else {
		gz, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			entry, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if match(entry.Name) && entry.Typeflag == tar.TypeReg {
				return copyLimited(dest, tr, maxBinarySize)
			}
		}
	}
	return fmt.Errorf("mise archive does not contain the expected executable")
}

// Observe cancellation while inflating large release binaries.
type contextWriter struct {
	ctx  context.Context
	dest io.Writer
}

func (w contextWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.dest.Write(p)
}
