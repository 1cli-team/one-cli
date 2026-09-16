package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/resources/bundled"
)

func TestSkillsInstallExplicitOfflineAndRepeated(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	// The installer uses embedded bytes; unavailable proxies must not affect it.
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	for range 2 {
		stdout, stderr, code := runBinaryIn(t, home, "skills", "install", "--agent", "codex", "-a", "claude-code", "-a", "codex", "-o", "json")
		if code != 0 || stderr != "" {
			t.Fatalf("install failed: %d %s %s", code, stdout, stderr)
		}
		got := mustParseJSON(t, stdout)
		if got["schema"] != "one-cli/skills-install/v1" || got["status"] != "completed" || got["skill_count"] != float64(1) {
			t.Fatalf("unexpected result: %v", got)
		}
		if len(got["targets"].([]any)) != 2 || len(got["installed_to"].([]any)) != 2 {
			t.Fatalf("duplicate targets: %v", got)
		}
		for _, dir := range []string{".codex", ".claude"} {
			body, err := os.ReadFile(filepath.Join(home, dir, "skills", "one-cli", "SKILL.md"))
			if err != nil || string(body) != string(bundled.OneCLISkill) {
				t.Fatalf("incorrect installed skill: %v", err)
			}
		}
	}
	for _, dir := range []string{".one", ".cursor", "one.manifest.json", "AGENTS.md"} {
		if fileExists(t, filepath.Join(home, dir)) {
			t.Fatalf("install unexpectedly wrote %s", dir)
		}
	}
}

func TestSkillsInstallDetectionAndErrors(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	for _, args := range [][]string{
		{"skills", "install", "--yes", "-o", "json"},
		{"skills", "install", "--agent", "cursor", "--agent", "invalid-agent", "-o", "json"},
	} {
		_, stderr, code := runBinaryIn(t, home, args...)
		if code != 1 {
			t.Fatalf("expected failure: %d %s", code, stderr)
		}
		got := mustParseJSON(t, firstJSONLine(stderr))
		if got["error"].(map[string]any)["code"] != "SKILLS_INSTALL_FAILED" {
			t.Fatalf("unexpected error: %v", got)
		}
		entries, err := os.ReadDir(home)
		if err != nil || len(entries) != 0 {
			t.Fatalf("target resolution wrote files: %v %v", entries, err)
		}
	}
	if err := os.Mkdir(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Non-TTY execution without --yes follows the same detection policy.
	for _, args := range [][]string{{"skills", "install", "--yes", "-o", "json"}, {"skills", "install", "-o", "json"}} {
		stdout, stderr, code := runBinaryIn(t, home, args...)
		if code != 0 {
			t.Fatalf("detected install failed: %d %s %s", code, stdout, stderr)
		}
		got := mustParseJSON(t, stdout)
		targets := got["targets"].([]any)
		if len(targets) != 1 || targets[0].(map[string]any)["agent_id"] != "cursor" {
			t.Fatalf("wrong detection: %v", got)
		}
	}
}

func TestSkillsHelpDoesNotInstall(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	for _, args := range [][]string{{"skills"}, {"skills", "install", "--help"}} {
		stdout, stderr, code := runBinaryIn(t, home, args...)
		if code != 0 || stderr != "" || !strings.Contains(stdout, "install") {
			t.Fatalf("help failed: %d %s %s", code, stdout, stderr)
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("help wrote files: %v %v", entries, err)
	}
}
