package ai

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func TestRefreshDoesNotLeakWorkspaceAbsolutePath(t *testing.T) {
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version:  workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{},
	}); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	serviceDir := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("mkdir service: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "go.mod"), []byte("module example.com/api\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	res := Refresh(root, false)
	if res.Status != "completed" {
		t.Fatalf("Refresh status = %q, error = %+v", res.Status, res.ErrorBody)
	}
	wantFiles := []string{"AGENTS.md", "CLAUDE.md"}
	if !reflect.DeepEqual(res.GeneratedFiles, wantFiles) {
		t.Fatalf("GeneratedFiles = %v, want %v", res.GeneratedFiles, wantFiles)
	}
	for _, generated := range res.GeneratedFiles {
		if filepath.IsAbs(generated) {
			t.Errorf("GeneratedFiles should be workspace-relative, got absolute path %q", generated)
		}
		if strings.Contains(generated, root) {
			t.Errorf("GeneratedFiles leaked workspace root %q in %q", root, generated)
		}
	}

	for _, name := range wantFiles {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(raw), root) {
			t.Errorf("%s leaked workspace root %q:\n%s", name, root, raw)
		}
	}
}
