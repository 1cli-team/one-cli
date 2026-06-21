package cli_test

// E2E coverage of `one create`.
//
// Regression targets:
//   commit 6c6b492 — `one create [dir] + --name`; positional arg is the dir
//   commit 61505b7 — dropped `--overwrite/--ignore`; non-empty target now errors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// expectedScaffoldPaths is the set of files/dirs `one create` must produce
// at the workspace root. Asserting this directly (rather than only via the
// JSON envelope) catches regressions where the envelope is correct but
// scaffold output silently changed.
var expectedScaffoldPaths = []string{
	"apps",
	"services",
	"packages",
	"package.json",
	"pnpm-workspace.yaml",
	"one.manifest.json",
	"commitlint.config.js",
	"AGENTS.md",
	"CLAUDE.md",
	filepath.Join(".one", "agents", "conventions.md"),
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
	claudeRaw, err := os.ReadFile(filepath.Join(target, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if string(claudeRaw) != "Follow ./AGENTS.md\n" {
		t.Errorf("CLAUDE.md should be a pointer to AGENTS.md, got:\n%s", claudeRaw)
	}

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

	// Workspace package.json should reflect the override too.
	pkg, err := os.ReadFile(filepath.Join(target, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	pkgMap := mustParseJSON(t, string(pkg))
	if pkgMap["name"] != "custom-name" {
		t.Errorf("package.json name: want %q, got %v", "custom-name", pkgMap["name"])
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
// post-trim defaults policy: `one create -y` auto-enables env/dotenv
// dev/process is always-on and not persisted; the same goes for
// ci/github-actions. deploy / container are template-driven (registry
// defaults applied at `one add` time), so neither is set here.
func TestSnapshot_E2E_Create_DefaultEnablesUniversalSet(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	target := filepath.Join(tmp, "ws")
	stdout, stderr, code := runBinary(t, "create", target, "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["secrets_backend"] != "dotenv" {
		t.Errorf("secrets_backend: want dotenv, got %v", got["secrets_backend"])
	}
	if got["ci_enabled"] != true {
		t.Errorf("ci_enabled: want true, got %v", got["ci_enabled"])
	}
	if got["dev_enabled"] != true {
		t.Errorf("dev_enabled: want true, got %v", got["dev_enabled"])
	}

	// Current manifest: env backend lives under domains.env.kind; ci / dev are
	// always-on and have no on-disk representation.
	mf := readManifest(t, target)
	if _, has := mf["plugins"]; has {
		t.Errorf("manifest should not carry legacy plugins map, got %v", mf["plugins"])
	}
	domains, _ := mf["domains"].(map[string]any)
	envSec, _ := domains["env"].(map[string]any)
	if envSec["kind"] != "dotenv" {
		t.Errorf("manifest.domains.env.kind: want dotenv, got %v", domains["env"])
	}
	for _, removed := range []string{"ci", "dev"} {
		if _, has := mf[removed]; has {
			t.Errorf("manifest must not carry top-level %q, got %v", removed, mf[removed])
		}
	}
}
