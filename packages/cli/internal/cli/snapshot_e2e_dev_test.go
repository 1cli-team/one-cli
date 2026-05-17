package cli_test

// E2E coverage of `one dev`. Dev is a leaf verb (no subcommands) that
// reads projects[].domains.dev.command from the manifest and dispatches
// each project to the built-in supervisor. Procfile.dev / external
// runners (overmind etc.) are no longer involved.
//
// The surviving tests lock the subcommand-removal contract, the
// project-selector error envelope, and the manifest-driven start path.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshot_E2E_DevRejectsRemovedSubcommands(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")
	for _, sub := range []string{"start", "path"} {
		_, stderr, code := runBinaryIn(t, ws, "dev", sub)
		if code == 0 {
			t.Fatalf("expected `one dev %s` to fail post-v0.8 (subcommand removed)", sub)
		}
		if !strings.Contains(stderr, "unknown command") {
			t.Fatalf("expected cobra unknown-command diagnostic for `dev %s`, got: %s", sub, stderr)
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

// TestSnapshot_E2E_DevFromManifest exercises the new manifest-driven
// dev path: `one add` writes projects[].domains.dev.command, `one dev`
// reads that and runs the built-in supervisor. We override the command
// to a trivial echo to keep the test fast and deterministic — the real
// `pnpm run dev` / `go run` paths are out of scope for an integration
// smoke test.
func TestSnapshot_E2E_DevFromManifest(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	if _, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json"); code != 0 {
		t.Fatalf("add api failed: %d\n  stderr: %s", code, stderr)
	}

	// Patch the resolved dev command to a self-contained echo. The
	// scaffolded default (`go run ./cmd/server`) would block forever in
	// the test environment.
	overrideDevCommand(t, ws, "api", "echo built-in-supervisor-works")

	stdout, stderr, code := runBinaryIn(t, ws, "dev", "-o", "json")
	if code != 0 {
		t.Fatalf("dev failed: %d\n  stderr: %s\n  stdout: %s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "built-in-supervisor-works") {
		t.Errorf("supervisor output missing echo line in stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, "api | ") && !strings.Contains(stdout, "api|") {
		t.Errorf("supervisor output missing api prefix in stdout:\n%s", stdout)
	}
	envelope := firstJSONLine(stdout)
	if envelope == "" {
		t.Fatalf("no JSON envelope in stdout:\n%s", stdout)
	}
	res := mustParseJSON(t, envelope)
	if res["runner"] != "builtin" {
		t.Errorf("envelope runner = %v, want %q", res["runner"], "builtin")
	}
	if res["schema"] != "one-cli/dev-start/v1" {
		t.Errorf("schema drift: %v", res["schema"])
	}
}

// TestSnapshot_E2E_DevManifestStoresDevCommand asserts the schema
// invariant: after `one add`, the manifest contains a non-empty
// projects[].domains.dev.command for the new project. This lock the
// contract so a future change can't quietly stop persisting the field.
func TestSnapshot_E2E_DevManifestStoresDevCommand(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	if _, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json"); code != 0 {
		t.Fatalf("add api failed: %d\n  stderr: %s", code, stderr)
	}
	if got := readDevCommandFromManifest(t, ws, "api"); got == "" {
		t.Fatalf("expected projects[api].domains.dev.command to be non-empty after `one add`")
	}

	// Procfile.dev must NOT have been written.
	if _, err := os.Stat(filepath.Join(ws, "Procfile.dev")); !os.IsNotExist(err) {
		t.Errorf("Procfile.dev should no longer be written, but stat returned err=%v", err)
	}
}

// overrideDevCommand directly patches one.manifest.json to set the
// dev command on a named project. Used by tests that want a
// deterministic, fast-exiting child rather than the real toolchain
// default.
func overrideDevCommand(t *testing.T, workspaceRoot, projectName, cmd string) {
	t.Helper()
	manifestPath := filepath.Join(workspaceRoot, "one.manifest.json")
	raw, err := os.ReadFile(manifestPath)
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
		domains, _ := p["domains"].(map[string]any)
		if domains == nil {
			domains = map[string]any{}
			p["domains"] = domains
		}
		domains["dev"] = map[string]any{"command": cmd}
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, append(out, '\n'), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
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
		domains, _ := p["domains"].(map[string]any)
		if domains == nil {
			return ""
		}
		dev, _ := domains["dev"].(map[string]any)
		if dev == nil {
			return ""
		}
		cmd, _ := dev["command"].(string)
		return cmd
	}
	return ""
}
