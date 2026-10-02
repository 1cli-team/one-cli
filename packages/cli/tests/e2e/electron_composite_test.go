package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_ElectronCompositeNamingAndGroupPlans(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	root := bootstrapWorkspace(t, tmp, "workspace")
	for _, name := range []string{"desktop", "electron"} {
		stdout, stderr, code := runBinaryIn(t, root, "add", "electron-app", "--name", name, "-y", "-o", "json")
		if code != 0 {
			t.Fatalf("add: %d\n%s\n%s", code, stdout, stderr)
		}
		var result struct {
			Name     string `json:"subproject_name"`
			Projects []struct {
				Name string `json:"name"`
				Path string `json:"target_path"`
			} `json:"projects"`
		}
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err, stdout)
		}
		if result.Name != name || len(result.Projects) != 3 {
			t.Fatal(stdout)
		}
		for i, location := range []struct{ directory, suffix string }{{"apps", "-renderer"}, {"services", "-main"}, {"packages", "-preload"}} {
			member := result.Projects[i]
			if member.Name != name+location.suffix || member.Path != filepath.Join(root, location.directory, member.Name) {
				t.Fatalf("member: %#v", member)
			}
			if _, err := os.Stat(filepath.Join(member.Path, "package.json")); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := os.Stat(filepath.Join(root, "apps", name)); !os.IsNotExist(err) {
			t.Fatal("nested wrapper still exists")
		}
	}
	for _, op := range []string{"dev", "build", "test", "pack"} {
		stdout, stderr, code := runBinaryIn(t, root, "run", "desktop:"+op, "--dry-run", "-o", "json")
		if code != 0 {
			t.Fatalf("plan %s: %d\n%s\n%s", op, code, stdout, stderr)
		}
		var plan struct {
			Tasks []struct {
				Name    string `json:"name"`
				Project string `json:"project"`
			} `json:"tasks"`
		}
		if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
			t.Fatal(err, stdout)
		}
		found := map[string]bool{}
		for _, task := range plan.Tasks {
			if strings.HasPrefix(task.Project, "electron-") {
				t.Fatalf("group crossed into second app: %s", stdout)
			}
			found[task.Name] = true
		}
		for _, member := range []string{"desktop-renderer", "desktop-main", "desktop-preload"} {
			task := "//:" + member + ":build"
			if op == "dev" {
				task = "//:" + member + ":dev"
			}
			if op == "test" && member == "desktop-main" {
				task = "//:" + member + ":test"
			}
			if !found[task] {
				t.Fatalf("missing %s in %s", task, stdout)
			}
		}
	}
	stdout, stderr, code := runBinaryIn(t, root, "init", "mise", "-o", "json")
	if code != 0 {
		t.Fatalf("refresh: %d\n%s\n%s", code, stdout, stderr)
	}
	stdout, stderr, code = runBinaryIn(t, root, "run", "desktop:pack", "--dry-run", "-o", "json")
	if code != 0 {
		t.Fatalf("refreshed group: %d\n%s\n%s", code, stdout, stderr)
	}
}
