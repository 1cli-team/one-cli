package execution

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestResolveWorkspaceScopeCreatesReusableSnapshot(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "apps", "web")
	workingDirectory := filepath.Join(projectDir, "src")
	if err := os.MkdirAll(workingDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Workspace: &workspace.ManifestWorkspace{ID: "ws-demo", Name: "demo"},
		Projects: []workspace.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", Toolchain: "node", PackageManager: "pnpm",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	parent := NewScope(context.Background(), workingDirectory)
	resolved, err := ResolveWorkspaceScope(parent)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Root() != root || resolved.Scope().WorkspaceRoot() != root {
		t.Fatalf("resolved root = %q, scope = %#v", resolved.Root(), resolved.Scope())
	}
	if parent.WorkspaceRoot() != "" {
		t.Fatal("workspace resolution mutated the parent scope")
	}
	byName, ok := resolved.Project("web")
	if !ok || byName.TargetDir != projectDir {
		t.Fatalf("Project(web) = %#v, %v", byName, ok)
	}
	byPath, ok := resolved.Project("./apps/web/")
	if !ok || byPath.Name != "web" {
		t.Fatalf("Project(path) = %#v, %v", byPath, ok)
	}
	fromCWD, ok := resolved.ProjectFromWorkingDirectory()
	if !ok || fromCWD.Name != "web" {
		t.Fatalf("ProjectFromWorkingDirectory() = %#v, %v", fromCWD, ok)
	}
}

func TestWorkspaceReloadRefreshesManifest(t *testing.T) {
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Workspace: &workspace.ManifestWorkspace{ID: "ws-demo", Name: "demo"},
	}); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveWorkspaceScope(NewScope(context.Background(), root))
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Workspace: &workspace.ManifestWorkspace{ID: "ws-demo", Name: "demo"},
		Projects:  []workspace.ManifestProject{{Name: "api", RelativeDir: "services/api"}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(resolved.Projects()) != 0 {
		t.Fatal("snapshot changed before Reload")
	}
	resolved, err = resolved.Reload()
	if err != nil || len(resolved.Projects()) != 1 || resolved.Projects()[0].Name != "api" {
		t.Fatalf("Reload() = %#v, %v", resolved.Projects(), err)
	}
}

func TestResolveWorkspaceScopeRequiresManifest(t *testing.T) {
	_, err := ResolveWorkspaceScope(NewScope(context.Background(), t.TempDir()))
	if code := codedError(err); code != "NOT_ONE_PROJECT" {
		t.Fatalf("error = %v, code = %q", err, code)
	}
}

func codedError(err error) string {
	if coded, ok := err.(interface{ ErrorCode() string }); ok {
		return coded.ErrorCode()
	}
	return ""
}
