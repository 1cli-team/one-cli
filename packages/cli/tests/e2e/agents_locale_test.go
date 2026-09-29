package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAgentGuidanceUsesResolvedLocaleAndRemainsTeamOwned(t *testing.T) {
	for _, tc := range []struct {
		name, preference, terminal, want string
	}{
		{"saved Chinese overrides terminal", "zh-CN", "en_US.UTF-8", "zh-CN"},
		{"saved English overrides terminal", "en-US", "zh_CN.UTF-8", "en-US"},
		{"automatic Chinese", "auto", "zh_CN.UTF-8", "zh-CN"},
		{"automatic fallback", "auto", "de_DE.UTF-8", "en-US"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			isolateHome(t, tmp)
			t.Setenv("LC_ALL", tc.terminal)
			if _, stderr, code := runBinary(t, "locale", tc.preference, "-o", "json"); code != 0 {
				t.Fatalf("set language: exit=%d %s", code, stderr)
			}
			root := bootstrapWorkspace(t, tmp, "demo")
			path := filepath.Join(root, "AGENTS.md")
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "modules", "creation", "templates", "AGENTS."+tc.want+".md"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("AGENTS.md does not match the %s Markdown template", tc.want)
			}
			other := "zh-CN"
			if tc.want == "zh-CN" {
				other = "en-US"
			}
			if _, stderr, code := runBinary(t, "locale", other, "-o", "json"); code != 0 {
				t.Fatalf("switch language: exit=%d %s", code, stderr)
			}
			if _, stderr, code := runBinaryIn(t, root, "add", "empty-app", "--name", "web", "--yes", "-o", "json"); code != 0 {
				t.Fatalf("add after switching language: exit=%d %s", code, stderr)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, after) {
				t.Fatalf("language switch or one add rewrote existing guidance: %v", err)
			}
		})
	}
}
