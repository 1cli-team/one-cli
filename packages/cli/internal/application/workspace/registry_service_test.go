package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	registrylocal "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/workspaceregistry/local"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestRegistryObserveIsIdempotentForSamePath(t *testing.T) {
	root := createRegistryTestWorkspace(t, "same-id", "same", 2)
	path := filepath.Join(t.TempDir(), "workspaces.json")
	store := registrylocal.NewAt(path)
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	generated := 0
	service := newRegistryTestService(t, store, func() time.Time { return now }, func() (string, error) {
		generated++
		return fmt.Sprintf("wsr_%d", generated), nil
	})

	first, err := service.Observe(context.Background(), root, "create")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	second, err := service.Observe(context.Background(), root, "serve")
	if err != nil {
		t.Fatal(err)
	}
	if first.EntryID != second.EntryID {
		t.Fatalf("same root changed entry ID: %q -> %q", first.EntryID, second.EntryID)
	}
	if generated != 1 {
		t.Fatalf("entry IDs generated = %d, want 1", generated)
	}
	if second.Status != WorkspaceStatusReady || second.ProjectCount != 2 {
		t.Fatalf("second observation = %#v", second)
	}

	registry, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Workspaces) != 1 {
		t.Fatalf("registry entries = %d, want 1", len(registry.Workspaces))
	}
	entry := registry.Workspaces[0]
	if !entry.RegisteredAt.Equal(now.Add(-time.Hour)) {
		t.Fatalf("RegisteredAt changed: %s", entry.RegisteredAt)
	}
	if !entry.LastSeenAt.Equal(now) || entry.LastSeenBy != "serve" {
		t.Fatalf("last observation not refreshed: %#v", entry)
	}
}

func TestRegistryObserveCanonicalizesSymlinkPath(t *testing.T) {
	root := createRegistryTestWorkspace(t, "symlink-id", "symlink", 0)
	alias := filepath.Join(t.TempDir(), "workspace-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	store := registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json"))
	service := newRegistryTestService(t, store, time.Now, sequenceEntryIDs())

	first, err := service.Observe(context.Background(), alias, "serve")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Observe(context.Background(), root, "serve")
	if err != nil {
		t.Fatal(err)
	}
	if first.EntryID != second.EntryID || first.Root != root || second.Root != root {
		t.Fatalf("symlink observations were not canonicalized: first=%#v second=%#v", first, second)
	}
}

func TestRegistryObserveMovePreservesEntryID(t *testing.T) {
	base := t.TempDir()
	oldRoot := filepath.Join(base, "old")
	newRoot := filepath.Join(base, "new")
	if err := os.Mkdir(oldRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRegistryTestManifest(t, oldRoot, "move-id", "moved", 1)
	store := registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json"))
	service := newRegistryTestService(t, store, time.Now, sequenceEntryIDs())

	before, err := service.Observe(context.Background(), oldRoot, "create")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}
	after, err := service.Observe(context.Background(), newRoot, "serve")
	if err != nil {
		t.Fatal(err)
	}
	if after.EntryID != before.EntryID {
		t.Fatalf("move changed entry ID: %q -> %q", before.EntryID, after.EntryID)
	}
	if after.Root != newRoot || after.Status != WorkspaceStatusReady {
		t.Fatalf("moved observation = %#v", after)
	}
	registry, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Workspaces) != 1 || registry.Workspaces[0].Root != newRoot {
		t.Fatalf("move left stale entries: %#v", registry.Workspaces)
	}
}

