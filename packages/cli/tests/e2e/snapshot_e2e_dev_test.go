package cli_test

// Development uses the same mise graph and result protocol as other tasks.

import (
	"encoding/json"
	"github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshot_E2E_DevPositionalSelectorUnknown(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")
	for _, sub := range []string{"start", "path"} {
		_, stderr, code := runBinaryIn(t, ws, "dev", sub, "-o", "json")
		if code == 0 {
			t.Fatalf("expected unknown positional project %q to fail", sub)
		}
		got := mustParseJSON(t, firstJSONLine(stderr))
		if !strings.Contains(got["error"].(map[string]any)["message"].(string), "-p") {
			t.Fatalf("expected project flag guidance for `one dev %s`, got: %s", sub, stderr)
		}
	}
}

func TestSnapshot_E2E_DevProjectSelectorUnknown(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add api failed: exit %d\n  stderr: %s", code, stderr)
	}

	t.Setenv("ONE_MISE_BINARY", filepath.Join(ws, "missing-mise"))
	_, stderr, code = runBinaryIn(t, ws, "dev", "-p", "nonexistent", "-o", "json")
	if code == 0 {
		t.Fatal("expected `one dev -p nonexistent` to fail")
	}
	got := mustParseJSON(t, firstJSONLine(stderr))
	errMap := got["error"].(map[string]any)
	if errMap["code"] != "SUBPROJECT_NOT_FOUND" {
		t.Fatalf("expected SUBPROJECT_NOT_FOUND, got %v", got)
	}
	ctx := errMap["context"].(map[string]any)
	available, _ := ctx["available_projects"].([]any)
	if len(available) == 0 {
		t.Fatalf("envelope should list available_projects, got %v", errMap)
	}
}

func TestSnapshot_E2E_DevFromMise(t *testing.T) {
	ws := buildFixture(t, true)
	overrideDevCommand(t, ws, "lib", "echo manifest-development")
	stdout, stderr, code := runBinaryIn(t, ws, "dev", "-p", "lib", "-o", "json")
	if code != 0 || !strings.Contains(stderr, "manifest-development") || !json.Valid([]byte(stdout)) {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	result := mustParseJSON(t, stdout)
	if result["schema"] != "one-cli/task-result/v1" || result["status"] != "succeeded" {
		t.Fatal(result)
	}
}

// TestSnapshot_E2E_ManifestDoesNotStoreDevCommand asserts the schema
// invariant: after `one add`, the manifest contains a non-empty
// projects[].domains.dev.command for the new project. This lock the
// contract so a future change can't quietly stop persisting the field.
func TestSnapshot_E2E_ManifestDoesNotStoreDevCommand(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	if _, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json"); code != 0 {
		t.Fatalf("add api failed: %d\n  stderr: %s", code, stderr)
	}
	if got := readDevCommandFromManifest(t, ws, "api"); got != "" {
		t.Fatalf("one add persisted the retired dev command")
	}

	// Procfile.dev must NOT have been written.
	if _, err := os.Stat(filepath.Join(ws, "Procfile.dev")); !os.IsNotExist(err) {
		t.Errorf("Procfile.dev should no longer be written, but stat returned err=%v", err)
	}
}

// overrideDevCommand defines a native mise task with a deterministic child.
func overrideDevCommand(t *testing.T, root, projectName, command string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "one.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Projects []struct {
			Name      string `json:"name"`
			Directory string `json:"relativeDir"`
		} `json:"projects"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, project := range manifest.Projects {
		if project.Name == projectName {
			raw, err = toml.Marshal(map[string]any{"tasks": map[string]any{projectName + ":dev": map[string]any{"dir": project.Directory, "run": command, "run_windows": command, "raw_args": true}}})
			if err != nil {
				t.Fatal(err)
			}
			appendRootTaskConfig(t, root, string(raw))
			return
		}
	}
	t.Fatal("project missing")
}

func readDevCommandFromManifest(t *testing.T, workspaceRoot, projectName string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(workspaceRoot, "one.manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	projects, _ := m["projects"].([]any)
	for _, raw := range projects {
		p, _ := raw.(map[string]any)
		if p["name"] != projectName {
			continue
		}
		dev, _ := p["dev"].(map[string]any)
		if dev == nil {
			return ""
		}
		cmd, _ := dev["command"].(string)
		return cmd
	}
	return ""
}
