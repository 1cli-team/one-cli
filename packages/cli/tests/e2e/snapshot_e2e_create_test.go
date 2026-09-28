package cli_test

// E2E coverage of `one create`.
//
// Regression targets:
//   commit 6c6b492 — `one create [dir] + --name`; positional arg is the dir
//   commit 61505b7 — dropped `--overwrite/--ignore`; non-empty target now errors

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func assertNoAgentDocs(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Name() == "AGENTS.md" || entry.Name() == "CLAUDE.md" ||
			entry.Name() == ".one" {
			t.Errorf("unexpected generated agent file or metadata directory: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// expectedScaffoldPaths is the set of files/dirs `one create` must produce
// at the workspace root. Asserting this directly (rather than only via the
// JSON envelope) catches regressions where the envelope is correct but
// scaffold output silently changed.
var expectedScaffoldPaths = []string{
	"apps",
	"services",
	"packages",
	"one.manifest.json",
	"AGENTS.md",
	"mise.toml",
	".config/hk.pkl",
	".git/hooks/pre-commit",
	".git/hooks/commit-msg",
}

func TestSnapshot_E2E_Create_Default(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "my-app")
	stdout, stderr, code := runBinary(t, "create", target, "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}

	got := mustParseJSON(t, stdout)
	assertSnapshot(t, "create-default.json", got)

	// File-tree shape is part of the contract.
	for _, p := range expectedScaffoldPaths {
		full := filepath.Join(target, p)
		if !fileExists(t, full) {
			t.Errorf("expected scaffold path missing: %s", full)
		}
	}
	assertWorkspaceAgentDocs(t, target)

	// Manifest sanity. Schema is the current ManifestVersion.
	mf := readManifest(t, target)
	if v, _ := mf["version"].(float64); v != float64(workspace.ManifestVersion) {
		t.Errorf("manifest version: want %d, got %v", workspace.ManifestVersion, mf["version"])
	}
	if subs, ok := mf["projects"].([]any); !ok || len(subs) != 0 {
		t.Errorf("fresh manifest should have empty projects, got %v", mf["projects"])
	}
}

func TestSnapshot_E2E_Create_NameOverride(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "services", "billing")
	stdout, stderr, code := runBinary(t, "create", target, "--name", "custom-name", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\n  stderr: %s", code, stderr)
	}

	got := mustParseJSON(t, stdout)
	if got["project_name"] != "custom-name" {
		t.Errorf("project_name override: want %q, got %v", "custom-name", got["project_name"])
	}

	// The language-neutral workspace keeps its name in the manifest.
	mf := readManifest(t, target)
	if mf["workspace"].(map[string]any)["name"] != "custom-name" {
		t.Fatal("workspace name override was lost")
	}
	if fileExists(t, filepath.Join(target, "package.json")) {
		t.Fatal("empty workspace unexpectedly initialized Node")
	}

}

func TestSnapshot_E2E_Create_NonEmptyTargetFails(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "occupied")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "preexisting.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write preexisting: %v", err)
	}

	_, stderr, code := runBinary(t, "create", target, "-y", "-o", "json")
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
	if errMap["code"] != "EXISTING_TARGET_NOT_EMPTY" {
		t.Errorf("expected error.code=EXISTING_TARGET_NOT_EMPTY (regression for commit 61505b7), got %v", errMap["code"])
	}
	assertSnapshot(t, "create-non-empty-error.json", got)
}

// TestSnapshot_E2E_Create_NestedInsideWorkspace_Refused locks the guard:
// `one create` must refuse to plant a workspace inside a directory that
// already has a one.manifest.json anywhere in its ancestry. Without this,
// two manifests in the same tree silently break env/add discovery.
func TestSnapshot_E2E_Create_NestedInsideWorkspace_Refused(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	outer := bootstrapWorkspace(t, tmp, "outer")

	// Try to create a NEW workspace path that lives inside `outer`. The
	// target dir doesn't exist yet, so this isn't EXISTING_TARGET_NOT_EMPTY —
	// only the new nesting guard would catch it.
	nested := filepath.Join(outer, "packages", "inner")
	_, stderr, code := runBinary(t, "create", nested, "-y", "-o", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit when creating inside an existing workspace, got 0\n  stderr: %s", stderr)
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
	if errMap["code"] != "WORKSPACE_NESTED_FORBIDDEN" {
		t.Errorf("expected error.code=WORKSPACE_NESTED_FORBIDDEN, got %v", errMap["code"])
	}

	// And the variant: target IS an existing workspace (re-init attempt).
	_, stderr, code = runBinary(t, "create", outer, "-y", "-o", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit when target is already a workspace, got 0\n  stderr: %s", stderr)
	}
	envelope = firstJSONLine(stderr)
	got = mustParseJSON(t, envelope)
	errMap = got["error"].(map[string]any)
	// The non-empty target check fires first here (workspace dir has files);
	// either guard is acceptable as long as it's a refusal, not a silent
	// re-scaffold. The contract that matters: code != 0 + structured error.
	if errMap["code"] == "" {
		t.Errorf("expected a refusal envelope, got empty code")
	}
}

// TestSnapshot_E2E_Create_DefaultEnablesUniversalSet verifies the
// post-trim defaults policy: `one create -y` enables env/dotenv and local
// development, but does not enable CI. deploy / container are also deferred.
func TestSnapshot_E2E_Create_DefaultEnablesUniversalSet(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "ws")
	stdout, stderr, code := runBinary(t, "create", target, "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["secrets_backend"] != "infisical" {
		t.Errorf("secrets_backend: want infisical, got %v", got["secrets_backend"])
	}
	if _, exists := got["ci_enabled"]; exists {
		t.Error("creation still exposes the removed CI feature")
	}
	if got["dev_enabled"] != true {
		t.Errorf("dev_enabled: want true, got %v", got["dev_enabled"])
	}

	// Current manifest: env backend lives under domains.env.kind; ci / dev have
	// no on-disk representation. CI is not enabled implicitly.
	mf := readManifest(t, target)
	if _, has := mf["plugins"]; has {
		t.Errorf("manifest should not carry legacy plugins map, got %v", mf["plugins"])
	}
	if _, exists := mf["env"]; exists {
		t.Fatal("creation unexpectedly enabled remote injection before binding")
	}
	if _, exists := mf["domains"]; exists {
		t.Fatal("creation wrote retired domains")
	}
	for _, removed := range []string{"ci", "dev"} {
		if _, has := mf[removed]; has {
			t.Errorf("manifest must not carry top-level %q, got %v", removed, mf[removed])
		}
	}
}

func assertWorkspaceAgentDocs(t *testing.T, root string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "one run") || !strings.Contains(string(body), "one exec") {
		t.Fatalf("workspace guidance does not explain skill installation: %s", body)
	}
	for _, entry := range []string{"apps", "services", "packages"} {
		assertNoAgentDocs(t, filepath.Join(root, entry))
	}
	for _, entry := range []string{"CLAUDE.md", ".one", ".agents"} {
		if _, err := os.Stat(filepath.Join(root, entry)); !os.IsNotExist(err) {
			t.Fatalf("unexpected generated %s: %v", entry, err)
		}
	}
}
