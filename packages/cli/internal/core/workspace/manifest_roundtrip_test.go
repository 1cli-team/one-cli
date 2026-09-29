package workspace

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestManifestReadMissing(t *testing.T) {
	m, err := ReadManifest(t.TempDir())
	if err != nil || m.Version != 2 || len(m.Projects) != 0 {
		t.Fatalf("%+v %v", m, err)
	}
}
func TestManifestEditsPreserveUserText(t *testing.T) {
	root := t.TempDir()
	original := []byte("# 用户说明\nversion = 2 # version\n\n[projects.web] # app\npath = \"apps/web\" # folder\ntoolchain = 'node'\n\n[workspace]\nid = 'demo'\nname = 'Old' # display\n")
	if err := os.WriteFile(ManifestPath(root), original, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	m.Workspace.Name = "New"
	m.Projects = append(m.Projects, ManifestProject{Name: "api", RelativeDir: "services/api", Toolchain: "go"})
	m.Env = &EnvironmentConfig{ProjectID: "remote", Environments: []string{"dev", "staging"}}
	if err := WriteManifest(root, m); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(ManifestPath(root))
	for _, keep := range []string{"# 用户说明", "version = 2 # version", "[projects.web] # app", `path = "apps/web" # folder`, "toolchain = 'node'", "# display"} {
		if !bytes.Contains(raw, []byte(keep)) {
			t.Fatalf("lost user text %q: %s", keep, raw)
		}
	}
	m, err = ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	m.Env = nil
	m.Projects = m.Projects[:1]
	if err := WriteManifest(root, m); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(ManifestPath(root))
	if strings.Contains(string(raw), "env.infisical") || strings.Contains(string(raw), "projects.api") {
		t.Fatalf("obsolete table retained: %s", raw)
	}
	if _, err = ReadManifest(root); err != nil {
		t.Fatal(err)
	}
}
func TestInitBindingPreservesProjects(t *testing.T) {
	root := t.TempDir()
	m := &Manifest{Version: 2, Projects: []ManifestProject{{Name: "api", RelativeDir: "services/api", Toolchain: "go"}}}
	if err := WriteManifest(root, m); err != nil {
		t.Fatal(err)
	}
	if err := InitWorkspaceEnv(root, EnvInit{Kind: EnvBackendInfisical, ConfigJSON: []byte(`{"projectId":"remote"}`), EnvironmentNames: []string{"dev", "prod"}}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadManifest(root)
	if err != nil || got.Env.ProjectID != "remote" || len(got.Projects) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
}
