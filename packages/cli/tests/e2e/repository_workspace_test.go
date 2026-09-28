package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The repository must remain bootstrappable without One, while project-scoped
// CLI tasks must prepare the resources and binary that Go alone cannot build.
func TestE2E_RepositoryWorkspaceTaskGraph(t *testing.T) {
	isolateHome(t, t.TempDir())
	root := filepath.Clean(filepath.Join(repoRoot(t), "..", ".."))
	for _, dir := range []string{"apps/dashboard", "apps/docs", "packages/cli", "packages/kernel"} {
		if _, err := os.Stat(filepath.Join(root, dir, "mise.toml")); !os.IsNotExist(err) {
			t.Fatalf("repository must use only root mise.toml: %s: %v", dir, err)
		}
	}
	for _, tc := range []struct {
		name      string
		args      []string
		required  []string
		bootstrap bool
	}{
		{"bootstrap", []string{"install"}, []string{"//:build-local", "//:sync-bundled", "//:sync-web"}, true},
		{"gate", []string{"check"}, []string{"//:build", "//:test:go:plain", "//:test:dashboard"}, true},
		{"cli build", []string{"build", "-p", "cli"}, []string{"//:sync-bundled", "//:build-web", "//:sync-web", "//:cli:build"}, false},
		{"cli test", []string{"test", "-p", "cli"}, []string{"//:build", "//:cli:test"}, false},
		{"development", []string{"dev"}, []string{"//:dev:serve", "//:dev:dashboard"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, tc.args...), "--dry-run", "-o", "json")
			out, logs, code := runBinaryIn(t, root, args...)
			if code != 0 {
				t.Fatal(code, out, logs)
			}
			var plan struct {
				Tasks []struct {
					Name    string `json:"name"`
					Managed bool   `json:"managed"`
				} `json:"tasks"`
				Changes []json.RawMessage `json:"config_changes"`
			}
			if err := json.Unmarshal([]byte(out), &plan); err != nil {
				t.Fatal(err)
			}
			if len(plan.Changes) != 0 {
				t.Fatal("repository task configuration is stale; run one init mise")
			}
			names := map[string]bool{}
			for _, task := range plan.Tasks {
				names[task.Name] = true
				if tc.bootstrap && task.Managed {
					t.Errorf("native root workflow requires an installed One adapter: %s", task.Name)
				}
				if strings.Contains(task.Name, "packages/templates/") || strings.Contains(task.Name, "/testdata/") {
					t.Errorf("source fixture became a runnable workspace project: %s", task.Name)
				}
			}
			for _, name := range tc.required {
				if !names[name] {
					t.Errorf("missing prerequisite %s in %v", name, names)
				}
			}
		})
	}
}
