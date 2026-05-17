package skill

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/torchstellar-team/one-cli/packages/cli/pkg/agentskills"
)

// linkOrCopy installs source into dest using the given method.
//   - MethodSymlink: creates an absolute symlink dest → source. If the
//     OS rejects symlinks (Windows without dev mode, e.g.), falls back
//     to a copy and reports the actual method used in the second return.
//   - MethodCopy: always copies the directory tree.
//
// Existing dest entries are removed first (idempotent re-install).
// Parent directories of dest are created on demand.
func linkOrCopy(source, dest string, method agentskills.Method) (agentskills.Method, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return method, err
	}
	if err := removeExisting(dest); err != nil {
		return method, err
	}
	if method == agentskills.MethodSymlink {
		err := os.Symlink(source, dest)
		if err == nil {
			return agentskills.MethodSymlink, nil
		}
		// Fall back to copy on Windows where symlinks need privilege.
		if !shouldFallbackToCopy(err) {
			return method, err
		}
	}
	return agentskills.MethodCopy, copyTree(source, dest)
}

// removeExisting clears whatever is at path: a symlink, a directory,
// or a file. Absent path is fine.
func removeExisting(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	if info.IsDir() {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// shouldFallbackToCopy decides whether a symlink failure is a normal
// "this OS / FS doesn't support symlinks" condition that warrants
// falling back to a copy, vs an error the caller should see.
func shouldFallbackToCopy(_ error) bool {
	// On Windows the privilege check happens before our error reaches
	// us; conservatively fall back to copy on Windows. On Unix-y
	// systems any symlink error is typically a real problem (path
	// missing etc.) so we surface it.
	return runtime.GOOS == "windows"
}

// copyTree recursively copies src into dst. Both must be directories
// (or src can be a single file). File permissions are preserved.
func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst, info.Mode())
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Resolve the target so we don't dangle on copy.
			real, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(real, target)
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