func TestRegistryCopiedWorkspaceCreatesIdentityConflict(t *testing.T) {
	firstRoot := createRegistryTestWorkspace(t, "copied-id", "original", 1)
	secondRoot := createRegistryTestWorkspace(t, "copied-id", "copy", 3)
	store := registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json"))
	service := newRegistryTestService(t, store, time.Now, sequenceEntryIDs())

	first, err := service.Observe(context.Background(), firstRoot, "serve")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Observe(context.Background(), secondRoot, "serve")
	if err != nil {
		t.Fatal(err)
	}
	if first.EntryID == second.EntryID {
		t.Fatalf("active copied workspaces shared entry ID %q", first.EntryID)
	}
	if second.Status != WorkspaceStatusIdentityConflict {
		t.Fatalf("second status = %q, want identity conflict", second.Status)
	}

	listed, err := service.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Workspaces) != 2 {
		t.Fatalf("listed workspaces = %d, want 2", len(listed.Workspaces))
	}
	for _, registered := range listed.Workspaces {
		if registered.Status != WorkspaceStatusIdentityConflict {
			t.Errorf("copy %s status = %q", registered.EntryID, registered.Status)
		}
		if _, resolveErr := service.Resolve(context.Background(), registered.EntryID); !errors.Is(resolveErr, ErrRegistryIdentityConflict) {
			t.Errorf("Resolve(%s) error = %v, want identity conflict", registered.EntryID, resolveErr)
		}
	}
	for _, test := range []struct {
		entryID string
		root    string
	}{
		{entryID: first.EntryID, root: firstRoot},
		{entryID: second.EntryID, root: secondRoot},
	} {
		resolved, resolveErr := service.ResolveRead(context.Background(), test.entryID)
		if resolveErr != nil {
			t.Errorf("ResolveRead(%s) error = %v", test.entryID, resolveErr)
			continue
		}
		if resolved.Root != test.root || resolved.Status != WorkspaceStatusIdentityConflict {
			t.Errorf("ResolveRead(%s) = %#v", test.entryID, resolved)
		}
	}
}

func TestRegistryLegacyIdentityCanBeListedButNotResolved(t *testing.T) {
	root := createRegistryTestWorkspace(t, "", "legacy", 1)
	service := newRegistryTestService(
		t,
		registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json")),
		time.Now,
		sequenceEntryIDs(),
	)

	registered, err := service.Observe(context.Background(), root, "serve")
	if err != nil {
		t.Fatalf("legacy Observe: %v", err)
	}
	if registered.Status != WorkspaceStatusIdentityMissing || registered.ID != "" {
		t.Fatalf("legacy registration = %#v", registered)
	}
	_, err = service.Resolve(context.Background(), registered.EntryID)
	if !errors.Is(err, ErrRegistryUnavailable) {
		t.Fatalf("legacy Resolve error = %v, want unavailable", err)
	}
}

func TestRegistryMissingRootRemainsVisibleAndResolveRejectsIt(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	missingRoot := filepath.Join(base, "workspace-offline")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRegistryTestManifest(t, root, "missing-id", "missing", 1)
	service := newRegistryTestService(
		t,
		registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json")),
		time.Now,
		sequenceEntryIDs(),
	)
	registered, err := service.Observe(context.Background(), root, "serve")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, missingRoot); err != nil {
		t.Fatal(err)
	}

	listed, err := service.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Workspaces) != 1 || listed.Workspaces[0].Status != WorkspaceStatusMissing {
		t.Fatalf("missing list = %#v", listed.Workspaces)
	}
	_, err = service.Resolve(context.Background(), registered.EntryID)
	if !errors.Is(err, ErrRegistryUnavailable) {
		t.Fatalf("missing Resolve error = %v, want unavailable", err)
	}
}

