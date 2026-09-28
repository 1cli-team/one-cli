package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func seedProjectSettingsWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	inherits := false
	manifest := &workspacecore.Manifest{
		Version:      workspacecore.ManifestVersion,
		Workspace:    &workspacecore.ManifestWorkspace{ID: "ws-demo", Name: "Demo"},
		Environments: &workspacecore.Environments{Names: []string{"dev", "staging", "prod"}, Default: "dev"},

		Env: &workspacecore.EnvironmentConfig{ProjectID: "remote"},

		Projects: []workspacecore.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
			BuildVersion: "1.2.3", PackageManager: "pnpm",

			Env: &workspacecore.ProjectEnvOverride{
				Path: "/apps/web", Inherits: &inherits, Keys: []string{"Z_KEY", "A_KEY"},
			},

			Dev: &workspacecore.ProjectDevOverride{Command: "pnpm dev"},
		}},
	}
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("repository data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func snapshotWorkspaceTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		value, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertWorkspaceTreeEqual(t *testing.T, got, want map[string][]byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("workspace file count changed: got %d want %d", len(got), len(want))
	}
	for path, expected := range want {
		if !bytes.Equal(got[path], expected) {
			t.Fatalf("workspace file %q changed", path)
		}
	}
}

func TestProjectSettingsReturnsEnvironmentAwareSafeProjection(t *testing.T) {
	root := seedProjectSettingsWorkspace(t)
	service, err := NewService(catalog.Builtin())
	if err != nil {
		t.Fatal(err)
	}
	settings, err := service.ProjectSettings(context.Background(), root, "web", "preview")
	if err != nil {
		t.Fatal(err)
	}
	project := settings.Project
	if settings.Schema != ProjectSettingsSchema || settings.Environment != "preview" ||
		project.Kind != workspacecore.ProjectKindApp {
		t.Fatalf("unexpected envelope: %#v", settings)
	}

	if got := strings.Join(project.Environment.Keys, ","); got != "A_KEY,Z_KEY" {
		t.Fatalf("environment keys = %q", got)
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "must-not-leak") || strings.Contains(string(raw), "never-return") {
		t.Fatalf("settings leaked a credential: %s", raw)
	}
}
