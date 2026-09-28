package tasks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
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
	writeTaskFile(t, root, "one.manifest.json", `{"version":1,"workspace":{"id":"test","name":"test"},"environments":{"names":["dev","prod"],"default":"dev"},"projects":[{"name":"web","relativeDir":"apps/web","toolchain":"node"},{"name":"lib","relativeDir":"packages/lib","toolchain":"node"}]}`)
	writeTaskFile(t, root, "package.json", `{"packageManager":"pnpm@12.3.4"}`)
	writeTaskFile(t, root, "apps/web/package.json", `{"name":"web","scripts":{"build":"echo web","test":"echo test"},"dependencies":{"lib":"workspace:*"}}`)
	writeTaskFile(t, root, "packages/lib/package.json", `{"name":"lib","scripts":{"build":"echo lib","test":"echo test"}}`)
	w, err := execution.ResolveWorkspaceScope(execution.NewScope(context.Background(), root))
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func TestPlanIncludesSharedDependenciesWithoutWrites(t *testing.T) {
	w := taskWorkspace(t)
	p, err := NewPlan(w, Options{Name: "build", Projects: []string{"web", "lib"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tasks) != 2 || p.Tasks[0].Project != "lib" || p.Tasks[1].Project != "web" {
		t.Fatalf("%+v", p.Tasks)
	}
	if _, err = os.Stat(filepath.Join(w.Root(), ".mise")); !os.IsNotExist(err) {
		t.Fatal("preview wrote configuration")
	}
	if _, err = NewPlan(w, Options{Name: "build", Projects: []string{"web", "lib"}, Arguments: []string{"flag"}}); err == nil {
		t.Fatal("ambiguous arguments accepted")
	}
	if _, err = NewPlan(w, Options{Name: "build", Environment: "missing"}); err == nil {
		t.Fatal("unknown environment accepted")
	}
}
func TestPlanRejectsCycleOutputConflictsAndUnsafeCache(t *testing.T) {
	for _, tc := range []struct{ name, root, project string }{
		{"cycle", `[tasks.build]
depends=["build"]`, ""},
		{"overlap", `[tasks.build]
depends=["a","b"]
[tasks.a]
run="echo a"
outputs=["dist"]
[tasks.b]
run="echo b"
outputs=["dist/child"]`, ""},
		{"glob overlap", `[tasks.build]
depends=["a","b"]
[tasks.a]
run="echo a"
outputs=["dist/**/*.js"]
[tasks.b]
run="echo b"
outputs=["dist/assets"]`, ""},
		{"cache without environment fingerprint", "", `[tasks.build]
sources=["package.json"]
outputs=["dist"]
cache={enabled=true}`},
		{"escaped output", "", `[tasks.build]
outputs=["../../outside"]`},
		{"managed directory override", "", `[tasks.build]
dir="../elsewhere"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := taskWorkspace(t)
			writeTaskFile(t, w.Root(), "mise.toml", tc.root)
			writeTaskFile(t, w.Root(), "apps/web/mise.toml", tc.project)
			if _, err := NewPlan(w, Options{Name: "build"}); err == nil {
				t.Fatal("invalid task plan accepted")
			}
		})
	}
}

type countingLoader struct {
	calls map[string]int
	value string
}

func (l *countingLoader) ID() string { return "infisical" }
func (l *countingLoader) Load(_ context.Context, _ string, project, _ string) (map[string]string, error) {
	l.calls[project]++
	return map[string]string{"VALUE": l.value + project}, nil
}
func TestContextFreezesEachProjectOnceAndCleansUp(t *testing.T) {
	w := taskWorkspace(t)
	w.Manifest().Env = &workspace.EnvironmentConfig{ProjectID: "remote"}
	p, err := NewPlan(w, Options{Name: "build"})
	if err != nil {
		t.Fatal(err)
	}
	// The same project may contribute multiple tasks to an aggregate.
	p.Tasks = append(p.Tasks, Task{Project: "web", Operation: "test", Managed: true})
	loader := &countingLoader{calls: map[string]int{}, value: "secret-"}
	env, _, cleanup, err := prepareContext(context.Background(), w, p, secrets.MustRegistry(loader))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	path := ""
	for _, entry := range env {
		if strings.HasPrefix(entry, contextVariable+"=") {
			path = strings.TrimPrefix(entry, contextVariable+"=")
		}
	}
	t.Setenv(contextVariable, path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatal("context readable outside owner")
	}
	if loader.calls["apps/web"] != 1 || loader.calls["packages/lib"] != 1 {
		t.Fatal(loader.calls)
	}
	state, web, err := loadContext(w, "web", "build")
	if err != nil {
		t.Fatal(err)
	}
	if web.Variables["VALUE"] != "secret-apps/web" {
		t.Fatal("wrong project environment")
	}
	first, err := InputFingerprint(context.Background(), w, "web", "build")
	if err != nil {
		t.Fatal(err)
	}
	web.Operations["unrelated"] = []string{"echo", "unrelated"}
	state.Projects["web"] = web
	raw, _ := json.Marshal(state)
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	second, err := InputFingerprint(context.Background(), w, "web", "build")
	if err != nil || first != second {
		t.Fatal("unrelated tasks changed cache key", err)
	}
	web.Variables["VALUE"] = "changed"
	state.Projects["web"] = web
	raw, _ = json.Marshal(state)
	_ = os.WriteFile(path, raw, 0600)
	third, err := InputFingerprint(context.Background(), w, "web", "build")
	if err != nil || third == first {
		t.Fatal("environment did not invalidate key", err)
	}
	encoded, _ := json.Marshal(p)
	if strings.Contains(string(encoded), "secret-") {
		t.Fatal("plan leaked environment")
	}
	cleanup()
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("context survived cleanup")
	}
	if _, _, err = loadContext(w, "web", "build"); err == nil {
		t.Fatal("expired context accepted")
	}
}
