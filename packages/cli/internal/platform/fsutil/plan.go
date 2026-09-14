package fsutil

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

// FilePlan stages related configuration writes and checks all read inputs
// before publishing. Callers hold WorkspaceLock across planning and applying.
type FilePlan struct {
	Root   string
	inputs map[string][]byte
	writes map[string][]byte
	modes  map[string]os.FileMode
}

func NewFilePlan(root string) *FilePlan {
	return &FilePlan{Root: root, inputs: map[string][]byte{}, writes: map[string][]byte{}, modes: map[string]os.FileMode{}}
}

func (p *FilePlan) path(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(p.Root, filepath.FromSlash(path))
}

// Read also accepts absolute paths for read-only inputs such as external
// members of a user's go.work. Missing files return nil, nil.
func (p *FilePlan) Read(path string) ([]byte, error) {
	path = p.path(path)
	if b, ok := p.writes[path]; ok {
		return b, nil
	}
	if b, ok := p.inputs[path]; ok {
		return b, nil
	}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	p.inputs[path] = b
	return b, nil
}

// Expect imports a read snapshot from a cooperating planner. This prevents
// a file changed between planning and Set from being mistaken for user input
// that was already validated by that planner.
func (p *FilePlan) Expect(path string, expected []byte) error {
	path = p.path(path)
	if previous, ok := p.inputs[path]; ok && (!bytes.Equal(previous, expected) || (previous == nil) != (expected == nil)) {
		return fmt.Errorf("%s changed between workspace plans; retry", path)
	}
	p.inputs[path] = bytes.Clone(expected)
	return nil
}

func (p *FilePlan) Set(path string, b []byte, mode os.FileMode) error {
	path = p.path(path)
	if err := SafeWritePath(p.Root, path); err != nil {
		return err
	}
	if _, err := p.Read(path); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	p.writes[path], p.modes[path] = b, mode
	return nil
}

// Remove stages deletion with the same snapshot and rollback guarantees as Set.
func (p *FilePlan) Remove(path string) error {
	return p.Set(path, nil, 0o644)
}

type FileChange struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
	Remove bool   `json:"remove,omitempty"`
}

func (p *FilePlan) Changes() []FileChange {
	changes := []FileChange{}
	for path, after := range p.writes {
		before := p.inputs[path]
		if bytes.Equal(before, after) && (before == nil) == (after == nil) {
			continue
		}
		rel, _ := filepath.Rel(p.Root, path)
		changes = append(changes, FileChange{Path: filepath.ToSlash(rel), Before: string(before), After: string(after), Remove: after == nil})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

// Overlay is suitable for another static planner reading the future files.
func (p *FilePlan) Overlay() map[string][]byte {
	out := map[string][]byte{}
	for path, b := range p.writes {
		rel, _ := filepath.Rel(p.Root, path)
		out[filepath.ToSlash(rel)] = b
	}
	return out
}

func (p *FilePlan) Apply(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for path, expected := range p.inputs {
		actual, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if !bytes.Equal(actual, expected) || (expected == nil) != os.IsNotExist(err) {
			return fmt.Errorf("%s changed while preparing the workspace; retry", path)
		}
	}
	paths := make([]string, 0, len(p.writes))
	for path, b := range p.writes {
		if err := SafeWritePath(p.Root, path); err != nil {
			return err
		}
		if !bytes.Equal(b, p.inputs[path]) || (b == nil) != (p.inputs[path] == nil) {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	applied := []string{}
	for _, path := range paths {
		err := ctx.Err()
		if err == nil {
			if p.writes[path] == nil {
				err = os.Remove(path)
			} else {
				err = WriteAtomic(path, p.writes[path], p.modes[path])
			}
		}
		if err != nil {
			failures := []error{err}
			for i := len(applied) - 1; i >= 0; i-- {
				path := applied[i]
				current, readErr := os.ReadFile(path)
				if (readErr != nil && !os.IsNotExist(readErr)) || !bytes.Equal(current, p.writes[path]) || (p.writes[path] == nil) != os.IsNotExist(readErr) {
					failures = append(failures, fmt.Errorf("cannot restore concurrently modified %s", path))
					continue
				}
				var restoreErr error
				if p.inputs[path] == nil {
					restoreErr = os.Remove(path)
				} else {
					restoreErr = WriteAtomic(path, p.inputs[path], p.modes[path])
				}
				if restoreErr != nil {
					failures = append(failures, restoreErr)
				}
			}
			return errors.Join(failures...)
		}
		applied = append(applied, path)
	}
	return nil
}

// SafeWritePath rejects escaping paths and symlink traversal before mutation.
func SafeWritePath(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("configuration path escapes workspace: %s", path)
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("configuration path is a symbolic link: %s", current)
		}
	}
	return nil
}

func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".one-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return ReplaceFile(f.Name(), path)
}

// WorkspaceLock leaves no metadata in the user's repository. Different
// purposes can serialize independent resources (creation, node, go).
func WorkspaceLock(ctx context.Context, root, purpose string) (func(), error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	lock := flock.New(filepath.Join(os.TempDir(), fmt.Sprintf("one-%s-%x.lock", purpose, sha256.Sum256([]byte(root)))))
	locked, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, fmt.Errorf("workspace %s is locked", purpose)
	}
	return func() { _ = lock.Unlock() }, nil
}
