package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func appendRootTaskConfig(t *testing.T, root, value string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	buildWrite(t, root, "mise.toml", string(raw)+"\n"+value)
}

func TestE2E_TasksRestoreArtifactsAndInvalidateRootConfiguration(t *testing.T) {
	root := buildFixture(t, true)
	for _, project := range []struct{ name, dir string }{{"web", "apps/web"}, {"lib", "packages/lib"}} {
		name, dir := project.name, project.dir
		buildWrite(t, root, dir+"/build.sh", "#!/bin/sh\nmkdir -p dist\nprintf '%s' \"$BUILD_VALUE\" > dist/value\necho executed >> executions\nprintf '<%s>\\n' \"$@\"\n")
		appendRootTaskConfig(t, root, "[tasks.\""+name+":build\"]\nenv={BUILD_VALUE='"+name+"'}\nsources=['package.json','build.sh']\noutputs=['dist']\ncache={enabled=true,env=['BUILD_VALUE'],command_inputs=['one __task-input --project "+name+" --task build']}\n")
	}
	run := func(args ...string) string {
		t.Helper()
		out, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 {
			t.Fatalf("%v: %d %s %s", args, code, out, stderr)
		}
		return out + stderr
	}
	runs := func(dir string) int {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, dir, "executions"))
		if err != nil {
			t.Fatal(err)
		}
		return strings.Count(string(raw), "executed")
	}
	run("run", "build", "-p", "web", "-o", "json")
	run("build", "-p", "web", "-o", "json")
	if runs("apps/web") != 1 || runs("packages/lib") != 1 {
		t.Fatal("repeat did not hit cache")
	}
	for _, dir := range []string{"apps/web", "packages/lib"} {
		if err := os.RemoveAll(filepath.Join(root, dir, "dist")); err != nil {
			t.Fatal(err)
		}
	}
	run("run", "build", "-p", "web", "-o", "json")
	for _, project := range []struct{ name, dir string }{{"web", "apps/web"}, {"lib", "packages/lib"}} {
		name, dir := project.name, project.dir
		raw, err := os.ReadFile(filepath.Join(root, dir, "dist/value"))
		if err != nil || string(raw) != name {
			t.Fatalf("restored environment %s: %s %v", name, raw, err)
		}
		if runs(dir) != 1 {
			t.Fatal("restore executed the task")
		}
	}
	config, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	buildWrite(t, root, "mise.toml", strings.Replace(string(config), "BUILD_VALUE='web'", "BUILD_VALUE='changed'", 1))
	run("run", "build", "-p", "web", "-o", "json")
	// mise includes the defining config in every task cache key. Editing the
	// shared root config invalidates both tasks, while their environments stay isolated.
	if runs("apps/web") != 2 || runs("packages/lib") != 2 {
		t.Fatalf("root configuration did not invalidate both tasks: web=%d lib=%d", runs("apps/web"), runs("packages/lib"))
	}
	raw, _ := os.ReadFile(filepath.Join(root, "apps/web/dist/value"))
	if string(raw) != "changed" {
		t.Fatal("stale environment cache hit")
	}
	lib, _ := os.ReadFile(filepath.Join(root, "packages/lib/dist/value"))
	if string(lib) != "lib" {
		t.Fatal("project environments are not isolated")
	}
	run("run", "build", "-p", "web", "--force", "--cache", "off")
	if runs("apps/web") != 3 || runs("packages/lib") != 3 {
		t.Fatal("forced execution skipped tasks")
	}
	out := run("run", "build", "-p", "web", "--", "a b", "中文", "$(nope)")
	for _, want := range []string{"<a b>", "<中文>", "<$(nope)>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("lost argv %q: %s", want, out)
		}
	}
	// Missing context fails even when an artifact is already cached.
	if _, _, code := runBinaryIn(t, root, "__task-input", "--project", "web", "--task", "build"); code == 0 {
		t.Fatal("missing context accepted")
	}
}

