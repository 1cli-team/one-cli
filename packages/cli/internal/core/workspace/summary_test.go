package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectDependenciesInstalledChecksDeclaredNodeDependencies(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "unrelated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(`{
  "name": "web",
  "dependencies": {"react": "latest"},
  "devDependencies": {"vite": "latest"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if ProjectDependenciesInstalled(root, projectDir, "node") {
		t.Fatal("an unrelated root node_modules must not mark a newly added project as installed")
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "react"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "node_modules", "vite"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !ProjectDependenciesInstalled(root, projectDir, "node") {
		t.Fatal("dependencies resolved across root and project node_modules should count as installed")
	}
}

func TestProjectDependenciesInstalledHandlesNoInstallToolchains(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "services", "api")
	if !ProjectDependenciesInstalled(root, projectDir, "go") {
		t.Fatal("non-node toolchains do not have a package-manager install gate")
	}
}

func TestProjectDependenciesInstalledAllowsDependencyFreeNodeProject(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "apps", "empty")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(`{"name":"empty"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ProjectDependenciesInstalled(root, projectDir, "node") {
		t.Fatal("a dependency-free Node project does not need an install")
	}
}
