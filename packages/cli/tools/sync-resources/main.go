// Command sync-resources refreshes the generated assets consumed by go:embed.
// It intentionally uses only the Go standard library so the same Taskfile
// commands work on Windows, macOS, and Linux.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/templatefiles"
)

func main() {
	if len(os.Args) != 2 {
		fatalf("usage: sync-resources <bundled|web>")
	}
	root, err := findRepoRoot()
	if err != nil {
		fatalf("locate repository root: %v", err)
	}
	switch os.Args[1] {
	case "bundled":
		err = syncBundled(root)
	case "web":
		err = syncWeb(root)
	default:
		err = fmt.Errorf("unknown resource set %q", os.Args[1])
	}
	if err != nil {
		fatalf("sync %s: %v", os.Args[1], err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.work not found in current directory or its parents")
		}
		dir = parent
	}
}

func syncBundled(root string) error {
	templates := filepath.Join(root, "packages", "templates")
	bundled := filepath.Join(root, "packages", "cli", "internal", "resources", "bundled")
	if err := os.MkdirAll(bundled, 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(templates, "registry.json"), filepath.Join(bundled, "registry.json")); err != nil {
		return err
	}
	if err := filepath.WalkDir(templates, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(templates, name)
		if err != nil {
			return err
		}
		if templatefiles.Excluded(filepath.ToSlash(rel)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("template symlinks are not supported: %s", name)
		}
		if entry.Name() == templatefiles.PackedGoMod || strings.HasSuffix(entry.Name(), ".hbs") {
			return fmt.Errorf("reserved template filename: %s", name)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := replaceDir(root, templates, filepath.Join(bundled, "_templates"), func(rel string, entry fs.DirEntry) bool {
		return rel != "registry.json" && !templatefiles.Excluded(filepath.ToSlash(rel))
	}, true); err != nil {
		return err
	}

	return replaceDir(root, filepath.Join(root, "packages", "agent-skills", "skills"),
		filepath.Join(bundled, "_skills"), nil, false)
}

func syncWeb(root string) error {
	source := filepath.Join(root, "apps", "dashboard", "dist")
	target := filepath.Join(root, "packages", "cli", "internal", "resources", "bundled", "_web")
	if _, err := os.Stat(filepath.Join(source, "index.html")); err != nil {
		return fmt.Errorf("dashboard build is missing: %w", err)
	}
	return replaceDir(root, source, target, nil, false)
}

// ensureGeneratedTarget prevents a future path typo from turning RemoveAll
// into a broad repository deletion. Every replace target must stay inside the
// generated bundled directory and may not be that directory itself.
func ensureGeneratedTarget(root, target string) error {
	generated, err := filepath.Abs(filepath.Join(root, "packages", "cli", "internal", "resources", "bundled"))
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	prefix := generated + string(filepath.Separator)
	if absTarget == generated || !strings.HasPrefix(absTarget, prefix) {
		return fmt.Errorf("refusing to replace path outside generated resources: %s", absTarget)
	}
	return nil
}

func replaceDir(root, source, target string, include func(string, fs.DirEntry) bool, pack bool) error {
	if err := ensureGeneratedTarget(root, target); err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return copyTree(source, target, include, pack)
}

func copyTree(source, target string, include func(string, fs.DirEntry) bool, pack bool) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel != "." && include != nil && !include(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dest := target
		if rel != "." {
			if pack {
				rel = filepath.FromSlash(templatefiles.PackPath(filepath.ToSlash(rel)))
			}
			dest = filepath.Join(target, rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(dest, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			return copyFile(resolved, dest)
		}
		return copyFile(path, dest)
	})
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
