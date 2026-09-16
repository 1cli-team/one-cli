package skills

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/resources/bundled"
)

func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, name := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "XDG_CONFIG_HOME", "VIBE_HOME"} {
		t.Setenv(name, "")
	}
	return home
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveTargetsExplicitAndDetected(t *testing.T) {
	home := isolatedHome(t)
	if _, err := ResolveTargets(nil); err == nil {
		t.Fatal("empty home must require an explicit target")
	}
	// A shared skills folder does not identify every agent that can read it.
	writeFile(t, filepath.Join(home, ".agents", "skills", "other", "SKILL.md"), "other")
	if _, err := ResolveTargets(nil); err == nil {
		t.Fatal("shared skills folder must not trigger unrelated agents")
	}
	writeFile(t, filepath.Join(home, ".cursor", "settings.json"), "{}")
	detected, err := ResolveTargets(nil)
	if err != nil || len(detected) != 1 || detected[0].ID != "cursor" {
		t.Fatalf("detected = %v, %v", detected, err)
	}
	explicit, err := ResolveTargets([]string{"claude-code", "codex", "codex"})
	if err != nil || len(explicit) != 2 {
		t.Fatalf("explicit = %v, %v", explicit, err)
	}
	if explicit[1].GlobalPath != filepath.Join(home, ".codex", "skills") {
		t.Fatalf("unexpected destination: %s", explicit[1].GlobalPath)
	}
	if _, err := ResolveTargets([]string{"cursor", "../../outside"}); err == nil {
		t.Fatal("invalid target must fail before installation")
	}
}

func TestAgentRootsAndRegistry(t *testing.T) {
	home := isolatedHome(t)
	for _, tc := range []struct{ id, env, suffix string }{
		{"codex", "CODEX_HOME", "skills"},
		{"claude-code", "CLAUDE_CONFIG_DIR", "skills"},
		{"opencode", "XDG_CONFIG_HOME", "opencode/skills"},
		{"mistral-vibe", "VIBE_HOME", "skills"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			root := filepath.Join(home, "custom", tc.id)
			t.Setenv(tc.env, root)
			targets, err := ResolveTargets([]string{tc.id})
			if err != nil || targets[0].GlobalPath != filepath.Join(root, filepath.FromSlash(tc.suffix)) {
				t.Fatalf("targets = %v, %v", targets, err)
			}
			t.Setenv(tc.env, "relative/path")
			if _, err := ResolveTargets([]string{tc.id}); err == nil {
				t.Fatal("relative configuration override must fail")
			}
		})
	}
	seen := map[string]bool{}
	for _, id := range AgentIDs() {
		if seen[id] {
			t.Fatalf("duplicate agent %s", id)
		}
		seen[id] = true
		targets, err := ResolveTargets([]string{id})
		if err != nil || len(targets) != 1 || !strings.HasPrefix(targets[0].GlobalPath, home+string(filepath.Separator)) {
			t.Fatalf("invalid registry entry %s: %v, %v", id, targets, err)
		}
	}
}

func TestInstallReplacesOnlyOneCLISkill(t *testing.T) {
	home := isolatedHome(t)
	targets, err := ResolveTargets([]string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	root := targets[0].GlobalPath
	other := filepath.Join(root, "other", "SKILL.md")
	writeFile(t, other, "keep me")
	writeFile(t, filepath.Join(root, Name, "references", "obsolete.md"), "old command reference")
	for range 2 {
		paths, err := Install(context.Background(), append(targets, targets[0]))
		if err != nil || !reflect.DeepEqual(paths, []string{root}) {
			t.Fatalf("install = %v, %v", paths, err)
		}
		body, err := os.ReadFile(filepath.Join(root, Name, "SKILL.md"))
		if err != nil || string(body) != string(bundled.OneCLISkill) {
			t.Fatalf("skill = %q, %v", body, err)
		}
		entries, err := os.ReadDir(filepath.Join(root, Name))
		if err != nil || len(entries) != 1 {
			t.Fatalf("obsolete skill files remain: %v, %v", entries, err)
		}
		body, err = os.ReadFile(other)
		if err != nil || string(body) != "keep me" {
			t.Fatalf("other skill changed: %q, %v", body, err)
		}
		entries, err = os.ReadDir(root)
		if err != nil || len(entries) != 2 {
			t.Fatalf("unexpected installation leftovers: %v, %v", entries, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".one")); !os.IsNotExist(err) {
		t.Fatalf("unexpected shared store: %v", err)
	}
}

func TestInstallReplacesLegacySymlinkWithoutChangingStore(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing", true: "broken"}[broken], func(t *testing.T) {
			home := isolatedHome(t)
			targets, err := ResolveTargets([]string{"cursor"})
			if err != nil {
				t.Fatal(err)
			}
			root := targets[0].GlobalPath
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			store := filepath.Join(home, ".one", "skills-store", "one-bundled", Name)
			if !broken {
				writeFile(t, filepath.Join(store, "SKILL.md"), "old shared copy")
			}
			if err := os.Symlink(store, filepath.Join(root, Name)); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if _, err := Install(context.Background(), targets); err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(filepath.Join(root, Name))
			if err != nil || !info.IsDir() {
				t.Fatalf("not a direct installation: %v, %v", info, err)
			}
			body, err := os.ReadFile(filepath.Join(store, "SKILL.md"))
			if broken {
				if !os.IsNotExist(err) {
					t.Fatalf("broken store was created: %v", err)
				}
			} else if err != nil || string(body) != "old shared copy" {
				t.Fatalf("legacy store changed: %q, %v", body, err)
			}
		})
	}
}

func TestInstallReportsPartialFailureAndCancellation(t *testing.T) {
	home := isolatedHome(t)
	good := filepath.Join(home, "good")
	bad := filepath.Join(home, "not-a-directory")
	writeFile(t, bad, "keep me")
	paths, err := Install(context.Background(), []Agent{{ID: "first", GlobalPath: good}, {ID: "second", GlobalPath: bad}})
	if err == nil || !reflect.DeepEqual(paths, []string{good}) {
		t.Fatalf("partial result = %v, %v", paths, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	untouched := filepath.Join(home, "cancelled")
	if _, err := Install(ctx, []Agent{{ID: "third", GlobalPath: untouched}}); err == nil {
		t.Fatal("cancelled install succeeded")
	}
	if _, err := os.Stat(untouched); !os.IsNotExist(err) {
		t.Fatalf("cancelled install wrote files: %v", err)
	}
	root := filepath.Join(home, "collision")
	writeFile(t, filepath.Join(root, Name), "not a skill directory")
	if _, err := Install(context.Background(), []Agent{{ID: "collision", GlobalPath: root}}); err == nil {
		t.Fatal("file collision must fail")
	}
	body, err := os.ReadFile(filepath.Join(root, Name))
	if err != nil || string(body) != "not a skill directory" {
		t.Fatalf("collision lost data: %q, %v", body, err)
	}
}
