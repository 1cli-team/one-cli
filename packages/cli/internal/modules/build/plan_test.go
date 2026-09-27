package build

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func write(t *testing.T, root, path, contents string) {
	t.Helper()
	target := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) execution.Workspace {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ONE_RUNTIME", "builtin")
	manifest := workspace.Manifest{Version: 1, Workspace: &workspace.ManifestWorkspace{ID: "build-test", Name: "build-test"}, Projects: []workspace.ManifestProject{
		{Name: "web", RelativeDir: "apps/web", Toolchain: "node"},
		{Name: "library", RelativeDir: "packages/lib", Toolchain: "node"},
		{Name: "api", RelativeDir: "services/api", Toolchain: "go"},
		{Name: "mobile", RelativeDir: "apps/mobile", Toolchain: "node"},
	}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "one.manifest.json", string(raw))
	write(t, root, "package.json", `{"packageManager":"npm@11.0.0"}`)
	write(t, root, "apps/web/package.json", `{"name":"@test/web","scripts":{"build":"build-web"},"dependencies":{"@test/lib":"workspace:*"}}`)
	write(t, root, "packages/lib/package.json", `{"name":"@test/lib","scripts":{"build":"build-lib"}}`)
	write(t, root, "services/api/Taskfile.yml", "version: '3'\ntasks:\n  build:\n    cmds: ['go build -o bin/server ./cmd/server']\n")
	write(t, root, "apps/mobile/package.json", `{"name":"@test/mobile","scripts":{"start":"start-mobile"}}`)
	scope := execution.NewScope(context.Background(), root)
	t.Cleanup(func() { _ = scope.Close(context.Background()) })
	w, err := execution.ResolveWorkspaceScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestPlanOrdersLibrariesAndSkipsMissingTasks(t *testing.T) {
	w := fixture(t)
	p, err := NewPlan(w, "", "")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, task := range p.Tasks {
		names = append(names, task.Project)
	}
	if !reflect.DeepEqual(names, []string{"library", "web", "api", "mobile"}) {
		t.Fatal(names)
	}
	if !reflect.DeepEqual(p.Tasks[0].Argv, []string{"npm", "run", "build"}) || !reflect.DeepEqual(p.Tasks[2].Argv, []string{"task", "build"}) {
		t.Fatal(p.Tasks)
	}
	if p.Tasks[3].Status != "skipped" || p.Tasks[3].Reason != "no-build-task" {
		t.Fatal(p.Tasks[3])
	}
	if !p.DryRun || p.Schema != "one-cli/build-plan/v1" {
		t.Fatal(p)
	}
	if _, err := os.Stat(filepath.Join(w.Root(), "node_modules")); !os.IsNotExist(err) {
		t.Fatal("planning prepared dependencies")
	}
}

func TestPlanSelectionAndErrors(t *testing.T) {
	w := fixture(t)
	for _, selector := range []string{"web", "apps/web", "./apps/web/"} {
		p, err := NewPlan(w, selector, "")
		if err != nil || len(p.Tasks) != 1 || p.Tasks[0].Project != "web" {
			t.Fatalf("%s: %+v %v", selector, p, err)
		}
	}
	for selector, code := range map[string]string{"unknown": "SUBPROJECT_NOT_FOUND", "mobile": "RUNTIME_TASK_NOT_FOUND"} {
		_, err := NewPlan(w, selector, "")
		var coded *output.Error
		if !errors.As(err, &coded) || coded.Code != code {
			t.Fatalf("%s: %v", selector, err)
		}
	}
	// Unrelated malformed projects must not block a selected build.
	write(t, w.Root(), "packages/lib/package.json", "broken")
	if _, err := NewPlan(w, "web", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPlan(w, "", ""); err == nil {
		t.Fatal("malformed package ignored")
	}
}

func TestPlanRejectsCycleAndDuplicatePackageNames(t *testing.T) {
	w := fixture(t)
	write(t, w.Root(), "packages/lib/package.json", `{"name":"@test/lib","scripts":{"build":"build-lib"},"devDependencies":{"@test/web":"workspace:*"}}`)
	if _, err := NewPlan(w, "", ""); err == nil || !strings.Contains(err.Error(), "web -> library -> web") {
		t.Fatal(err)
	}
	write(t, w.Root(), "packages/lib/package.json", `{"name":"@test/web","scripts":{"build":"build-lib"}}`)
	if _, err := NewPlan(w, "", ""); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatal(err)
	}
}

func TestPlanLocalDirectoryDependencies(t *testing.T) {
	for _, prefix := range []string{"file:", "link:"} {
		t.Run(prefix, func(t *testing.T) {
			w := fixture(t)
			write(t, w.Root(), "apps/web/package.json", `{"name":"@test/web","scripts":{"build":"build-web"},"optionalDependencies":{"alias":"`+prefix+`../../packages/lib"}}`)
			p, err := NewPlan(w, "", "")
			if err != nil || p.Tasks[0].Project != "library" {
				t.Fatalf("%+v %v", p, err)
			}
		})
	}
}

func TestGoBuildRequiresTaskfileAndBuildTask(t *testing.T) {
	w := fixture(t)
	if err := os.Remove(filepath.Join(w.Root(), "services/api/Taskfile.yml")); err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{"api", ""} {
		if _, err := NewPlan(w, selector, ""); err == nil || !strings.Contains(err.Error(), "Taskfile.yml") {
			t.Fatal(err)
		}
	}
	write(t, w.Root(), "services/api/Taskfile.yml", "version: '3'\ntasks:\n  test:\n    cmds: ['go test ./...']\n")
	if _, err := NewPlan(w, "api", ""); err == nil {
		t.Fatal("missing build accepted")
	}
	p, err := NewPlan(w, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range p.Tasks {
		if task.Project == "api" && task.Status != "skipped" {
			t.Fatal(task)
		}
	}
}

func TestEmptyBuildAndInvalidEnvironment(t *testing.T) {
	w := fixture(t)
	for _, path := range []string{"apps/web/package.json", "packages/lib/package.json"} {
		write(t, w.Root(), path, `{"scripts":{}}`)
	}
	write(t, w.Root(), "services/api/Taskfile.yml", "version: '3'\ntasks: {}\n")
	if _, err := NewPlan(w, "", ""); err == nil {
		t.Fatal("empty build accepted")
	}
	w.Manifest().Environments = &workspace.Environments{Names: []string{"dev", "prod"}, Default: "dev"}
	raw, _ := json.Marshal(w.Manifest())
	write(t, w.Root(), "one.manifest.json", string(raw))
	if _, err := NewPlan(w, "web", "typo"); err == nil || !strings.Contains(err.Error(), "typo") {
		t.Fatal(err)
	}
}

func TestMultipleProjectSelectionOrdersDependenciesAndDeduplicates(t *testing.T) {
	w := fixture(t)
	p, err := NewPlanForProjects(w, []string{"apps/web", "library", "web"}, "")
	if err != nil || len(p.Tasks) != 2 || p.Tasks[0].Project != "library" || p.Tasks[1].Project != "web" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := NewPlanForProjects(w, []string{"web", "unknown"}, ""); err == nil {
		t.Fatal("unknown project accepted")
	}
}