func TestE2E_NativeMiseFileTask(t *testing.T) {
	root := buildFixture(t, true)
	buildWrite(t, root, ".mise/tasks/hello", "#!/bin/sh\necho native-file-task\n")
	for _, args := range [][]string{{"run", "-o", "json"}, {"run", "hello", "--dry-run", "-o", "json"}} {
		stdout, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 || !strings.Contains(stdout, "//:hello") {
			t.Fatalf("%v: %d %s %s", args, code, stdout, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "apps/web/mise.toml")); !os.IsNotExist(err) {
		t.Fatal("static discovery wrote managed configuration")
	}
	stdout, stderr, code := runBinaryIn(t, root, "run", "hello", "-o", "json")
	if code != 0 || !strings.Contains(stderr, "native-file-task") || !strings.Contains(stdout, "one-cli/task-result/v1") {
		t.Fatalf("%d %s %s", code, stdout, stderr)
	}
}

func TestE2E_TasksRejectCacheWithoutEnvironmentFingerprint(t *testing.T) {
	root := buildFixture(t, true)
	appendRootTaskConfig(t, root, "[tasks.\"web:build\"]\nsources=['package.json']\noutputs=['dist']\ncache={enabled=true}\n")
	stdout, stderr, code := runBinaryIn(t, root, "run", "build", "-p", "web", "-o", "json")
	if code == 0 || !strings.Contains(stderr+stdout, "__task-input") {
		t.Fatalf("effective cache without fingerprint accepted: %d %s %s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "order")); !os.IsNotExist(err) {
		t.Fatal("task ran before effective cache was validated")
	}
}

func TestE2E_NativeMiseIncludedTOMLTask(t *testing.T) {
	root := buildFixture(t, true)
	buildWrite(t, root, "mise.toml", "[tools]\nnode='system'\npnpm='system'\n[task_config]\nincludes=['tasks.toml']\n")
	buildWrite(t, root, "tasks.toml", "[hello]\nrun='echo included-task'\ndescription='Included task'\n")
	for _, args := range [][]string{{"run", "-o", "json"}, {"run", "hello", "--dry-run", "-o", "json"}, {"run", "hello", "-o", "json"}} {
		stdout, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 || !strings.Contains(stdout, "//:hello") {
			t.Fatalf("%v: %d %s %s", args, code, stdout, stderr)
		}
	}
}

func TestE2E_TasksReuseArtifactsAcrossCheckouts(t *testing.T) {
	t.Setenv("MISE_TASK_CACHE_DIR", t.TempDir())
	for i := 0; i < 2; i++ {
		root := buildFixture(t, true)
		for _, project := range []struct{ name, dir string }{{"web", "apps/web"}, {"lib", "packages/lib"}} {
			name, dir := project.name, project.dir
			buildWrite(t, root, dir+"/build.sh", "#!/bin/sh\nmkdir -p dist\necho portable-artifact > dist/value\necho executed >> executions\n")
			appendRootTaskConfig(t, root, "[tasks.\""+name+":build\"]\nsources=['package.json','build.sh']\noutputs=['dist']\ncache={enabled=true,command_inputs=['one __task-input --project "+name+" --task build']}\n")
		}
		stdout, stderr, code := runBinaryIn(t, root, "build", "-o", "json")
		if code != 0 {
			t.Fatalf("checkout %d: %d %s %s", i, code, stdout, stderr)
		}
		if i == 1 {
			for _, dir := range []string{"apps/web", "packages/lib"} {
				if _, err := os.Stat(filepath.Join(root, dir, "executions")); !os.IsNotExist(err) {
					t.Fatal("cache was not reused across checkout paths", err)
				}
				raw, err := os.ReadFile(filepath.Join(root, dir, "dist/value"))
				if err != nil || string(raw) != "portable-artifact\n" {
					t.Fatalf("missing restored artifact: %s %v", raw, err)
				}
			}
		}
	}
}

func TestE2E_GoTaskBuildRestoresExecutableAndForwardsArguments(t *testing.T) {
	taskBinary, err := exec.LookPath("task")
	if err != nil {
		t.Skip("real Task is required for Go integration")
	}
	// Resolve the actual binary before isolating mise's data directory. A mise
	// shim would otherwise look for this tool inside the empty fixture cache.
	if out, err := exec.Command("mise", "which", "task").Output(); err == nil {
		taskBinary = strings.TrimSpace(string(out))
	}
	root := buildFixture(t, true)
	if err := os.Symlink(taskBinary, filepath.Join(root, "tools/task")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CGO_ENABLED", "0")
	buildWrite(t, root, "one.manifest.json", `{"version":1,"workspace":{"id":"go-test","name":"go-test"},"projects":[{"name":"api","relativeDir":"services/api","toolchain":"go","templateId":"go-api"}]}`)
	buildWrite(t, root, "mise.toml", "[tools]\ngo='system'\ntask='system'\nnode='system'\npnpm='system'\n")
	buildWrite(t, root, "services/api/go.mod", "module example.com/task-test\n\ngo 1.25.0\n")
	buildWrite(t, root, "services/api/cmd/server/main.go", "package main\nimport \"fmt\"\nvar message=\"default\"\nfunc main(){fmt.Print(message)}\n")
	scaffold, err := os.ReadFile(filepath.Join(repoRoot(t), "../templates/go-api/Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	buildWrite(t, root, "services/api/Taskfile.yml", string(scaffold))
	run := func(args ...string) {
		t.Helper()
		stdout, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 {
			t.Fatalf("%v: %d %s %s", args, code, stdout, stderr)
		}
	}
	check := func(want string) {
		t.Helper()
		out, err := exec.Command(filepath.Join(root, "services/api/bin/server")).CombinedOutput()
		if err != nil || string(out) != want {
			t.Fatalf("built executable: %s %v, want %s", out, err, want)
		}
	}
	run("build", "-p", "api")
	check("default")
	if err = os.RemoveAll(filepath.Join(root, "services/api/bin")); err != nil {
		t.Fatal(err)
	}
	run("run", "build", "-p", "api")
	check("default")
	run("run", "build", "-p", "api", "--", "-ldflags=-X 'main.message=hello world'")
	check("hello world")
}
