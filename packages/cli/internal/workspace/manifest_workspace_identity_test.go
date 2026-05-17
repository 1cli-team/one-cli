package workspace_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// TestReadManifest_WithoutWorkspaceField confirms that a current-version
// manifest without the optional workspace block still parses cleanly.
// New scaffolds always set workspace, but env init may run on workspaces
// that were created before the field existed.
func TestReadManifest_WithoutWorkspaceField(t *testing.T) {
	tmp := t.TempDir()
	bare := `{
  "version": 1,
  "projects": []
}
`
	if err := os.WriteFile(filepath.Join(tmp, "one.manifest.json"), []byte(bare), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := workspace.ReadManifest(tmp)
	if err != nil {
		t.Fatalf("ReadManifest = %v", err)
	}
	if m.Workspace != nil {
		t.Errorf("manifest without workspace should round-trip with nil Workspace; got %+v", m.Workspace)
	}
}

func TestSetManifestWorkspaceIdentity_NewWorkspace(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.SetManifestWorkspaceIdentity(tmp, "demo-abc123", "demo"); err != nil {
		t.Fatalf("SetManifestWorkspaceIdentity = %v", err)
	}
	m, err := workspace.ReadManifest(tmp)
	if err != nil {
		t.Fatalf("ReadManifest = %v", err)
	}
	if m.Workspace == nil {
		t.Fatalf("Workspace not set after SetManifestWorkspaceIdentity")
	}
	if m.Workspace.ID != "demo-abc123" || m.Workspace.Name != "demo" {
		t.Errorf("workspace mismatch: got %+v", *m.Workspace)
	}
}

func TestSetManifestWorkspaceIdentity_OverwritesPrevious(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.SetManifestWorkspaceIdentity(tmp, "old-id", "old"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SetManifestWorkspaceIdentity(tmp, "new-id", "new"); err != nil {
		t.Fatal(err)
	}
	m, err := workspace.ReadManifest(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if m.Workspace.ID != "new-id" || m.Workspace.Name != "new" {
		t.Errorf("expected overwrite to new; got %+v", *m.Workspace)
	}
}

// TestRebuildManifest_PreservesWorkspace is the regression test for
// RebuildManifest accidentally wiping the workspace identity.
// RebuildManifest rewrites the projects list wholesale; Workspace must
// survive.
func TestRebuildManifest_PreservesWorkspace(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.SetManifestWorkspaceIdentity(tmp, "demo-abc", "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.RebuildManifest(tmp, []workspace.ManifestProjectInput{
		{Name: "api", RelativeDir: "services/api", TemplateID: "go-api", Toolchain: "go"},
	}); err != nil {
		t.Fatalf("RebuildManifest = %v", err)
	}
	m, err := workspace.ReadManifest(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if m.Workspace == nil {
		t.Fatalf("RebuildManifest dropped Workspace")
	}
	if m.Workspace.ID != "demo-abc" || m.Workspace.Name != "demo" {
		t.Errorf("workspace mutated by rebuild: %+v", *m.Workspace)
	}
	if len(m.Projects) != 1 || m.Projects[0].Name != "api" {
		t.Errorf("projects not rebuilt: %+v", m.Projects)
	}
}

// TestWorkspaceField_OmitemptyOnDisk confirms the JSON encoder elides
// the Workspace field when nil so we don't introduce churn on every
// WriteManifest of a workspace-less manifest.
func TestWorkspaceField_OmitemptyOnDisk(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Version:  workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{},
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(tmp, "one.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, has := doc["workspace"]; has {
		t.Errorf("expected workspace key omitted when nil; got=%v", doc)
	}
}
