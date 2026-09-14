package fsutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestFilePlanChecksEveryInputBeforeWriting(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "source")
	if err := os.WriteFile(input, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewFilePlan(root)
	if _, err := p.Read("source"); err != nil {
		t.Fatal(err)
	}
	if err := p.Set("new", []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte("user update"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err == nil {
		t.Fatal("stale plan applied")
	}
	if _, err := os.Stat(filepath.Join(root, "new")); !os.IsNotExist(err) {
		t.Fatal("stale plan wrote files")
	}
}

func TestFilePlanRefusesSymlinkWrites(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary")
	}
	root, external := t.TempDir(), t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := NewFilePlan(root).Set("linked/file", []byte("should not write"), 0o644); err == nil {
		t.Fatal("accepted symlink traversal")
	}
}

func TestWorkspaceLockCancelsWaitingCallerAndCanBeReacquired(t *testing.T) {
	root := t.TempDir()
	unlock, err := WorkspaceLock(context.Background(), root, "plan-test")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if release, err := WorkspaceLock(ctx, root, "plan-test"); err == nil {
		release()
		t.Fatal("acquired held lock")
	}
	unlock()
	release, err := WorkspaceLock(context.Background(), root, "plan-test")
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestFilePlanDeletionChecksInputsAndDistinguishesEmptyFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "legacy")
	if err := os.WriteFile(path, []byte("original"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := NewFilePlan(root)
	if err := p.Remove("legacy"); err != nil {
		t.Fatal(err)
	}
	if len(p.Changes()) != 1 || !p.Changes()[0].Remove {
		t.Fatal("missing deletion preview")
	}
	if err := os.WriteFile(path, []byte("edited"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err == nil {
		t.Fatal("deleted concurrent edit")
	}
	p = NewFilePlan(root)
	if err := p.Remove("legacy"); err != nil {
		t.Fatal(err)
	}
	if err := p.Set("empty", []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("legacy file remains")
	}
	if b, err := os.ReadFile(filepath.Join(root, "empty")); err != nil || len(b) != 0 {
		t.Fatal("empty file wasn't created")
	}
}
