package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestStoreRoundTripAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one", "workspaces.json")
	store := NewAt(path)
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	want := workspace.RegistryEntry{
		EntryID:      "wsr_roundtrip",
		WorkspaceID:  "demo-a1b2c3",
		Name:         "demo",
		Root:         filepath.Join(t.TempDir(), "demo"),
		RegisteredAt: now,
		LastSeenAt:   now,
		LastSeenBy:   "create",
	}
	if err := store.Update(context.Background(), func(registry *workspace.Registry) error {
		registry.Workspaces = append(registry.Workspaces, want)
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != workspace.RegistrySchemaVersion {
		t.Fatalf("version = %d, want %d", got.Version, workspace.RegistrySchemaVersion)
	}
	if len(got.Workspaces) != 1 || got.Workspaces[0] != want {
		t.Fatalf("round trip = %#v, want %#v", got.Workspaces, want)
	}
	if store.Path() != path {
		t.Fatalf("Path() = %q, want %q", store.Path(), path)
	}

	if runtime.GOOS != "windows" {
		fileInfo, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if mode := fileInfo.Mode().Perm(); mode != 0o600 {
			t.Fatalf("registry mode = %o, want 600", mode)
		}
		dirInfo, statErr := os.Stat(filepath.Dir(path))
		if statErr != nil {
			t.Fatal(statErr)
		}
		if mode := dirInfo.Mode().Perm(); mode != 0o700 {
			t.Fatalf("registry directory mode = %o, want 700", mode)
		}
	}
}

func TestPathHonorsXDGConfigHome(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(xdg, "one", "workspaces.json")
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestStoreRenameFailureLeavesPreviousFileUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one", "workspaces.json")
	store := NewAt(path)
	seedRegistryEntry(t, store, "wsr_original")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	store.renameFile = func(string, string) error { return errors.New("injected rename failure") }
	err = store.Update(context.Background(), func(registry *workspace.Registry) error {
		registry.Workspaces[0].Name = "changed"
		return nil
	})
	if err == nil {
		t.Fatal("Update succeeded despite injected rename failure")
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("failed atomic publication changed destination\nbefore: %s\nafter: %s", before, after)
	}
	temps, globErr := filepath.Glob(filepath.Join(filepath.Dir(path), ".workspaces-*.json"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(temps) != 0 {
		t.Fatalf("temporary files leaked: %v", temps)
	}
}

func TestStoreDamagedOrFutureRegistryIsNeverOverwritten(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "damaged", raw: `{"version":1,"workspaces":[`},
		{name: "future", raw: `{"version":2,"workspaces":[]}`},
		{name: "duplicate entry ids", raw: `{"version":1,"workspaces":[{"entryId":"same","root":"/a","registeredAt":"2026-08-30T00:00:00Z","lastSeenAt":"2026-08-30T00:00:00Z"},{"entryId":"same","root":"/b","registeredAt":"2026-08-30T00:00:00Z","lastSeenAt":"2026-08-30T00:00:00Z"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "one", "workspaces.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.raw), 0o600); err != nil {
				t.Fatal(err)
			}
			mutated := false
			err := NewAt(path).Update(context.Background(), func(*workspace.Registry) error {
				mutated = true
				return nil
			})
			if err == nil {
				t.Fatal("Update succeeded for unsafe registry")
			}
			if mutated {
				t.Fatal("mutator ran before registry validation")
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(after) != test.raw {
				t.Fatalf("unsafe registry was overwritten: %s", after)
			}
		})
	}
}

func TestStoreConcurrentUpdatesDoNotLoseEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one", "workspaces.json")
	const writers = 32
	start := make(chan struct{})
	errorsByWriter := make(chan error, writers)
	var wait sync.WaitGroup
	for index := 0; index < writers; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			store := NewAt(path)
			err := store.Update(context.Background(), func(registry *workspace.Registry) error {
				registry.Workspaces = append(registry.Workspaces, workspace.RegistryEntry{
					EntryID:      fmt.Sprintf("wsr_%02d", index),
					Root:         fmt.Sprintf("/workspace/%02d", index),
					RegisteredAt: time.Unix(int64(index), 0).UTC(),
					LastSeenAt:   time.Unix(int64(index), 0).UTC(),
				})
				return nil
			})
			errorsByWriter <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsByWriter)
	for err := range errorsByWriter {
		if err != nil {
			t.Fatalf("concurrent Update: %v", err)
		}
	}

	registry, err := NewAt(path).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Workspaces) != writers {
		t.Fatalf("entries = %d, want %d", len(registry.Workspaces), writers)
	}
	seen := make(map[string]bool, writers)
	for _, entry := range registry.Workspaces {
		seen[entry.EntryID] = true
	}
	for index := 0; index < writers; index++ {
		entryID := fmt.Sprintf("wsr_%02d", index)
		if !seen[entryID] {
			t.Errorf("lost concurrent entry %s", entryID)
		}
	}
}

func TestStoreHonorsShorterContextDeadlineWhileLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one", "workspaces.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	externalLock := flock.New(path + ".lock")
	if err := externalLock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer externalLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := NewAt(path).Update(ctx, func(*workspace.Registry) error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Update error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("short caller deadline was not respected: %s", elapsed)
	}
}

func TestStoreContextDeadlineAlsoBoundsInProcessLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one", "workspaces.json")
	store := NewAt(path)
	release, err := store.acquire(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = store.Update(ctx, func(*workspace.Registry) error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Update error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("in-process lock ignored caller deadline: %s", elapsed)
	}
}

func TestStoreBackgroundContextWaitsPastShortLockContention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one", "workspaces.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	externalLock := flock.New(path + ".lock")
	if err := externalLock.Lock(); err != nil {
		t.Fatal(err)
	}
	const hold = 2200 * time.Millisecond
	timer := time.AfterFunc(hold, func() {
		_ = externalLock.Unlock()
	})
	defer func() {
		timer.Stop()
		_ = externalLock.Unlock()
	}()

	started := time.Now()
	err := NewAt(path).Update(context.Background(), func(registry *workspace.Registry) error {
		registry.Workspaces = append(registry.Workspaces, workspace.RegistryEntry{
			EntryID:      "wsr_after_contention",
			Root:         "/workspace/after-contention",
			RegisteredAt: time.Unix(1, 0).UTC(),
			LastSeenAt:   time.Unix(1, 0).UTC(),
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Update after short contention: %v", err)
	}
	if elapsed := time.Since(started); elapsed < hold {
		t.Fatalf("Update returned before contention ended: %s < %s", elapsed, hold)
	}
}

func TestStoreMutatorErrorDoesNotPublish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one", "workspaces.json")
	store := NewAt(path)
	seedRegistryEntry(t, store, "wsr_original")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("stop mutation")
	err = store.Update(context.Background(), func(registry *workspace.Registry) error {
		registry.Workspaces = nil
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Update error = %v, want %v", err, wantErr)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatal("mutator error changed registry file")
	}
}

func seedRegistryEntry(t *testing.T, store *Store, entryID string) {
	t.Helper()
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	if err := store.Update(context.Background(), func(registry *workspace.Registry) error {
		registry.Workspaces = append(registry.Workspaces, workspace.RegistryEntry{
			EntryID:      entryID,
			Root:         filepath.Join(t.TempDir(), entryID),
			RegisteredAt: now,
			LastSeenAt:   now,
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
