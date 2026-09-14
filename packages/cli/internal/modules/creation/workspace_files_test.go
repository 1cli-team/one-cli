package creation

// Unit tests for workspace generation. Assert every file lands at its
// expected path with the expected JSON shape; cross-binary parity is
// captured separately by the e2e snapshot suite.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestGenerateWorkspaceFiles(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "demo")

	err := generateWorkspaceFiles(target, workspaceFilesOptions{
		ProjectName: "demo",
	})
	if err != nil {
		t.Fatalf("generateWorkspaceFiles() = %v", err)
	}

	for _, rel := range []string{"one.manifest.json", ".gitignore", "apps", "services", "packages"} {
		if _, err := os.Stat(filepath.Join(target, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	for _, rel := range []string{"package.json", "pnpm-workspace.yaml", "go.work", ".husky", ".changeset", "commitlint.config.js"} {
		if _, err := os.Stat(filepath.Join(target, rel)); !os.IsNotExist(err) {
			t.Fatalf("empty workspace should not create %s", rel)
		}
	}

	// Manifest must be parseable + carry the current minimum: version,
	// workspace identity, and an empty projects array. The current schema dropped
	// top-level packageManager and ai blocks (per-project still has
	// packageManager).
	manifestRaw, err := os.ReadFile(filepath.Join(target, "one.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(manifestRaw, &m); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}
	if v, _ := m["version"].(float64); v != float64(workspace.ManifestVersion) {
		t.Errorf("manifest version=%v; want %d", m["version"], workspace.ManifestVersion)
	}
	if _, has := m["packageManager"]; has {
		t.Errorf("manifest must not carry top-level packageManager; got %v", m["packageManager"])
	}
	if _, has := m["ai"]; has {
		t.Errorf("manifest must not carry top-level ai block; got %v", m["ai"])
	}

	// workspace block must be set at scaffold time so env init can read
	// the workspace identity without re-prompting the user.
	ws, ok := m["workspace"].(map[string]any)
	if !ok {
		t.Fatalf("manifest has no workspace block; got keys=%v", keys(m))
	}
	if ws["name"] != "demo" {
		t.Errorf("workspace.name = %v; want demo", ws["name"])
	}
	if _, has := ws["roots"]; has {
		t.Errorf("workspace must not carry roots override; got %v", ws["roots"])
	}
	id, _ := ws["id"].(string)
	if !strings.HasPrefix(id, "demo-") || len(id) != len("demo-")+6 {
		t.Errorf("workspace.id = %q; want shape demo-<6-hex>", id)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestIsDirectoryEmpty(t *testing.T) {
	tmp := t.TempDir()
	empty, err := isDirectoryEmpty(tmp)
	if err != nil || !empty {
		t.Errorf("expected fresh tempdir empty; got empty=%v err=%v", empty, err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "x"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty, err = isDirectoryEmpty(tmp)
	if err != nil || empty {
		t.Errorf("expected non-empty after write; got empty=%v err=%v", empty, err)
	}

	missing, err := isDirectoryEmpty(filepath.Join(tmp, "does-not-exist"))
	if err != nil {
		t.Errorf("missing dir should not error: %v", err)
	}
	if !missing {
		t.Errorf("missing dir should be empty=true")
	}
}
