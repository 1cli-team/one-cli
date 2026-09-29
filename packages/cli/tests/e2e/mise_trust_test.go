package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_CreateAndAddTrustGeneratedMiseFiles(t *testing.T) {
	mise, err := exec.LookPath("mise")
	if err != nil {
		t.Skip("real mise is required")
	}
	temp := t.TempDir()
	isolateHome(t, filepath.Join(temp, "home"))
	t.Setenv("ONE_MISE_BINARY", mise)
	t.Setenv("MISE_PARANOID", "1")
	t.Setenv("MISE_TRUSTED_CONFIG_PATHS", "")
	// Both mise-action and mise's CI mode enable automatic confirmation,
	// bypassing the trust checks this test observes even in paranoid mode.
	t.Setenv("MISE_YES", "0")
	t.Setenv("CI", "false")
	parent := filepath.Join(temp, "workspaces")
	buildWrite(t, parent, "mise.toml", "[env]\nPARENT_USER_CONFIG='unchanged'\n")
	checkTrust := func(path string, want bool) {
		t.Helper()
		command := exec.Command(mise, "trust", "--show", path)
		command.Dir, command.Env = filepath.Dir(path), os.Environ()
		out, err := command.CombinedOutput()
		trusted := strings.HasSuffix(strings.TrimSpace(string(out)), ": trusted")
		if err != nil || trusted != want {
			t.Fatalf("trust(%s)=%s, %v; want %v", path, out, err, want)
		}
	}
	run := func(root string, args ...string) {
		t.Helper()
		stdout, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 || strings.Contains(stdout, "\"warnings\"") {
			t.Fatalf("%v: %d %s %s", args, code, stdout, stderr)
		}
	}
	ws := filepath.Join(parent, "demo")
	run(parent, "create", ws, "--yes", "-o", "json")
	checkTrust(filepath.Join(ws, "mise.toml"), true)
	checkTrust(filepath.Join(parent, "mise.toml"), false)
	run(ws, "add", "react-spa", "--name", "web", "--yes", "-o", "json")
	checkTrust(filepath.Join(ws, "mise.toml"), true)
	if fileExists(t, filepath.Join(ws, "apps/web/mise.toml")) {
		t.Fatal("unexpected project mise configuration")
	}
	// Adding a project must not grant trust to unrelated custom configuration.
	path := filepath.Join(ws, "mise.toml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	buildWrite(t, ws, "mise.toml", string(raw)+"\n[env]\nUSER_OVERRIDE='preserve'\n")
	checkTrust(path, false)
	run(ws, "add", "ts-library", "--name", "shared", "--yes", "-o", "json")
	checkTrust(path, false)
	if fileExists(t, filepath.Join(ws, "packages/shared/mise.toml")) {
		t.Fatal("unexpected project mise configuration")
	}
	multi := filepath.Join(parent, "multi")
	run(parent, "create", multi, "--yes", "-o", "json")
	run(multi, "add", "react-spa", "--name", "web", "--yes", "-o", "json")
	run(multi, "add", "ts-library", "--name", "shared", "--yes", "-o", "json")
	checkTrust(filepath.Join(multi, "mise.toml"), true)
	if fileExists(t, filepath.Join(multi, "apps/web/mise.toml")) {
		t.Fatal("unexpected project mise configuration")
	}
	if fileExists(t, filepath.Join(multi, "packages/shared/mise.toml")) {
		t.Fatal("unexpected project mise configuration")
	}
}

func TestE2E_CreatePreservesFilesWhenMiseTrustFails(t *testing.T) {
	for _, locale := range []string{"en_US.UTF-8", "zh_CN.UTF-8"} {
		t.Run(locale, func(t *testing.T) {
			temp := t.TempDir()
			isolateHome(t, filepath.Join(temp, "home"))
			t.Setenv("LC_ALL", locale)
			t.Setenv("ONE_MISE_BINARY", filepath.Join(temp, "missing-mise"))
			root := filepath.Join(temp, "demo")
			stdout, stderr, code := runBinary(t, "create", root, "--yes", "-o", "json")
			if code != 0 {
				t.Fatalf("creation failed: %d %s %s", code, stdout, stderr)
			}
			result := mustParseJSON(t, stdout)
			warnings, ok := result["warnings"].([]any)
			if !ok || len(warnings) != 1 || !strings.Contains(warnings[0].(string), "one mise trust") {
				t.Fatal(result)
			}
			if strings.HasPrefix(locale, "zh") && !strings.Contains(warnings[0].(string), "自动信任") {
				t.Fatal(warnings)
			}
			if !fileExists(t, filepath.Join(root, "mise.toml")) || !fileExists(t, filepath.Join(root, "one.manifest.toml")) {
				t.Fatal("creation was rolled back")
			}
		})
	}
}
