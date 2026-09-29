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
	if !reflect.DeepEqual(EnvironmentNames(manifest), []string{"dev", "staging", "prod"}) {
		t.Fatal(EnvironmentNames(manifest))
	}
	if EnvBackend(manifest) != "" {
		t.Fatal("development fixture should not bind a real remote project")
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
}
