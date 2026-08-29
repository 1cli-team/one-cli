package workspace

import (
	"context"
	"errors"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func seedTestWorkspace(t *testing.T, templateID string) string {
	t.Helper()
	root := t.TempDir()
	manifest := &workspacecore.Manifest{
		Version:   workspacecore.ManifestVersion,
		Workspace: &workspacecore.ManifestWorkspace{ID: "demo", Name: "demo"},
		Projects: []workspacecore.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: templateID, Toolchain: "node",
		}},
	}
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestServiceOwnsWorkspaceSelectionMutations(t *testing.T) {
	service, err := NewService(catalog.Builtin())
	if err != nil {
		t.Fatal(err)
	}
	root := seedTestWorkspace(t, "react-spa")
	if _, err := service.SetEnvironment(root, catalog.EnvDotenv); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetProjectDeployment(context.Background(), root, "web", catalog.DeployVercel); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetProjectContainer(root, "web", catalog.ContainerGHCR, "web:latest"); err != nil {
		t.Fatal(err)
	}

	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if workspacecore.EnvBackend(manifest) != catalog.EnvDotenv ||
		workspacecore.DeployForProject(manifest, "web").Backend != catalog.DeployVercel ||
		workspacecore.ContainerKindForProject(manifest, "web") != catalog.ContainerGHCR {
		t.Fatalf("selection mutation did not persist: %#v", manifest)
	}
}

func TestServiceRejectsIncompatibleDeploymentBeforeWrite(t *testing.T) {
	service, err := NewService(catalog.Builtin())
	if err != nil {
		t.Fatal(err)
	}
	root := seedTestWorkspace(t, "go-api")
	_, err = service.SetProjectDeployment(context.Background(), root, "web", catalog.DeployVercel)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	manifest, readErr := workspacecore.ReadManifest(root)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if workspacecore.DeployForProject(manifest, "web").Backend != "" {
		t.Fatal("incompatible deployment changed the manifest")
	}
}
