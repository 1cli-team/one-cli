package cli_test

// E2E coverage of `one create --preset <id>` (the v3 envelope path).
//
// The non-preset `one create` path keeps emitting create/v2 envelopes
// and is covered by snapshot_e2e_create_test.go; touching this file
// must not bleed back into create-default.json.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// presetFullstackPaths is the subset of expected paths a fullstack
// preset must produce on top of the workspace skeleton. We check
// services/ + apps/ have the right subdirs without enumerating every
// file in each template (template content is exercised by its own
// tests).
var presetFullstackPaths = []string{
	"services/go-api",
	"services/go-api/Taskfile.yml",
	"apps/nextjs-app",
	"one.manifest.json",
}

// TestSnapshot_E2E_Create_Preset_Fullstack covers the happy path:
// `--preset 1.bgok.fnav` → workspace + go-api@kustomize + nextjs-app@vercel.
// Asserts the v3 envelope shape and the on-disk project tree.
func TestSnapshot_E2E_Create_Preset_Fullstack(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "fs")
	stdout, stderr, code := runBinary(t, "create", target, "--preset", "1.bgo.fna", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}

	got := mustParseJSON(t, stdout)
	assertSnapshot(t, "create-preset-fullstack.json", got)
	assertWorkspaceAgentDocs(t, target)

	for _, p := range presetFullstackPaths {
		full := filepath.Join(target, p)
		if !fileExists(t, full) {
			t.Errorf("expected scaffold path missing: %s", full)
		}
	}

	rawManifest, err := os.ReadFile(filepath.Join(target, "one.manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		Projects []struct {
			Name    string `json:"name"`
			Domains *struct {
				Container *struct {
					Kind string `json:"kind"`
				} `json:"container,omitempty"`
			} `json:"domains,omitempty"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	foundGoAPI := false
	for _, project := range manifest.Projects {
		if project.Name == "go-api" {
			foundGoAPI = true
			if project.Domains != nil && project.Domains.Container != nil {
				t.Fatalf("go-api includes retired container domain: %s", rawManifest)
			}
		}
	}
	if !foundGoAPI {
		t.Fatalf("go-api project missing from manifest: %s", rawManifest)
	}
}

// TestSnapshot_E2E_Create_Preset_InvalidNoProject locks the
// PRESET_INVALID path for a structurally-broken id (version only,
// no project segments). The contract: zero filesystem mutation,
// structured error on stderr.
func TestSnapshot_E2E_Create_Preset_InvalidNoProject(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "bad")
	_, stderr, code := runBinary(t, "create", target, "--preset", "1", "-y", "-o", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0\n  stderr: %s", stderr)
	}

	envelope := firstJSONLine(stderr)
	if envelope == "" {
		t.Fatalf("expected JSON error envelope on stderr, got: %q", stderr)
	}
	got := mustParseJSON(t, envelope)
	errMap, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("envelope missing error object: %s", envelope)
	}
	if errMap["code"] != "PRESET_INVALID" {
		t.Errorf("expected error.code=PRESET_INVALID, got %v", errMap["code"])
	}
	assertSnapshot(t, "create-preset-invalid.json", got)

	if fileExists(t, target) {
		t.Errorf("pre-flight failure must not create target dir: %s", target)
	}
}

// TestSnapshot_E2E_Create_Preset_RejectsRetiredDotenv locks the
// PRESET_FLAG_CONFLICT path: preset declares env=dotenv but the user
// also passed --env-provider infisical.
func TestSnapshot_E2E_Create_Preset_RejectsRetiredDotenv(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "bad")
	_, stderr, code := runBinary(t, "create", target,
		"--preset", "1.bgo.ed",
		"-y", "-o", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0\n  stderr: %s", stderr)
	}

	envelope := firstJSONLine(stderr)
	got := mustParseJSON(t, envelope)
	errMap, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("envelope missing error object: %s", envelope)
	}
	if errMap["code"] != "BACKEND_ID_UNKNOWN" {
		t.Errorf("expected error.code=BACKEND_ID_UNKNOWN, got %v", errMap["code"])
	}
}

// TestSnapshot_E2E_Create_Preset_AcceptsPrefix verifies that an id
// pasted with the optional `preset:` prefix produces the same result
// (the canonical envelope still echoes the bare form).
func TestSnapshot_E2E_Create_Preset_AcceptsPrefix(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "ws")
	stdout, stderr, code := runBinary(t, "create", target, "--preset", "preset:1.bgo", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\n  stderr: %s", code, stderr)
	}

	got := mustParseJSON(t, stdout)
	presetMap, _ := got["preset"].(map[string]any)
	if presetMap["id"] != "1.bgo" {
		t.Errorf("preset.id: want %q (canonical, no prefix), got %v", "1.bgo", presetMap["id"])
	}
}

// TestSnapshot_E2E_Create_Preset_ProjectNames verifies --project-names
// overrides every generated subproject name in preset apply order.
func TestSnapshot_E2E_Create_Preset_ProjectNames(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "custom-names")
	stdout, stderr, code := runBinary(t, "create", target,
		"--preset", "1.bne.frs.ltl.ei",
		"--project-names", "api,admin,shared",
		"-y", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}

	got := mustParseJSON(t, stdout)
	projectsRaw, ok := got["projects"].([]any)
	if !ok {
		t.Fatalf("result missing projects array: %s", stdout)
	}
	var gotNames []string
	for _, item := range projectsRaw {
		project, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("project item is not object: %#v", item)
		}
		gotNames = append(gotNames, project["name"].(string))
	}
	wantNames := []string{"api", "admin", "shared"}
	if len(gotNames) != len(wantNames) {
		t.Fatalf("project names = %v, want %v", gotNames, wantNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("project names = %v, want %v", gotNames, wantNames)
		}
	}

	for _, p := range []string{"services/api", "apps/admin", "packages/shared"} {
		if !fileExists(t, filepath.Join(target, p)) {
			t.Fatalf("expected custom project path missing: %s", p)
		}
	}
}

func TestSnapshot_E2E_Create_Preset_ProjectNamesCountMismatch(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "bad-names")
	_, stderr, code := runBinary(t, "create", target,
		"--preset", "1.bne.frs.ed",
		"--project-names", "api",
		"-y", "-o", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0\n  stderr: %s", stderr)
	}

	envelope := firstJSONLine(stderr)
	got := mustParseJSON(t, envelope)
	errMap, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("envelope missing error object: %s", envelope)
	}
	if errMap["code"] != "PRESET_INVALID" {
		t.Errorf("expected error.code=PRESET_INVALID, got %v", errMap["code"])
	}
	if fileExists(t, target) {
		t.Errorf("pre-flight failure must not create target dir: %s", target)
	}
}

func TestSnapshot_E2E_Create_Preset_ProjectNamesInvalidName(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "bad-name")
	_, stderr, code := runBinary(t, "create", target,
		"--preset", "1.bne.frs.ed",
		"--project-names", "api,not valid",
		"-y", "-o", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0\n  stderr: %s", stderr)
	}

	envelope := firstJSONLine(stderr)
	got := mustParseJSON(t, envelope)
	errMap, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("envelope missing error object: %s", envelope)
	}
	if errMap["code"] != "INVALID_NAME" {
		t.Errorf("expected error.code=INVALID_NAME, got %v", errMap["code"])
	}
	if fileExists(t, target) {
		t.Errorf("pre-flight failure must not create target dir: %s", target)
	}
}

// TestSnapshot_E2E_Create_Preset_RefusesMissingDir locks the
// PROJECT_NAME_REQUIRED path: `--preset` without a dir must error,
// not drop into a TTY prompt.
func TestSnapshot_E2E_Create_Preset_RefusesMissingDir(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	_, stderr, code := runBinary(t, "create", "--preset", "1.bgo", "-y", "-o", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0\n  stderr: %s", stderr)
	}

	envelope := firstJSONLine(stderr)
	got := mustParseJSON(t, envelope)
	errMap, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("envelope missing error object: %s", envelope)
	}
	if errMap["code"] != "PROJECT_NAME_REQUIRED" {
		t.Errorf("expected error.code=PROJECT_NAME_REQUIRED, got %v", errMap["code"])
	}
	_ = tmp
}
