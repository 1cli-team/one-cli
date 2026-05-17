package infisical

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// TestBuildPullTargets_DrivenByManifest pins the discovery contract: pull
// reads from one.manifest.json, NOT from filesystem-walking. A subproject
// missing from the manifest must not become a pull target.
func TestBuildPullTargets_DrivenByManifest(t *testing.T) {
	tmp := t.TempDir()

	// Manifest declares one subproject. Filesystem has another that's NOT
	// in the manifest — pull must ignore it.
	if err := writeFile(filepath.Join(tmp, "services", "api", "package.json"), `{"name":"api"}`); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(tmp, "services", "ghost", "package.json"), `{"name":"ghost"}`); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SetManifestWorkspaceIdentity(tmp, "demo-id", "demo"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.UpsertManifestProject(tmp, workspace.ManifestProjectInput{
		Name: "api", RelativeDir: "services/api", TemplateID: "go-api", Toolchain: "go",
	}); err != nil {
		t.Fatal(err)
	}

	cfg := &WorkspaceConfig{ProjectID: "x", RootPath: "/"}
	got, err := buildPullTargets(tmp, cfg, "")
	if err != nil {
		t.Fatalf("buildPullTargets = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 target; got %d (%+v)", len(got), got)
	}
	if got[0].RelativeDir != "services/api" {
		t.Errorf("unexpected target relativeDir = %q", got[0].RelativeDir)
	}
	// No "<workspace-root>" target. Pull writes per subproject only.
	for _, tg := range got {
		if tg.RelativeDir == "" || tg.Name == "<workspace-root>" {
			t.Errorf("workspace-root target was reintroduced: %+v", tg)
		}
	}
}

func TestBuildPullTargets_EmptyManifest_ReturnsTypedError(t *testing.T) {
	tmp := t.TempDir()
	// EnsureManifest writes an empty manifest; no subprojects.
	if _, err := workspace.EnsureManifest(tmp); err != nil {
		t.Fatal(err)
	}
	cfg := &WorkspaceConfig{ProjectID: "x", RootPath: "/"}
	_, err := buildPullTargets(tmp, cfg, "")
	if err == nil {
		t.Fatalf("expected MANIFEST_MISSING_OR_EMPTY error; got nil")
	}
	var typed *output.Error
	if !errors.As(err, &typed) || typed.Code != string(cliErrors.MANIFEST_MISSING_OR_EMPTY) {
		t.Errorf("expected MANIFEST_MISSING_OR_EMPTY; got %v", err)
	}
}

func TestBuildPullTargets_MissingManifest_ReturnsTypedError(t *testing.T) {
	tmp := t.TempDir()
	cfg := &WorkspaceConfig{ProjectID: "x", RootPath: "/"}
	_, err := buildPullTargets(tmp, cfg, "")
	if err == nil {
		t.Fatalf("expected MANIFEST_MISSING_OR_EMPTY error on bare workspace; got nil")
	}
	var typed *output.Error
	if !errors.As(err, &typed) || typed.Code != string(cliErrors.MANIFEST_MISSING_OR_EMPTY) {
		t.Errorf("expected MANIFEST_MISSING_OR_EMPTY on bare workspace; got %v", err)
	}
}

func TestBuildPullTargets_FilterByName(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.SetManifestWorkspaceIdentity(tmp, "demo-id", "demo"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.UpsertManifestProject(tmp, workspace.ManifestProjectInput{
		Name: "api", RelativeDir: "services/api", TemplateID: "go-api", Toolchain: "go",
	}); err != nil {
		t.Fatal(err)
	}
	if err := workspace.UpsertManifestProject(tmp, workspace.ManifestProjectInput{
		Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
	}); err != nil {
		t.Fatal(err)
	}

	cfg := &WorkspaceConfig{ProjectID: "x", RootPath: "/"}

	// Filter by subproject name.
	got, err := buildPullTargets(tmp, cfg, "api")
	if err != nil {
		t.Fatalf("buildPullTargets(name) = %v", err)
	}
	if len(got) != 1 || !strings.HasSuffix(got[0].TargetDir, filepath.Join("services", "api")) {
		t.Fatalf("name filter: want 1 target ending in services/api, got %v", got)
	}

	// Filter by relative path.
	got, err = buildPullTargets(tmp, cfg, "apps/web")
	if err != nil {
		t.Fatalf("buildPullTargets(path) = %v", err)
	}
	if len(got) != 1 || !strings.HasSuffix(got[0].TargetDir, filepath.Join("apps", "web")) {
		t.Fatalf("path filter: want 1 target ending in apps/web, got %v", got)
	}

	// Unknown selector → SUBPROJECT_NOT_FOUND.
	if _, err := buildPullTargets(tmp, cfg, "nope"); err == nil {
		t.Fatal("expected error for unknown selector")
	}
}

// writeFile is a local helper — we can't import the workspace_test helper
// because we live in package infisical (not infisical_test).
func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
