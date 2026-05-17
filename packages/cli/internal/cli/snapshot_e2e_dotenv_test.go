package cli_test

// E2E coverage of `one env list / get` dispatched to the env/dotenv
// backend. Post capability-interface refactor: `one dotenv` is gone;
// the command is now `one env <verb>` and dispatches to whichever
// pkgplugin.EnvBackend is active in manifest.plugins.env (env/dotenv
// is the default).
//
// These tests stand on their own filesystem fixtures — no Infisical
// network calls. dotenv is by design always available, which makes
// it the easiest second-provider proof for the secrets Loader registry.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// (`TestSnapshot_E2E_Env_HelpListsSubcommands` is in
// snapshot_e2e_env_test.go; not duplicated here.)

func TestSnapshot_E2E_Env_DotenvBackend_ListReadsSubprojectEnvFile(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	// Drop a fake subproject with a .env file. We don't go through
	// `one add` — that'd pull in template renderers and slow the
	// test for no reason. dotenv only needs a manifest entry pointing
	// at a directory containing .env.
	subRel := "apps/web"
	subDir := filepath.Join(ws, subRel)
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir subproject: %v", err)
	}
	envContent := "# header comment\nFOO=bar\nBAZ=quux\n"
	if err := os.WriteFile(filepath.Join(subDir, ".env"), []byte(envContent), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	// Hand-edit the manifest to register the subproject.
	manifestPath := filepath.Join(ws, "one.manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	patched := strings.Replace(string(raw),
		`"projects": []`,
		`"projects": [{"name":"web","relativeDir":"`+subRel+`","templateId":"react-spa","toolchain":"node","buildVersion":"0.1.0"}]`,
		1)
	if patched == string(raw) {
		t.Fatalf("could not find subprojects array in manifest: %s", raw)
	}
	if err := os.WriteFile(manifestPath, []byte(patched), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	stdout, stderr, code := runBinaryIn(t, ws,
		"env", "list", "-p", subRel, "-o", "json",
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/env-list/v1" {
		t.Errorf("schema: want one-cli/env-list/v1, got %v", got["schema"])
	}
	keys, ok := got["keys"].([]any)
	if !ok {
		t.Fatalf("keys: want []any, got %T", got["keys"])
	}
	if len(keys) != 2 {
		t.Errorf("keys: want 2 entries, got %d (%v)", len(keys), keys)
	}
	// Sorted alphabetically by the command, so order is stable.
	if keys[0] != "BAZ" || keys[1] != "FOO" {
		t.Errorf("keys order: want [BAZ FOO], got %v", keys)
	}
}

func TestSnapshot_E2E_Env_DotenvBackend_GetMissingKeyReturnsStructuredError(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	subRel := "apps/web"
	subDir := filepath.Join(ws, subRel)
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, ".env"), []byte("ONLY_KEY=1\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	_, stderr, code := runBinaryIn(t, ws,
		"env", "get", "MISSING_KEY", "-p", subRel, "-o", "json",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing key")
	}
	envelope := firstJSONLine(stderr)
	if envelope == "" {
		t.Fatalf("expected JSON error envelope on stderr, got: %q", stderr)
	}
	got := mustParseJSON(t, envelope)
	if got["schema"] != "one-cli/error/v1" {
		t.Errorf("schema: want one-cli/error/v1, got %v", got["schema"])
	}
	errMap := got["error"].(map[string]any)
	if errMap["code"] != "ENV_KEY_NOT_FOUND" {
		t.Errorf("error.code: want ENV_KEY_NOT_FOUND, got %v", errMap["code"])
	}
}
