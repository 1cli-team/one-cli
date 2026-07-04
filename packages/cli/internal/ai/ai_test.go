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
		Version: workspace.ManifestVersion,
		Workspace: &workspace.ManifestWorkspace{
			Name: "demo",
		},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendDotenv},
		},
		Projects: []workspace.ManifestProject{{
			Name:         "api",
			RelativeDir:  "services/api",
			TemplateID:   "go-api",
			Toolchain:    "go",
			BuildVersion: workspace.DefaultBuildVersion,
			Domains: &workspace.ProjectDomains{
				Container: &workspace.ProjectContainerOverride{},
				Deploy:    &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendKustomize},
				Dev:       &workspace.ProjectDevOverride{Command: "go run ./cmd/server"},
			},
		}},
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
	wantFiles := []string{
		"AGENTS.md",
		"CLAUDE.md",
		".one/agents/conventions.md",
		".one/agents/projects/services-api.md",
		".one/agents/ops/dev.md",
		".one/agents/ops/secrets.md",
		".one/agents/ops/container.md",
		".one/agents/ops/deploy.md",
	}
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
	claudeRaw, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if got := string(claudeRaw); got != ClaudePointerContent() {
		t.Fatalf("CLAUDE.md = %q, want %q", got, ClaudePointerContent())
	}
}
