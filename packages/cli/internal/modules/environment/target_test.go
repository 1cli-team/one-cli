package environment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestResolveInfisicalFolderPath(t *testing.T) {
	root := t.TempDir()
	if err := workspace.SetManifestWorkspaceIdentity(root, "id", "demo"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []workspace.ManifestProjectInput{
		{Name: "api", RelativeDir: "services/api", TemplateID: "go-api", Toolchain: "go"},
		{Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node"},
	} {
		if err := workspace.UpsertManifestProject(root, input); err != nil {
			t.Fatal(err)
		}
	}
	config := &infisical.WorkspaceConfig{ProjectID: "x", RootPath: "/"}
	previous, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(previous) })
	if err := os.Chdir(filepath.Dir(root)); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t)
	for _, test := range []struct {
		selector string
		want     string
	}{
		{want: "/"},
		{selector: "api", want: "/services/api"},
		{selector: "apps/web", want: "/apps/web"},
		{selector: "./apps/web", want: "/apps/web"},
		{selector: "/shared", want: "/shared"},
	} {
		activeWorkspace := resolveTestWorkspace(t, root)
		got, err := service.resolveInfisicalFolderPath(activeWorkspace, config, test.selector)
		if err != nil {
			t.Fatalf("selector %q: %v", test.selector, err)
		}
		if got != test.want {
			t.Fatalf("selector %q: path = %q, want %q", test.selector, got, test.want)
		}
	}
	activeWorkspace := resolveTestWorkspace(t, root)
	if _, err := service.resolveInfisicalFolderPath(activeWorkspace, config, "missing"); errorCode(err) != "SUBPROJECT_NOT_FOUND" {
		t.Fatalf("unknown selector error = %v", err)
	}
	projectDir := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}
	activeWorkspace = resolveTestWorkspace(t, root)
	if got, err := service.resolveInfisicalFolderPath(activeWorkspace, config, ""); err != nil || got != "/services/api" {
		t.Fatalf("cwd path = %q, err = %v", got, err)
	}
}

func TestResolveSetTarget(t *testing.T) {
	root := t.TempDir()
	if err := workspace.SetManifestWorkspaceIdentity(root, "id", "demo"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.UpsertManifestProject(root, workspace.ManifestProjectInput{
		Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
	}); err != nil {
		t.Fatal(err)
	}
	activeWorkspace := executionWorkspaceAt(t, root, root)
	project, target := resolveSetTarget(activeWorkspace, "web")
	if project == nil || project.Name != "web" || target != "apps/web" {
		t.Fatalf("declared target = (%#v, %q)", project, target)
	}
	project, target = resolveSetTarget(activeWorkspace, " shared ")
	if project != nil || target != "shared" {
		t.Fatalf("raw target = (%#v, %q)", project, target)
	}
}

func resolveTestWorkspace(t *testing.T, root string) execution.Workspace {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return executionWorkspaceAt(t, root, cwd)
}

func executionWorkspaceAt(t *testing.T, root, cwd string) execution.Workspace {
	t.Helper()
	scope := execution.NewScope(context.Background(), cwd).Derive(execution.ScopePatch{WorkspaceRoot: root})
	activeWorkspace, err := execution.ResolveWorkspaceScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	return activeWorkspace
}

func errorCode(err error) string {
	if coded, ok := err.(interface{ ErrorCode() string }); ok {
		return coded.ErrorCode()
	}
	return ""
}
