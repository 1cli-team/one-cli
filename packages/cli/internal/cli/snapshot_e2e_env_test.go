package cli_test

// E2E coverage of `one env`. Post-profile-refactor: endpoint setup
// moved to `one configure add env/infisical`, so this file only locks
// the help surface + offline gates. Profile-driven verbs (set / get /
// list / pull) need a real or stubbed Infisical server, covered by
// unit tests in internal/secrets/infisical/.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshot_E2E_Env_HelpListsSubcommands(t *testing.T) {
	stdout, _, code := runBinary(t, "env", "--help")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// Each Available Command must show up in --help. Drift here means
	// somebody renamed or removed a subcommand without updating callers.
	// `init` was removed in the profile refactor; `profile` parent is
	// the new entry point for endpoint setup.
	for _, sub := range []string{"set", "get", "list", "pull", "profile"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("`one env --help` does not mention subcommand %q", sub)
		}
	}
}

// TestSnapshot_E2E_Env_DotenvBackend_SetWritesToOverlay locks the
// post-v0.7 contract for env/dotenv set: writing creates the per-env
// overlay file (.env.<env>) with the supplied key + value, and the
// JSON envelope reports schema=one-cli/env-set/v1 with action=created.
func TestSnapshot_E2E_Env_DotenvBackend_SetWritesToOverlay(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	subRel := "apps/web"
	subDir := filepath.Join(ws, subRel)
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir subproject: %v", err)
	}
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
		"env", "set", "FOO", "bar",
		"-p", subRel, "--env", "dev",
		"-o", "json",
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/env-set/v1" {
		t.Errorf("schema: want one-cli/env-set/v1, got %v", got["schema"])
	}
	if got["action"] != "created" {
		t.Errorf("action: want created, got %v", got["action"])
	}
	body, err := os.ReadFile(filepath.Join(subDir, ".env.dev"))
	if err != nil {
		t.Fatalf("read .env.dev: %v", err)
	}
	if !strings.Contains(string(body), "FOO=bar") {
		t.Errorf(".env.dev missing expected line; got: %q", body)
	}
}
