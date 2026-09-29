package tasks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
)

func writeTaskFile(t *testing.T, root, path, content string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func taskWorkspace(t *testing.T) execution.Workspace {
	t.Helper()
	root := t.TempDir()
	writeTaskFile(t, root, "one.manifest.toml", `version = 2

[workspace]
id = "test"
name = "test"

[projects."web"]
path = "apps/web"
toolchain = "none"

[projects."lib"]
path = "packages/lib"
toolchain = "none"
`)
	writeTaskFile(t, root, "apps/web/package.json", `{"scripts":{"dev":"echo web","start":"echo start","build":"echo build"}}`)
	writeTaskFile(t, root, "packages/lib/package.json", `{"scripts":{"build":"echo lib"}}`)
	writeTaskFile(t, root, "mise.toml", `[tasks.build]
depends=["web:build","lib:build"]
[tasks."web:build"]
dir="apps/web"
run="echo web"
depends=["lib:build"]
[tasks."lib:build"]
dir="packages/lib"
run="echo lib"
`)
	w, err := execution.ResolveWorkspaceScope(execution.NewScope(context.Background(), root))
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func TestPlanUsesExistingTasksWithoutWrites(t *testing.T) {
	w := taskWorkspace(t)
	before, _ := os.ReadFile(filepath.Join(w.Root(), "mise.toml"))
	p, err := NewPlan(w, Options{Name: "build", Projects: []string{"web", "lib"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tasks) != 2 || p.Tasks[0].Project != "lib" || p.Tasks[1].Project != "web" || len(p.ConfigChanges) != 0 {
		t.Fatalf("%+v", p)
	}
	for _, opts := range []Options{{Name: "dev"}, {Name: "start", Projects: []string{"web"}}, {Name: "build", Projects: []string{"web", "lib"}, Arguments: []string{"flag"}}, {Name: "build", Environment: "missing"}} {
		if _, err := NewPlan(w, opts); err == nil {
			t.Fatalf("unexpected plan: %+v", opts)
		}
	}
	after, _ := os.ReadFile(filepath.Join(w.Root(), "mise.toml"))
	if string(before) != string(after) {
		t.Fatal("planning changed configuration")
	}
}
func TestPlanProjectOwnershipFollowsDirectory(t *testing.T) {
	w := taskWorkspace(t)
	writeTaskFile(t, w.Root(), "mise.toml", `[tasks.serve-backend]
dir="apps/web"
run="echo real-task"`)
	p, err := NewPlan(w, Options{Name: "serve-backend"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Tasks[0].Project != "web" {
		t.Fatal(p.Tasks)
	}
	if _, err = NewPlan(w, Options{Name: "dev", Projects: []string{"web"}}); err == nil {
		t.Fatal("invented dev task")
	}
}
func TestPlanRejectsCyclesAndOverlappingOutputs(t *testing.T) {
	for _, config := range []string{`[tasks.build]
depends=["build"]`, `[tasks.build]
depends=["a","b"]
[tasks.a]
run="echo a"
outputs=["dist"]
[tasks.b]
run="echo b"
outputs=["dist/child"]`, `[tasks.build]
run="echo x"
outputs=["../outside"]`} {
		w := taskWorkspace(t)
		writeTaskFile(t, w.Root(), "mise.toml", config)
		if _, err := NewPlan(w, Options{Name: "build"}); err == nil {
			t.Fatal("invalid plan accepted")
		}
	}
}
func TestPlanDoesNotEvaluateCommandInputs(t *testing.T) {
	w := taskWorkspace(t)
	writeTaskFile(t, w.Root(), "mise.local.toml", `[tasks."web:build".cache]
enabled=true
command_inputs=["echo forbidden > SHOULD_NOT_EXIST"]`)
	_, err := NewPlan(w, Options{Name: "build"})
	if err == nil || !strings.Contains(err.Error(), "cache.command_inputs") {
		t.Fatal("unsupported command input accepted", err)
	}
	if _, err = os.Stat(filepath.Join(w.Root(), "SHOULD_NOT_EXIST")); !os.IsNotExist(err) {
		t.Fatal("dry-run executed input")
	}
}

func TestStaticNativeScopesAliasesAndCommandOverrides(t *testing.T) {
	w := taskWorkspace(t)
	writeTaskFile(t, w.Root(), "mise.toml", `monorepo_root=true
[monorepo]
config_roots=["apps/*"]
[tasks.build]
depends=["//apps/web:compile"]
`)
	writeTaskFile(t, w.Root(), "apps/web/mise.toml", `[tasks.build]
alias="compile"
run="echo base"
depends=["missing-old-dependency"]
`)
	writeTaskFile(t, w.Root(), "apps/web/mise.local.toml", `[tasks.build]
alias="compile"
run="echo override"
`)
	plan, err := NewPlan(w, Options{Name: "build"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 2 || plan.Tasks[0].Run != "echo override" || plan.Tasks[0].Project != "web" {
		t.Fatal(plan.Tasks)
	}
	if _, err = NewPlan(w, Options{Name: "build", Projects: []string{"web"}}); err != nil {
		t.Fatal(err)
	}
}
