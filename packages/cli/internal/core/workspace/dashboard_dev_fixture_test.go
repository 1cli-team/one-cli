package workspace

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDashboardDevelopmentFixture(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "dashboard-dev-workspace")
	manifest, err := ReadManifest(root)
	if err != nil {
		t.Fatalf("ReadManifest(%q): %v", root, err)
	}
	if manifest.Version != ManifestVersion {
		t.Fatalf("manifest version = %d, want %d", manifest.Version, ManifestVersion)
	}
	if manifest.Workspace == nil || manifest.Workspace.ID != "one-dashboard-dev" {
		t.Fatalf("workspace identity = %#v", manifest.Workspace)
	}
	if manifest.Environments == nil || manifest.Environments.Default != "dev" ||
		!reflect.DeepEqual(manifest.Environments.Names, []string{"dev", "preview", "prod"}) {
		t.Fatalf("environments = %#v", manifest.Environments)
	}
	if got := EnvBackend(manifest); got != "infisical" {
		t.Fatalf("env backend = %q, want infisical", got)
	}

	projects := make(map[string]*ManifestProject, len(manifest.Projects))
	kinds := make(map[string]bool)
	for index := range manifest.Projects {
		project := &manifest.Projects[index]
		projects[project.Name] = project
		kinds[projectKindFromDir(project.RelativeDir)] = true
	}
	if !reflect.DeepEqual(kinds, map[string]bool{
		ProjectKindApp: true, ProjectKindService: true, ProjectKindPackage: true,
	}) {
		t.Fatalf("project kinds = %#v", kinds)
	}
	for _, name := range []string{"web", "api", "docs", "shared"} {
		if projects[name] == nil {
			t.Fatalf("project %q is missing from fixture", name)
		}
	}
	if got := ContainerKindForProject(manifest, "web"); got != "docker" {
		t.Fatalf("web container backend = %q, want docker", got)
	}
	if got := DeployForProject(manifest, "web").Backend; got != "vercel" {
		t.Fatalf("web deploy backend = %q, want vercel", got)
	}
	if got := DeployForProject(manifest, "api").Backend; got != "kustomize" {
		t.Fatalf("api deploy backend = %q, want kustomize", got)
	}
	if got := DeployForProject(manifest, "docs").Backend; got != "aws-s3" {
		t.Fatalf("docs deploy backend = %q, want aws-s3", got)
	}
}