func TestRegistryExistingRootWithoutManifestIsInvalid(t *testing.T) {
	root := t.TempDir()
	store := registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json"))
	now := time.Now().UTC()
	if err := store.Update(context.Background(), func(registry *workspacecore.Registry) error {
		registry.Workspaces = append(registry.Workspaces, workspacecore.RegistryEntry{
			EntryID:      "wsr_invalid",
			WorkspaceID:  "invalid-id",
			Name:         "invalid",
			Root:         root,
			RegisteredAt: now,
			LastSeenAt:   now,
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	service := newRegistryTestService(t, store, time.Now, sequenceEntryIDs())

	listed, err := service.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Workspaces) != 1 || listed.Workspaces[0].Status != WorkspaceStatusInvalid {
		t.Fatalf("manifest-less root list = %#v", listed.Workspaces)
	}
	if _, err := service.Observe(context.Background(), root, "serve"); !errors.Is(err, ErrRegistryUnavailable) {
		t.Fatalf("manifest-less Observe error = %v, want unavailable", err)
	}
	if _, err := service.Resolve(context.Background(), "wsr_invalid"); !errors.Is(err, ErrRegistryUnavailable) {
		t.Fatalf("manifest-less Resolve error = %v, want unavailable", err)
	}
}

func TestRegistryResolveRevalidatesManifestIdentity(t *testing.T) {
	root := createRegistryTestWorkspace(t, "before-id", "before", 1)
	service := newRegistryTestService(
		t,
		registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json")),
		time.Now,
		sequenceEntryIDs(),
	)
	registered, err := service.Observe(context.Background(), root, "serve")
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := service.Resolve(context.Background(), registered.EntryID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Root != root || resolved.Manifest == nil || resolved.Manifest.Workspace.ID != "before-id" {
		t.Fatalf("resolved = %#v", resolved)
	}

	writeRegistryTestManifest(t, root, "after-id", "after", 1)
	_, err = service.Resolve(context.Background(), registered.EntryID)
	if !errors.Is(err, ErrRegistryIdentityConflict) {
		t.Fatalf("changed identity Resolve error = %v, want identity conflict", err)
	}
}

func TestRegistryForgetRemovesOnlyRegistration(t *testing.T) {
	root := createRegistryTestWorkspace(t, "forget-id", "forget", 1)
	service := newRegistryTestService(
		t,
		registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json")),
		time.Now,
		sequenceEntryIDs(),
	)
	registered, err := service.Observe(context.Background(), root, "serve")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Forget(context.Background(), registered.EntryID); err != nil {
		t.Fatal(err)
	}
	if !workspacecore.HasManifest(root) {
		t.Fatal("Forget removed or changed the workspace")
	}
	listed, err := service.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Workspaces) != 0 {
		t.Fatalf("workspaces after Forget = %#v", listed.Workspaces)
	}
	if err := service.Forget(context.Background(), registered.EntryID); !errors.Is(err, ErrRegistryEntryNotFound) {
		t.Fatalf("second Forget error = %v, want not found", err)
	}
}

func newRegistryTestService(
	t *testing.T,
	store *registrylocal.Store,
	clock func() time.Time,
	entryIDs func() (string, error),
) *RegistryService {
	t.Helper()
	service, err := NewRegistryService(
		store,
		WithRegistryClock(clock),
		WithRegistryEntryIDGenerator(entryIDs),
	)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func sequenceEntryIDs() func() (string, error) {
	next := 0
	return func() (string, error) {
		next++
		return fmt.Sprintf("wsr_%d", next), nil
	}
}

func createRegistryTestWorkspace(t *testing.T, id string, name string, projectCount int) string {
	t.Helper()
	root := t.TempDir()
	writeRegistryTestManifest(t, root, id, name, projectCount)
	return root
}

func writeRegistryTestManifest(t *testing.T, root string, id string, name string, projectCount int) {
	t.Helper()
	projects := make([]workspacecore.ManifestProject, 0, projectCount)
	for index := 0; index < projectCount; index++ {
		projects = append(projects, workspacecore.ManifestProject{
			Name:         fmt.Sprintf("project-%d", index),
			RelativeDir:  fmt.Sprintf("apps/project-%d", index),
			TemplateID:   "react-spa",
			Toolchain:    "node",
			BuildVersion: workspacecore.DefaultBuildVersion,
		})
	}
	manifest := &workspacecore.Manifest{
		Version: workspacecore.ManifestVersion,
		Workspace: &workspacecore.ManifestWorkspace{
			ID:   id,
			Name: name,
		},
		Projects: projects,
	}
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
}
