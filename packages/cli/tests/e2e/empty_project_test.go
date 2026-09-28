package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_AddEmptyProjects(t *testing.T) {
	for _, locale := range []string{"en_US.UTF-8", "zh_CN.UTF-8"} {
		t.Run(locale, func(t *testing.T) {
			tmp := t.TempDir()
			isolateHome(t, tmp)
			t.Setenv("LC_ALL", locale)
			root := bootstrapWorkspace(t, tmp, "empty-projects")
			initialMise, err := os.ReadFile(filepath.Join(root, "mise.toml"))
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct{ id, name, directory string }{
				{"empty-app", "web", "apps"},
				{"empty-service", "api", "services"},
				{"empty-library", "shared", "packages"},
			} {
				stdout, stderr, code := runBinaryIn(t, root, "add", tc.id, "--name", tc.name, "--yes", "-o", "json")
				if code != 0 {
					t.Fatalf("add %s: exit %d\n%s\n%s", tc.id, code, stdout, stderr)
				}
				result := mustParseJSON(t, stdout)
				if result["template_id"] != tc.id || result["toolchain"] != "none" || result["package_manager"] != nil {
					t.Fatalf("unexpected result: %v", result)
				}
				dir := filepath.Join(root, tc.directory, tc.name)
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 1 || entries[0].Name() != ".gitkeep" {
					t.Fatalf("empty project files = %v, err = %v", entries, err)
				}
				// Reusing the name in another category must neither register it nor
				// leave the newly rendered directory behind.
				_, _, code = runBinaryIn(t, root, "add", "empty-library", "--name", tc.name, "--yes", "-o", "json")
				if code == 0 {
					t.Fatal("duplicate project name was accepted")
				}
				if tc.directory != "packages" {
					if _, err := os.Stat(filepath.Join(root, "packages", tc.name)); !os.IsNotExist(err) {
						t.Fatalf("duplicate left a directory: %v", err)
					}
				}
			}
			manifest := readManifest(t, root)
			projects := manifest["projects"].([]any)
			if len(projects) != 3 {
				t.Fatalf("projects = %v", projects)
			}
			for _, raw := range projects {
				p := raw.(map[string]any)
				if p["toolchain"] != "none" || p["dev"] != nil || p["packageManager"] != nil {
					t.Fatalf("empty project acquired runtime configuration: %v", p)
				}
			}
			for _, file := range []string{"package.json", "pnpm-workspace.yaml", "go.work"} {
				if _, err := os.Stat(filepath.Join(root, file)); !os.IsNotExist(err) {
					t.Fatalf("empty projects generated %s: %v", file, err)
				}
			}
			mise, err := os.ReadFile(filepath.Join(root, "mise.toml"))
			if err != nil {
				t.Fatal(err)
			}
			if string(mise) != string(initialMise) {
				t.Fatalf("empty projects changed workspace tools or tasks:\n%s", mise)
			}
			stdout, stderr, code := runBinaryIn(t, root, "add", "empty-app", "--name", "custom", "--yes", "-o", "text")
			hint := "Add your code"
			if locale == "zh_CN.UTF-8" {
				hint = "添加代码"
			}
			if code != 0 || !strings.Contains(stdout, hint) || strings.Contains(stdout, "one dev -p") {
				t.Fatalf("empty project guidance: exit %d\n%s\n%s", code, stdout, stderr)
			}
		})
	}
}
