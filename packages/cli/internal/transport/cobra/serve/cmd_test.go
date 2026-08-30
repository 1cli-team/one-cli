package servecmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	registrylocal "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/workspaceregistry/local"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestDiscoverServeWorkspaceObservesAncestorManifest(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "apps", "web")
	if err := workspacecore.WriteManifest(root, &workspacecore.Manifest{
		Version: workspacecore.ManifestVersion,
		Workspace: &workspacecore.ManifestWorkspace{
			ID: "serve-test-a1b2c3", Name: "serve-test",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	store := registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json"))
	registry, err := workspaceapp.NewRegistryService(store)
	if err != nil {
		t.Fatal(err)
	}

	discovered, err := discoverServeWorkspace(context.Background(), child, registry)
	if err != nil {
		t.Fatalf("discoverServeWorkspace() error = %v", err)
	}
	if discovered != root {
		t.Fatalf("discovered root = %q, want %q", discovered, root)
	}
	persisted, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Workspaces) != 1 || persisted.Workspaces[0].Root != canonicalRoot ||
		persisted.Workspaces[0].LastSeenBy != "serve" {
		t.Fatalf("persisted registry = %#v", persisted.Workspaces)
	}
}

func TestDiscoverServeWorkspaceOutsideWorkspaceDoesNotRegister(t *testing.T) {
	store := registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json"))
	registry, err := workspaceapp.NewRegistryService(store)
	if err != nil {
		t.Fatal(err)
	}

	root, err := discoverServeWorkspace(context.Background(), t.TempDir(), registry)
	if err != nil {
		t.Fatalf("discoverServeWorkspace() error = %v", err)
	}
	if root != "" {
		t.Fatalf("root = %q, want empty", root)
	}
	persisted, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Workspaces) != 0 {
		t.Fatalf("outside serve registered entries: %#v", persisted.Workspaces)
	}
}
