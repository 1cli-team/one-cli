package creation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
)

func TestServiceOwnsWorkspaceAndProjectCreation(t *testing.T) {
	service := newCreationService(t)
	target := filepath.Join(t.TempDir(), "demo")

	created, err := service.CreateWorkspace(context.Background(), WorkspaceInput{
		TargetDir:  target,
		Name:       "demo",
		EnvBackend: workspace.EnvBackendDotenv,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.TargetDir != target || created.PackageManager != "pnpm" || created.EnvBackend != "dotenv" {
		t.Fatalf("CreateWorkspace() = %+v", created)
	}
	manifest, err := workspace.ReadManifest(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := workspace.EnvBackend(manifest); got != workspace.EnvBackendDotenv {
		t.Fatalf("environment backend = %q", got)
	}
	if _, err := os.Stat(filepath.Join(target, ".gitignore")); err != nil {
		t.Fatalf("workspace files were not generated: %v", err)
	}

	registry, err := template.Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	var selected *template.Template
	for index := range registry.Templates {
		if registry.Templates[index].ID == "react-spa" {
			selected = &registry.Templates[index]
			break
		}
	}
	if selected == nil {
		t.Fatal("react-spa template is absent")
	}
	added, err := service.AddProject(context.Background(), target, ProjectInput{
		Template: selected, Name: "web", DeferDeployment: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if added.Project.Name != "web" || added.Project.TemplateID != "react-spa" {
		t.Fatalf("AddProject() = %+v", added.Project)
	}
	manifest, err = workspace.ReadManifest(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Projects) != 1 || manifest.Projects[0].Name != "web" ||
		manifest.Projects[0].Domains == nil || manifest.Projects[0].Domains.Dev == nil {
		t.Fatalf("manifest projects = %+v", manifest.Projects)
	}
}

func TestCreateWorkspaceRechecksTargetBeforeMutation(t *testing.T) {
	service := newCreationService(t)
	target := t.TempDir()
	marker := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(marker, []byte("user data"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := service.CreateWorkspace(context.Background(), WorkspaceInput{
		TargetDir: target, Name: "demo", EnvBackend: workspace.EnvBackendDotenv,
	})
	if coded, ok := err.(interface{ ErrorCode() string }); !ok || coded.ErrorCode() != "EXISTING_TARGET_NOT_EMPTY" {
		t.Fatalf("error = %v", err)
	}
	if raw, readErr := os.ReadFile(marker); readErr != nil || string(raw) != "user data" {
		t.Fatalf("existing target was mutated: raw=%q err=%v", raw, readErr)
	}
}

func newCreationService(t *testing.T) *Service {
	t.Helper()
	backendCatalog := catalog.Builtin()
	profiles, err := configureapp.NewProfileService(
		backendCatalog,
		configureapp.LocalProfileRepository{},
	)
	if err != nil {
		t.Fatal(err)
	}
	environments, err := environmentmodule.NewService(backendCatalog, profiles)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(environments)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
