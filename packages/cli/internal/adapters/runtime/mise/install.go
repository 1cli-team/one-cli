package mise

import (
	"archive/tar"
	"archive/zip"
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
)

const maxArchiveSize = 256 << 20
const maxBinarySize = 512 << 20

// installer owns only One's private versioned runtime directory. It never changes
// PATH, shell configuration, an existing mise installation, or config trust.
type installer struct {
	root       string
	legacyRoot string
	downloader downloader
	out        io.Writer
}

func (i installer) ensure(ctx context.Context, a releaseAsset) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	dir := filepath.Join(i.root, managedVersion, a.Platform)
	target := filepath.Join(dir, a.BinaryName())
	if err := verifyExecutable(target, a.BinarySHA256); err == nil {
		return target, nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
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
	if i.legacyRoot != "" {
		legacy := filepath.Join(i.legacyRoot, managedVersion, a.Platform, a.BinaryName())
		if verifyExecutable(legacy, a.BinarySHA256) == nil {
			if i.out != nil {
				fmt.Fprintf(i.out, "[one] Migrating verified mise %s to %s.\n", managedVersion, dir)
			}
			return target, publishBinary(ctx, target, a.BinarySHA256, func(dest io.Writer) error {
				f, err := os.Open(legacy)
				if err != nil {
					return err
				}
				defer f.Close()
				return copyLimited(contextWriter{ctx: ctx, dest: dest}, f, maxBinarySize)
			})
		}
	}
	if i.out != nil {
		fmt.Fprintf(i.out, "[one] Downloading official mise %s (%s) into %s.\n", managedVersion, a.Platform, dir)
	}
	archive, err := i.downloader.fetch(ctx, dir, a)
	if err != nil {
		return "", err
	}
	defer os.Remove(archive)
	return target, publishBinary(ctx, target, a.BinarySHA256, func(dest io.Writer) error {
		return extractBinary(ctx, archive, a, dest)
	})
}

func publishBinary(ctx context.Context, target, digest string, write func(io.Writer) error) error {
	dir := filepath.Dir(target)
	binary, err := os.CreateTemp(dir, ".extract-*")
	if err != nil {
		return err
	}
	defer os.Remove(binary.Name())
	err = write(binary)
	if err == nil {
		err = binary.Sync()
	}
	closeErr := binary.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := verifyFile(binary.Name(), digest, maxBinarySize); err != nil {
		return fmt.Errorf("mise executable verification failed: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Chmod(binary.Name(), 0o755); err != nil {
		return err
	}
	if err := fsutil.ReplaceFile(binary.Name(), target); err != nil {
		return err
	}
	return fsutil.SyncDir(dir)
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
func extractBinary(ctx context.Context, archive string, a releaseAsset, dest io.Writer) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	dest = contextWriter{ctx: ctx, dest: dest}
	match := func(name string) bool {
		name = strings.TrimPrefix(name, "./")
		return name == "mise/bin/"+a.BinaryName() || name == a.BinaryName()
	}
	if a.Format == "zip" {
		info, err := f.Stat()
		if err != nil {
			return err
		}
		z, err := zip.NewReader(f, info.Size())
		if err != nil {
			return err
		}
		for _, entry := range z.File {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !match(entry.Name) || !entry.Mode().IsRegular() {
				continue
			}
			if entry.UncompressedSize64 > maxBinarySize {
				return fmt.Errorf("mise executable exceeds size limit")
			}
			r, err := entry.Open()
			if err != nil {
				return err
			}
			defer r.Close()
			return copyLimited(dest, r, maxBinarySize)
		}
	} else {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			entry, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if match(entry.Name) && entry.Typeflag == tar.TypeReg {
				if entry.Size > maxBinarySize {
					return fmt.Errorf("mise executable exceeds size limit")
				}
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
